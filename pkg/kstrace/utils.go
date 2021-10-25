package kstrace

import (
	"context"
	"io"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
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
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
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
		Stdin:   reqOptions.Stdin != nil,
		Stdout:  reqOptions.Stdout != nil,
		Stderr:  reqOptions.Stderr != nil,
		TTY:     reqOptions.TTY,
	}
	req := reqOptions.Client.CoreV1().RESTClient().Post().Resource("pods").Name(reqOptions.PodName).
		VersionedParams(option, scheme.ParameterCodec).Namespace(reqOptions.Namespace).SubResource("exec")

	exec, err := remotecommand.NewSPDYExecutor(reqOptions.RestConfig, "POST", req.URL())
	if err != nil {
		return exitCode, err
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  reqOptions.Stdin,
		Stdout: reqOptions.Stdout,
		Stderr: reqOptions.Stderr,
	})
	if err != nil {
		return exitCode, err
	}

	if err != nil {
		if exitErr, ok := err.(utilexec.ExitError); ok && exitErr.Exited() {
			exitCode = exitErr.ExitStatus()
			log.Debugf("Command %q failed with exit code: %d", reqOptions.Command, exitCode)
			log.Debugf("Command %q failed with exit code %d and error %v", reqOptions.Command, exitCode, err)
			return exitCode, nil
		}
		return exitCode, err
	}

	return exitCode, nil
}

func CreateNamespace(ctx context.Context, clientset kubernetes.Interface) (*corev1.Namespace, error) {
	return clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kstrace-",
			Labels: map[string]string{
				"kstrace-generated-namespace": "",
			},
		},
	}, metav1.CreateOptions{})
}

func CleanupNamespace(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Delete Namespace
	err := client.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil {
		log.Fatalf("unable to delete strace namespace %q. manual deletion is required.", namespace)
	}
	return err
}
