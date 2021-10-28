package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/michaelwasher/kube-strace/pkg/kstrace"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
)

type KubeStraceCommandArgs struct {
	traceImage *string
	socketPath *string
}
type KubeStraceCommand struct {
	KubeStraceCommandArgs
	clientset  *kubernetes.Clientset
	targetPods []corev1.Pod

	builder    *resource.Builder
	restConfig *rest.Config

	kubeConfigFlags *genericclioptions.ConfigFlags

	tracers  []*kstrace.KStracer
	loglevel log.Level
}

func NewKubeStraceCommand() *cobra.Command {
	kCmd := &KubeStraceCommand{
		loglevel: log.TraceLevel,
	}

	cmd := &cobra.Command{
		Use:   "kubectl-strace",
		Short: "Run strace against Pods and Deployments in Kubernetes",
		Long: `kubectl-strace is a CLI tool that provides the ability to easily perform
		debugging of system-calls and process state for applications running on the Kubernetes platform.`,
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := kCmd.Complete(cmd, args); err != nil {
				return err
			}
			if err := kCmd.Validate(); err != nil {
				return err
			}
			if err := kCmd.Run(); err != nil {
				return err
			}

			return nil
		},
	}
	// Add Kubectl / Kubernetes CLI flags
	flags := cmd.PersistentFlags()
	kCmd.kubeConfigFlags = genericclioptions.NewConfigFlags(true)
	kCmd.kubeConfigFlags.AddFlags(flags)

	// Add command-specific flags
	kCmd.socketPath = flags.String("socket-path", "/run/crio/crio.sock", "The location of the CRI socket on the host machine. The defaults for common runtimes are as below: [ default /run/crio/crio.sock ] used to mount through")
	kCmd.traceImage = flags.String("image", "quay.io/mwasher/crictl:0.0.2", "The trace image for use when performing the strace.")

	return cmd
}

func (kCmd *KubeStraceCommand) configureClientset() error {
	var err error

	configFlags := genericclioptions.NewConfigFlags(true)
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: configFlags.ToRawKubeConfigLoader().ConfigAccess().GetDefaultFilename()},
		&clientcmd.ConfigOverrides{},
	).ClientConfig()

	if err != nil {
		return err
	}
	restConfig.Timeout = 30 * time.Second

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	kCmd.clientset, kCmd.restConfig = clientset, restConfig

	return nil
}
func (kCmd *KubeStraceCommand) Complete(cmd *cobra.Command, args []string) error {
	var err error

	if kCmd.loglevel != log.InfoLevel {
		log.Infof("Running with loglevel: %v", kCmd.loglevel)
		log.SetLevel(kCmd.loglevel)
	}

	err = kCmd.configureClientset()
	if err != nil {
		return err
	}

	// Create flag factory
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kCmd.kubeConfigFlags)
	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

	// TODO Deal with -n Namespace overwrites
	namespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	kCmd.builder = f.NewBuilder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		ResourceNames("pod", args...).NamespaceParam(namespace).DefaultNamespace()

	return nil
}

func (kCmd *KubeStraceCommand) Validate() error {
	// TODO Perform Validation on the Flags
	return nil
}

func (kCmd *KubeStraceCommand) Run() error {
	var err error
	ctx := context.TODO()

	// Collect target pods
	kCmd.targetPods, err = processResources(kCmd.builder, kCmd.clientset)
	if err != nil {
		return err
	}

	// Create namespace for Strace Pods
	ns, err := kstrace.CreateNamespace(ctx, kCmd.clientset)
	defer kstrace.CleanupNamespace(ctx, kCmd.clientset, ns.Name)

	if err != nil {
		return err
	}

	// Create Tracers for each Pod
	for _, targetPod := range kCmd.targetPods {
		tracer := kstrace.NewKStracer(kCmd.clientset, kCmd.restConfig, *kCmd.traceImage, &targetPod, ns.Name, *kCmd.socketPath)

		kCmd.tracers = append(kCmd.tracers, tracer)
	}

	// TODO: Configure the desired output streams
	for _, tracer := range kCmd.tracers {
		// TODO Place in goroutine
		err = tracer.Start()

		// Configure Cleanup
		defer tracer.Cleanup()
		defer tracer.Stop()

		if err != nil {
			return err
		}
	}

	return nil
}

func processResources(builder *resource.Builder, clientset *kubernetes.Clientset) ([]corev1.Pod, error) {
	r := builder.Do()
	podSlice := []corev1.Pod{}
	err := r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			// TODO(verb): configurable early return
			return err
		}
		var visitErr error

		switch obj := info.Object.(type) {

		case *corev1.Pod:
			log.Debugf("Adding pod to strace list %v", obj)
			podSlice = append(podSlice, *obj)

		default:
			visitErr = fmt.Errorf("%q not supported by kstrace", info.Mapping.GroupVersionKind)
		}
		if visitErr != nil {
			return visitErr
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	/// Build the list of Nodes and Pods to select from; With
	log.Debugf("Pod List: '%v'", podSlice)

	return podSlice, nil
}
