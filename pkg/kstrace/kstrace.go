package kstrace

import (
	"time"

	"github.com/michaelwasher/library-collection/pkg/collection"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type KStracer struct {
	collection.PodTracer

	/// Strace Speciifc Options
	traceNamespace string
	traceImage     string
	socketPath     string
}
type PrivilegedPodOptions struct {
	Namespace     string
	ContainerName string
	Image         string
	NodeName      string
	SocketPath    string
}

func NewKStracer(clientset kubernetes.Interface, restConfig *rest.Config, traceImage string,
	targetPod *corev1.Pod, namespace string, socketPath string,
	timeout time.Duration, outputDirectory string,
	tracerPodSpec *corev1.PodSpec) KStracer {
	tracer := KStracer{
		PodTracer: collection.PodTracer{
			DefaultTracer: collection.DefaultTracer{
				RestConfig:        restConfig,
				Client:            clientset,
				CollectionTimeout: timeout,
				OutputDirectory:   outputDirectory,
				TracerPodSpec:     tracerPodSpec,
			},
			TargetPod: targetPod,
		},
		traceImage:     traceImage,
		traceNamespace: namespace,
		socketPath:     socketPath,
	}

	return tracer
}
