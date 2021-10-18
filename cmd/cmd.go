package cmd

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
)

type KubeStraceCall struct {
	clientset *kubernetes.Clientset
	pods      []corev1.Pod
	builder   *resource.Builder
}

func NewKubeStraceCall() *KubeStraceCall {
	return &KubeStraceCall{}
}

func NewKubeStraceCommand() *cobra.Command {
	kStrace := NewKubeStraceCall()
	cmd := &cobra.Command{
		Use:   "kubectl-strace",
		Short: "Run strace against Pods and Deployments in Kubernetes",
		Long: `kubectl-strace is a CLI tool that provides the ability to easily perform
		debugging of system-calls and process state for applications running on the Kubernetes platform.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := kStrace.Complete(cmd, args); err != nil {
				return err
			}
			if err := kStrace.Validate(); err != nil {
				return err
			}
			if err := kStrace.Run(); err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func getClientSet() (*kubernetes.Clientset, error) {
	var err error

	configFlags := genericclioptions.NewConfigFlags(true)
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: configFlags.ToRawKubeConfigLoader().ConfigAccess().GetDefaultFilename()},
		&clientcmd.ConfigOverrides{},
	).ClientConfig()

	if err != nil {
		return nil, err
	}
	restConfig.Timeout = 30 * time.Second

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}
func (kStrace *KubeStraceCall) Complete(cmd *cobra.Command, args []string) error {
	var err error
	// TODO Parse the flags from the Cobra Command
	// TODO use this as a flag
	// if kStrace.LogLevel {
	log.Info("run in debug mode")
	log.SetLevel(log.DebugLevel)
	// }

	kStrace.clientset, err = getClientSet()
	if err != nil {
		return err
	}
	// Include the Kubectl factory for flags
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

	// TODO Deal with -n Namespace overwrites
	namespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	kStrace.builder = f.NewBuilder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		ResourceNames("pod", args...).NamespaceParam(namespace).DefaultNamespace()

	return nil
}

func (kStrace *KubeStraceCall) Validate() error {
	// TODO Perform Validation on the Flags
	return nil
}
func (kStrace *KubeStraceCall) Run() error {
	// TODO Main logic
	// TODO
	var err error

	kStrace.pods, err = processResources(kStrace.builder, kStrace.clientset)
	if err != nil {
		return err
	}

	log.Trace("Running strace on the following pods: %v", kStrace.pods)
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
