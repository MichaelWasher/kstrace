package collection

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// Cleanup removes all  up the collection trace
func (tracer *DefaultTracer) Cleanup() {
	ctx := context.TODO()
	// Delete Pod
	if tracer == nil || tracer.tracePod == nil {
		return
	}

	err := tracer.Client.CoreV1().Pods(tracer.tracePod.Namespace).Delete(ctx, tracer.tracePod.Name, metav1.DeleteOptions{})
	if err != nil {
		log.Fatalf("unable to delete collection pod %q from namespace %q. manual deletion may be required.", tracer.tracePod.Name, tracer.tracePod.Namespace)
	}
	tracer.tracePod = nil
}

// StartCollection runs the defined command within the tracer pod
func (tracer *DefaultTracer) StartCollection(iostreams *genericclioptions.IOStreams, command string) (int, error) {

	// Configure Command Timeout
	if tracer.CollectionTimeout != 0 {
		command = fmt.Sprintf("timeout -s 2 --preserve-status %f %s", tracer.CollectionTimeout.Seconds(), command)
	}

	log.Infof("Running command %q inside pod %q", command, tracer.tracePod.Name)

	execRequest := execRequest{
		Client: tracer.Client, RestConfig: tracer.RestConfig, PodName: tracer.tracePod.Name,
		Namespace: tracer.tracePod.Namespace, Command: command, TTY: false, IOStreams: iostreams,
	}
	exitCode, err := execCommand(execRequest)

	log.Infof("'%s' for Pod %q completed", command, tracer.tracePod.Name)

	if err != nil {
		return exitCode, err
	}

	return exitCode, nil
}

func (tracer *DefaultTracer) createCollectionPod(ctx context.Context, options CollectionOptions) (*corev1.Pod, error) {
	// Define the Pod definition
	typeMetadata := metav1.TypeMeta{
		Kind:       "Pod",
		APIVersion: "v1",
	}

	objectMetadata := metav1.ObjectMeta{
		GenerateName: fmt.Sprintf("%s-", options.Name),
		Namespace:    options.Namespace,
		Labels: map[string]string{
			"app": options.Name,
		},
	}

	pod := &corev1.Pod{
		TypeMeta:   typeMetadata,
		ObjectMeta: objectMetadata,
		Spec:       *tracer.TracerPodSpec,
	}

	// Create Pod
	createdPod, err := tracer.Client.CoreV1().Pods(options.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	log.Infof("creating privileged pod %q in namespace %q on node %q", createdPod.Name, createdPod.Namespace, createdPod.Spec.NodeName)
	log.Tracef("creating privileged pod with the following options: { %v }", options)

	// Wait for Ready
	err = waitForPodRunning(tracer.Client, createdPod.Namespace, createdPod.Name)
	if err != nil {
		return nil, err
	}

	log.Infof("created pod %q successfully in namespace %q", createdPod.Name, createdPod.Namespace)
	log.Tracef("created pod details: %v", createdPod)
	return createdPod, nil
}
