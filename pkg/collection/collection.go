package collection

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Tracer interface {
	Start(CollectionOptions) error
	Cleanup()
}

type CollectionOptions struct {
	Name      string
	Namespace string
	Image     string
	NodeName  string
	Command   string
}

type DefaultTracer struct {
	Client     kubernetes.Interface
	RestConfig *rest.Config

	// Trace Details
	tracePod      *corev1.Pod
	TracerPodSpec *corev1.PodSpec

	// Collection Defaults
	CollectionTimeout time.Duration
	OutputDirectory   string
}

type PodTracer struct {
	DefaultTracer

	// Target Details
	TargetPod           *corev1.Pod
	targetContainerPIDs []int64
}

type NodeTracer struct {
	DefaultTracer

	TargetNode *corev1.Node
}
