package collection

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/buger/jsonparser"
	log "github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"k8s.io/kubectl/pkg/scheme"
)

// Function is heavily based on
// https://stackoverflow.com/questions/43314689/example-of-exec-in-k8ss-pod-by-using-go-client/54317689
type execRequest struct {
	Client     kubernetes.Interface
	RestConfig *restclient.Config
	PodName    string
	Namespace  string
	Command    string
	IOStreams  *genericclioptions.IOStreams
	TTY        bool
}

func execCommand(reqOptions execRequest) (int, error) {
	exitCode := 0
	cmd := []string{
		"sh",
		"-c",
		reqOptions.Command,
	}
	option := &corev1.PodExecOptions{
		Command: cmd,
		Stdin:   reqOptions.IOStreams.In != nil,
		Stdout:  reqOptions.IOStreams.Out != nil,
		Stderr:  reqOptions.IOStreams.ErrOut != nil,
		TTY:     reqOptions.TTY,
	}
	req := reqOptions.Client.CoreV1().RESTClient().Post().Resource("pods").Name(reqOptions.PodName).
		VersionedParams(option, scheme.ParameterCodec).Namespace(reqOptions.Namespace).SubResource("exec")

	exec, err := remotecommand.NewSPDYExecutor(reqOptions.RestConfig, "POST", req.URL())
	if err != nil {
		return exitCode, err
	}

	// This is a blocking statement - Manage the bidi stream in Goroutine
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  reqOptions.IOStreams.In,
		Stdout: reqOptions.IOStreams.Out,
		Stderr: reqOptions.IOStreams.ErrOut,
	})

	if err != nil {
		if exitErr, ok := err.(utilexec.ExitError); ok && exitErr.Exited() {
			exitCode = exitErr.ExitStatus()
			log.Debugf("Command %q exited with code: %d", reqOptions.Command, exitCode)
			return exitCode, nil
		}
		return exitCode, err
	}
	return exitCode, nil
}

func CreateNamespace(ctx context.Context, clientset kubernetes.Interface, name string) (*corev1.Namespace, error) {
	ns, err := clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", name),
			Labels: map[string]string{
				fmt.Sprintf("%s-generated-namespace", name): "",
			},
		},
	}, metav1.CreateOptions{})

	if err == nil {
		log.Infof("Namespace %q Created", ns.Name)
	}

	return ns, err

}

func CleanupNamespace(ctx context.Context, client kubernetes.Interface, namespace string) {
	// Delete Namespace
	err := client.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil {
		log.Fatalf("unable to delete strace namespace %q. manual deletion is required.", namespace)
	}
}

func findPodPIDs(ctx context.Context, client kubernetes.Interface, restConfig *restclient.Config, targetPod *corev1.Pod, tracePod *corev1.Pod) ([]int64, error) {
	// Specific to Crictl
	iostreams := &genericclioptions.IOStreams{
		In: nil, Out: new(bytes.Buffer), ErrOut: new(bytes.Buffer),
	}

	// Get all Container IDs for Pod
	containerPIDs := []int64{}

	for _, containerStatus := range targetPod.Status.ContainerStatuses {

		containerID := strings.SplitAfter(containerStatus.ContainerID, "//")[1]
		// TODO Set an env var with the socket path to speed up CRIO resolution
		command := fmt.Sprintf("crictl inspect %s", containerID)
		log.Infof("Running command %q inside pod %q", command, tracePod.Name)

		execRequest := execRequest{
			Client: client, RestConfig: restConfig, PodName: tracePod.Name,
			Namespace: tracePod.Namespace, Command: command, IOStreams: iostreams, TTY: false,
		}
		exitCode, err := execCommand(execRequest)
		if exitCode != 0 || err != nil {
			return nil, err
		}

		// Process JSON
		// TODO perform checks against casting
		json := iostreams.Out.(*bytes.Buffer).Bytes()

		containerPID, err := jsonparser.GetInt(json, "info", "pid")
		if err != nil {
			return nil, err
		}

		log.Infof("Container PID %d found for Container %q", containerPID, containerID)
		containerPIDs = append(containerPIDs, containerPID)

	}

	if len(containerPIDs) < 1 {
		log.Errorf("No container PIDs found for Pod %q", targetPod)
		return nil, fmt.Errorf("no container pids found from %q", targetPod)
	}

	// Run command in Pod
	return containerPIDs, nil
}

func waitForPodRunning(clientset kubernetes.Interface, namespace string, pod string) error {
	// TODO maybe this should be relative to the collection timeout?
	timeout := time.Second * 30

	checkPodState := func() bool {

		podStatus, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), pod, metav1.GetOptions{})
		if err != nil {
			return false
		}

		if podStatus.Status.Phase == corev1.PodRunning {
			return true
		}

		return false
	}

	for timeout > 0 {
		log.Debugf("Waiting for Tracer Pod %q to become ready", pod)
		if checkPodState() {
			break
		}

		// Sleep
		time.Sleep(time.Second * 2)
		timeout -= time.Second * 2
	}

	if timeout <= 0 {
		return fmt.Errorf("tracer pod did not start correctly. review event objects from namespace %q relating to pod %q for more information", namespace, pod)
	}
	return nil
}

func getIOStreams(outputDirectory string, target string) (*genericclioptions.IOStreams, error) {
	var err error

	// Special case for std-out
	if outputDirectory == "-" {
		return &genericclioptions.IOStreams{Out: os.Stdout, In: nil, ErrOut: os.Stderr}, nil
	}

	// Ensure trace folder is present
	if _, err := os.Stat(outputDirectory); errors.Is(err, os.ErrNotExist) {
		err = os.MkdirAll(outputDirectory, 0775)
		if err != nil {
			log.Infof("Unable to create directory for the strace collection. %v", err)
			return nil, err
		}
	}

	// Create file for all sub-traces
	fileWriter, err := os.Create(fmt.Sprintf("%s/%s.log", outputDirectory, target))
	if err != nil {
		log.Infof("Unable to create logfile for the data collection. %v", err)
		return nil, err
	}

	return &genericclioptions.IOStreams{
		Out:    fileWriter,
		ErrOut: fileWriter,
		In:     nil,
	}, nil
}

// Public function for starting the collection trace
// CollectionOptions.Command is used as a template
// Options: {target_pid}
func tprintf(format string, params map[string]interface{}) string {
	for key, val := range params {
		format = strings.Replace(format, "{"+key+"}", fmt.Sprintf("%s", val), -1)
	}
	return format
}
