package kstrace

import (
	"context"

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
type ExecRequest struct {
	Client     kubernetes.Interface
	RestConfig *restclient.Config
	PodName    string
	Namespace  string
	Command    string
	IOStreams  *genericclioptions.IOStreams
	TTY        bool
}

func ExecCommand(reqOptions ExecRequest) (int, error) {
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

func CreateNamespace(ctx context.Context, clientset kubernetes.Interface) (*corev1.Namespace, error) {
	ns, err := clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kstrace-",
			Labels: map[string]string{
				"kstrace-generated-namespace": "",
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
