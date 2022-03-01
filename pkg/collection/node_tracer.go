package collection

import (
	"context"

	log "github.com/sirupsen/logrus"
)

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

	log.Infof("Node collection complete for %s", tracer.TargetNode.GetName())
	return err
}
