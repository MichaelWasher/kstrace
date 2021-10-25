package kstrace

import (
	"bytes"
	"context"
	"os"

	"fmt"
	"strings"
	"time"

	"github.com/buger/jsonparser"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type KStracer struct {
	client         kubernetes.Interface
	targetPod      *corev1.Pod
	traceNamespace string
	tracePod       *corev1.Pod
	traceImage     string
	containerPIDs  []int64
	restConfig     *rest.Config
}

type PrivilegedPodOptions struct {
	Namespace     string
	ContainerName string
	Image         string
	NodeName      string
}

type Tracer interface {
	Start() error
	Stop() error
	Cleanup() error
}

func NewKStracer(clientset kubernetes.Interface, restConfig *rest.Config, targetPod *corev1.Pod, namespace string) *KStracer {
	straceObject := KStracer{
		traceImage:     "quay.io/mwasher/crictl:0.0.2",
		traceNamespace: namespace,
		restConfig:     restConfig,
		client:         clientset,
		targetPod:      targetPod,
	}

	return &straceObject
}

func getPodDefinition(options PrivilegedPodOptions) *corev1.Pod {
	// Change with different runtimes
	socketPath := "/run/crio/crio.sock"

	typeMetadata := metav1.TypeMeta{
		Kind:       "Pod",
		APIVersion: "v1",
	}

	objectMetadata := metav1.ObjectMeta{
		GenerateName: "kstrace-",
		Namespace:    options.Namespace,
		Labels: map[string]string{
			"app": "kstrace",
		},
	}

	// TODO: This should work with all dockershim / containerd / docker as they all speak OCI
	directoryType := corev1.HostPathSocket
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "runtime-socket",
			ReadOnly:  false,
			MountPath: "/run/crio/crio.sock",
		},
	}
	volumes := []corev1.Volume{
		{
			Name: "runtime-socket",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: socketPath,
					Type: &directoryType,
				},
			},
		},
	}
	// Create Privileged container
	privileged := true
	privilegedContainer := corev1.Container{
		Name:  options.ContainerName,
		Image: options.Image,

		SecurityContext: &corev1.SecurityContext{
			Privileged: &privileged,
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
		podStatus, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), pod, metav1.GetOptions{})
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

func (tracer *KStracer) Start() error {
	var err error
	ctx := context.TODO()

	options := PrivilegedPodOptions{
		Namespace:     tracer.traceNamespace,
		ContainerName: "container-name",
		Image:         tracer.traceImage,
		NodeName:      tracer.targetPod.Spec.NodeName,
	}
	tracer.tracePod, err = tracer.CreateStracePod(ctx, options)

	// Find out the PID for the requested Pod
	log.Infof("Running strace on pod %q", tracer.targetPod.Name)
	tracer.containerPIDs, err = tracer.FindPodPIDs()
	if err != nil {
		return err
	}

	// Run Strace for collected containerPIDs
	for _, containerPID := range tracer.containerPIDs {
		err = tracer.StartStrace(containerPID)
		if err != nil {
			return err
		}
	}

	return err
}

func (tracer *KStracer) Stop() error {
	return nil

}

func (tracer *KStracer) CreateStracePod(ctx context.Context, options PrivilegedPodOptions) (*corev1.Pod, error) {
	podDefinition := getPodDefinition(options)
	createdPod, err := tracer.client.CoreV1().Pods(options.Namespace).Create(ctx, podDefinition, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	log.Infof("creating privileged pod %q in namespace %q on node %q", createdPod.Name, createdPod.Namespace, createdPod.Spec.NodeName)
	log.Tracef("creating privileged pod with the following options: { %v }", options)

	err = waitForPodRunning(tracer.client, createdPod.Namespace, createdPod.Name)
	if err != nil {
		return nil, err
	}

	log.Infof("created pod %q successfully in namespace %q", createdPod.Name, createdPod.Namespace)
	log.Tracef("created pod details: %v", createdPod)
	return createdPod, nil
}

func (tracer *KStracer) StartStrace(targetPID int64) error {
	// TODO Replace with other stream
	stdOut := os.Stdout
	stdErr := os.Stderr

	command := fmt.Sprintf("strace -fp %d", targetPID)
	log.Infof("Running command %q inside pod %q", command, tracer.tracePod.Name)

	execRequest := ExecRequest{
		Client: tracer.client, RestConfig: tracer.restConfig, PodName: tracer.tracePod.Name,
		Namespace: tracer.tracePod.Namespace, Command: command, Stdin: nil,
		Stdout: stdOut, Stderr: stdErr, TTY: false,
	}
	exitCode, err := ExecCommand(execRequest)

	if exitCode != 0 {
		return fmt.Errorf("the function has failed with exit code: %d", exitCode)
	}

	if err != nil {
		return err
	}

	return nil
}
func (tracer *KStracer) Cleanup() error {
	ctx := context.TODO()
	// Delete Pod
	err := tracer.client.CoreV1().Pods(tracer.tracePod.Namespace).Delete(ctx, tracer.tracePod.Name, metav1.DeleteOptions{})
	if err != nil {
		log.Fatalf("unable to delete strace pod %q from namespace %q. manual deletion is required.", tracer.tracePod.Name, tracer.tracePod.Namespace)
		return err
	}
	tracer.tracePod = nil
	return nil
}

func (tracer *KStracer) FindPodPIDs() ([]int64, error) {
	// Specific to Crictl
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	// Get all Container IDs for Pod
	containerPIDs := []int64{}

	for _, containerStatus := range tracer.targetPod.Status.ContainerStatuses {
		containerID := strings.SplitAfter(containerStatus.ContainerID, "//")[1]

		command := fmt.Sprintf("crictl inspect %s", containerID)
		log.Infof("Running command %q inside pod %q", command, tracer.tracePod.Name)

		execRequest := ExecRequest{
			Client: tracer.client, RestConfig: tracer.restConfig, PodName: tracer.tracePod.Name,
			Namespace: tracer.tracePod.Namespace, Command: command, Stdin: nil,
			Stdout: stdOut, Stderr: stdErr, TTY: false,
		}
		exitCode, err := ExecCommand(execRequest)
		if exitCode != 0 || err != nil {
			return nil, err
		}

		// Process JSON
		json := stdOut.Bytes()
		containerPID, err := jsonparser.GetInt(json, "info", "pid")
		if err != nil {
			return nil, err
		}

		log.Infof("Container PID %q found for Pod %q", containerPID, containerID)
		containerPIDs = append(containerPIDs, containerPID)

	}
	// Run command in Pod
	return containerPIDs, nil
}
