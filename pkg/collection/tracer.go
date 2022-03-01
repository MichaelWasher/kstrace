package collection

import (
	"context"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func (tracer *PodTracer) checkConfig(options CollectionOptions) error {
	// TODO Ensure that Pod Spec and required values have been set
	if tracer.TracerPodSpec == nil {
		return fmt.Errorf("the Pod Spec needs to be added the tracer")
	}
	return nil
}

// Public function for starting the collection trace
// /// CollectionOptions.Command is used as a template
// Options: {target_pid}
// sweaters := Inventory{"wool", 17}
// tmpl, err :=
// if err != nil { panic(err) }
// err = tmpl.Execute(os.Stdout, sweaters)
// if err != nil { panic(err) }
func tprintf(format string, params map[string]interface{}) string {
	for key, val := range params {
		format = strings.Replace(format, "{"+key+"}", fmt.Sprintf("%s", val), -1)
	}
	return format
}

func (tracer *PodTracer) Start(options CollectionOptions) error {
	var err error
	ctx := context.TODO()

	err = tracer.checkConfig(options)
	if err != nil {
		log.Errorf("Pod tracer requires additional values")
		return err
	}

	tracer.tracePod, err = tracer.createCollectionPod(ctx, options)
	if err != nil {
		return err
	}

	// Find out the PID for the requested Pod
	log.Infof("Running %s", options.Name)
	tracer.targetContainerPIDs, err = findPodPIDs(ctx, tracer.Client, tracer.RestConfig, tracer.TargetPod, tracer.tracePod)

	if err != nil {
		return err
	}

	// Run Strace for collected targetContainerPIDs
	for index, containerPID := range tracer.targetContainerPIDs {
		log.Debugf("Running strace on container %d", containerPID)

		// Process the command template
		templateData := map[string]interface{}{"target_pid": fmt.Sprintf("%d", containerPID)}
		compiledCommand := tprintf(options.Command, templateData)

		// Write to a file with the container name
		podDir := fmt.Sprintf("%s%c%s", tracer.OutputDirectory, os.PathSeparator, tracer.TargetPod.GetName())
		iostream, err := getIOStreams(podDir, tracer.TargetPod.Spec.Containers[index].Name)
		if err != nil {
			return err
		}

		_, err = tracer.StartCollection(iostream, compiledCommand)

		if err != nil {
			return err
		}
	}

	log.Info("Strace complete")
	return err
}

func (tracer *NodeTracer) Start(options CollectionOptions) error {
	var err error
	ctx := context.TODO()

	tracer.tracePod, err = tracer.createCollectionPod(ctx, options)
	if err != nil {
		return err
	}

	// Write to a file with
	iostream, err := getIOStreams(tracer.OutputDirectory, tracer.TargetNode.GetName())
	if err != nil {
		return err
	}

	_, err = tracer.StartCollection(iostream, options.Command)

	if err != nil {
		return err
	}

	log.Info("Node collection complete for %s", tracer.TargetNode.GetName())
	return err
}

// Cleanup removes all  up the collection trace
func (tracer *DefaultTracer) Cleanup() {
	ctx := context.TODO()
	// Delete Pod
	if tracer == nil || tracer.tracePod == nil {
		return
	}

	err := tracer.Client.CoreV1().Pods(tracer.tracePod.Namespace).Delete(ctx, tracer.tracePod.Name, metav1.DeleteOptions{})
	if err != nil {
		log.Fatalf("unable to delete strace pod %q from namespace %q. manual deletion is required.", tracer.tracePod.Name, tracer.tracePod.Namespace)
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

	execRequest := ExecRequest{
		Client: tracer.Client, RestConfig: tracer.RestConfig, PodName: tracer.tracePod.Name,
		Namespace: tracer.tracePod.Namespace, Command: command, TTY: false, IOStreams: iostreams,
	}
	exitCode, err := ExecCommand(execRequest)

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
