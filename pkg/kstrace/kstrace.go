package kstrace

import (
	"bytes"
	"context"
	"errors"
	"os"

	"fmt"
	"strings"
	"time"

	"github.com/buger/jsonparser"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type KStracer struct {
	client            kubernetes.Interface
	targetPod         *corev1.Pod
	traceNamespace    string
	tracePod          *corev1.Pod
	traceImage        string
	containerPIDs     []int64
	restConfig        *rest.Config
	socketPath        string
	collectionTimeout time.Duration
	outputDirectory   string
}

type PrivilegedPodOptions struct {
	Namespace     string
	ContainerName string
	Image         string
	NodeName      string
	SocketPath    string
}

type Tracer interface {
	Start() error
	Stop() error
	Cleanup() error
}

func NewKStracer(clientset kubernetes.Interface, restConfig *rest.Config, traceImage string, targetPod *corev1.Pod, namespace string, socketPath string,
	timeout time.Duration, outputDirectory string) Tracer {
	straceObject := KStracer{
		traceImage:     traceImage,
		traceNamespace: namespace,
		restConfig:     restConfig,
		client:         clientset,
		targetPod:      targetPod,
		socketPath:     socketPath,
		// TODO: the following have not been used
		collectionTimeout: timeout,
		outputDirectory:   outputDirectory,
	}

	return &straceObject
}

func getPodDefinition(options PrivilegedPodOptions) *corev1.Pod {
	// Change with different runtimes
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

	// Mount the CRI socket through
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
					Path: options.SocketPath,
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
	timeout := time.Second * 30

	checkPodState := func() bool {

		podStatus, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), pod, metav1.GetOptions{})
		if err != nil {
			return false
		}

		if podStatus.Status.Phase == corev1.PodRunning {
			return true
		}

		return false
	}

	for timeout > 0 {
		log.Debugf("Waiting for Tracer Pod %q to become ready", pod)
		if checkPodState() {
			break
		}

		// Sleep
		time.Sleep(time.Second * 2)
		timeout -= time.Second * 2
	}

	if timeout <= 0 {
		return fmt.Errorf("tracer pod did not start correctly. review event objects from namespace %q relating to pod %q for more information", namespace, pod)
	}
	return nil
}

func (tracer *KStracer) getIOStream(container string) (*genericclioptions.IOStreams, error) {
	var err error

	// Special case for std-out
	if tracer.outputDirectory == "-" {
		return &genericclioptions.IOStreams{Out: os.Stdout, In: nil, ErrOut: os.Stderr}, nil
	}

	// Ensure trace folder is present
	if _, err := os.Stat(tracer.outputDirectory); errors.Is(err, os.ErrNotExist) {
		err = os.Mkdir(tracer.outputDirectory, 0775)
		if err != nil {
			log.Infof("Unable to create directory for the strace collection. %v", err)
			return nil, err
		}
	}

	// Ensure Pod-folder is present
	podTraceFolder := fmt.Sprintf("%s%c%s", tracer.outputDirectory, os.PathSeparator, tracer.targetPod.Name)
	if _, err := os.Stat(podTraceFolder); errors.Is(err, os.ErrNotExist) {
		err = os.Mkdir(podTraceFolder, 0775)
		if err != nil {
			log.Infof("Unable to create directory for the strace collection. %v", err)
			return nil, err
		}
	}
	// Create file for container trace
	fileWriter, err := os.Create(fmt.Sprintf("%s/%s/%s_strace.log", tracer.outputDirectory, tracer.targetPod.Name, container))
	if err != nil {
		log.Infof("Unable to create logfile for the strace collection. %v", err)
		return nil, err
	}

	return &genericclioptions.IOStreams{
		Out:    fileWriter,
		ErrOut: fileWriter,
		In:     nil,
	}, nil
}

func (tracer *KStracer) Start() error {
	var err error
	ctx := context.TODO()

	// Create the Strace Pod
	options := PrivilegedPodOptions{
		Namespace:     tracer.traceNamespace,
		ContainerName: "container-name",
		Image:         tracer.traceImage,
		NodeName:      tracer.targetPod.Spec.NodeName,
		SocketPath:    tracer.socketPath,
	}
	tracer.tracePod, err = tracer.CreateStracePod(ctx, options)
	if err != nil {
		return err
	}

	// Find out the PID for the requested Pod
	log.Infof("Running strace on pod %q", tracer.targetPod.Name)
	tracer.containerPIDs, err = tracer.FindPodPIDs()
	if err != nil {
		return err
	}

	// Run Strace for collected containerPIDs
	for index, containerPID := range tracer.containerPIDs {
		log.Debugf("Running strace on container %d", containerPID)

		// Write to a file with the container name
		iostream, err := tracer.getIOStream(tracer.targetPod.Spec.Containers[index].Name)
		if err != nil {
			return err
		}

		err = tracer.StartStrace(containerPID, iostream)
		if err != nil {
			return err
		}
	}

	log.Info("Strace complete")
	return err
}

func (tracer *KStracer) Stop() error {
	return nil

}

func (tracer *KStracer) CreateStracePod(ctx context.Context, options PrivilegedPodOptions) (*corev1.Pod, error) {
	// TODO: ensure that the target pod is actually active
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

func (tracer *KStracer) StartStrace(targetPID int64, iostreams *genericclioptions.IOStreams) error {
	command := fmt.Sprintf("strace -fp %d", targetPID)
	log.Infof("Running command %q inside pod %q", command, tracer.tracePod.Name)

	execRequest := ExecRequest{
		Client: tracer.client, RestConfig: tracer.restConfig, PodName: tracer.tracePod.Name,
		Namespace: tracer.tracePod.Namespace, Command: command, TTY: false, IOStreams: iostreams,
	}
	exitCode, err := ExecCommand(execRequest)

	if exitCode != 0 {
		return fmt.Errorf("the function has failed with exit code: %d", exitCode)
	}
	log.Infof("Strace command for Pod %q complete", tracer.tracePod.Name)

	if err != nil {
		return err
	}

	return nil
}
func (tracer *KStracer) Cleanup() error {
	ctx := context.TODO()
	// Delete Pod
	if tracer == nil || tracer.tracePod == nil {
		return nil
	}

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
	iostreams := &genericclioptions.IOStreams{
		In: nil, Out: new(bytes.Buffer), ErrOut: new(bytes.Buffer),
	}

	// Get all Container IDs for Pod
	containerPIDs := []int64{}

	for _, containerStatus := range tracer.targetPod.Status.ContainerStatuses {

		containerID := strings.SplitAfter(containerStatus.ContainerID, "//")[1]

		command := fmt.Sprintf("crictl inspect %s", containerID)
		log.Infof("Running command %q inside pod %q", command, tracer.tracePod.Name)

		execRequest := ExecRequest{
			Client: tracer.client, RestConfig: tracer.restConfig, PodName: tracer.tracePod.Name,
			Namespace: tracer.tracePod.Namespace, Command: command, IOStreams: iostreams, TTY: false,
		}
		exitCode, err := ExecCommand(execRequest)
		if exitCode != 0 || err != nil {
			return nil, err
		}

		// Process JSON
		// TODO perform checks against casting
		json := iostreams.Out.(*bytes.Buffer).Bytes()

		containerPID, err := jsonparser.GetInt(json, "info", "pid")
		if err != nil {
			return nil, err
		}

		log.Infof("Container PID %d found for Container %q", containerPID, containerID)
		containerPIDs = append(containerPIDs, containerPID)

	}

	if len(containerPIDs) < 1 {
		log.Errorf("No container PIDs found for Pod %q", tracer.targetPod)
		return nil, fmt.Errorf("no container pids found from %q", tracer.targetPod)
	}

	// Run command in Pod
	return containerPIDs, nil
}
