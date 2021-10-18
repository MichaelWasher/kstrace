package kstrace

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type KStrace struct {
}

type PrivilegedPodOptions struct {
	Namespace     string
	ContainerName string
	Image         string
	NodeName      string
}

func getPodDefinition(options PrivilegedPodOptions) *corev1.Pod {
	typeMetadata := v1.TypeMeta{
		Kind:       "Pod",
		APIVersion: "v1",
	}

	objectMetadata := v1.ObjectMeta{
		GenerateName: "kstrace-",
		Namespace:    options.Namespace,
		Labels: map[string]string{
			"app": "kstrace",
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "host",
			ReadOnly:  false,
			MountPath: "/host",
		},
	}
	directoryType := corev1.HostPathDirectory
	volumes := []corev1.Volume{
		{
			Name: "host",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/",
					Type: &directoryType,
				},
			},
		},
	}
	// Create Privileged container
	privilegedContainer := corev1.Container{
		Name:  options.ContainerName,
		Image: options.Image,

		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					corev1.Capability("SYS_ADMIN"),
					corev1.Capability("SYS_PTRACE"),
				},
			},
		},

		Command:      []string{"sh", "-c", "sleep 10000000"},
		VolumeMounts: volumeMounts,
	}

	podSpecs := corev1.PodSpec{
		NodeName:      options.NodeName,
		RestartPolicy: corev1.RestartPolicyNever,
		HostPID:       true,
		Containers:    []corev1.Container{privilegedContainer},
		Volumes:       volumes,
	}

	pod := corev1.Pod{
		TypeMeta:   typeMetadata,
		ObjectMeta: objectMetadata,
		Spec:       podSpecs,
	}
	return &pod
}

func waitForPodRunning(clientset kubernetes.Interface, namespace string, pod string) error {
	checkPodState := func() bool {
		// TODO WaitGroup
		podStatus, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), pod, v1.GetOptions{})
		if err != nil {
			return false
		}

		if podStatus.Status.Phase == corev1.PodRunning {
			return true
		}

		return false
	}

	// TODO Replace with WaitGroup
	for {
		if checkPodState() {
			break
		}
		time.Sleep(time.Second)
	}
	return nil
}

func CreateStracePod(ctx context.Context, options PrivilegedPodOptions, clientset kubernetes.Interface) (*corev1.Pod, error) {
	podDefinition := getPodDefinition(options)
	createdPod, err := clientset.CoreV1().Pods(options.Namespace).Create(ctx, podDefinition, v1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	log.Infof("creating privileged pod %q in namespace %q on node %q", createdPod.Name, createdPod.Namespace, createdPod.Spec.NodeName)
	log.Tracef("creating privileged pod with the following options: { %v }", options)

	err = waitForPodRunning(clientset, createdPod.Namespace, createdPod.Name)
	if err != nil {
		return nil, err
	}

	log.Infof("created pod %q successfully in namespace %q", createdPod.Name, createdPod.Namespace)
	log.Tracef("created pod details: %v", createdPod)
	return createdPod, nil
}
