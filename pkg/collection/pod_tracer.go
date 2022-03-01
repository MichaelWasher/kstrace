package collection

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

func (tracer *PodTracer) checkConfig(options CollectionOptions) error {
	// TODO Ensure that Pod Spec and required values have been set
	if tracer.TracerPodSpec == nil {
		return fmt.Errorf("the Pod Spec needs to be added the tracer")
	}
	return nil
}

// Public function for starting the collection trace
// CollectionOptions.Command is used as a template
// Options: {target_pid}
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

	// Run tracer against Pods in targetContainerPIDs
	for index, containerPID := range tracer.targetContainerPIDs {
		log.Debugf("Running collection for PID %d", containerPID)

		// Process the command template
		templateData := map[string]interface{}{"target_pid": fmt.Sprintf("%d", containerPID)}
		compiledCommand := tprintf(options.Command, templateData)

		// Write to a file with the container name
		podDir := tracer.OutputDirectory
		if podDir != "-" {
			podDir = fmt.Sprintf("%s%c%s", tracer.OutputDirectory, os.PathSeparator, tracer.TargetPod.GetName())
		}

		iostream, err := getIOStreams(podDir, tracer.TargetPod.Spec.Containers[index].Name)
		if err != nil {
			return err
		}

		_, err = tracer.StartCollection(iostream, compiledCommand)

		if err != nil {
			return err
		}
	}

	log.Info("Collection complete")
	return err
}
