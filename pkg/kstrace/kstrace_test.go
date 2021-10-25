package kstrace

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	testingcore "k8s.io/client-go/testing"
)

func SetPodStatusPhaseRunning(action testingcore.Action) (handled bool, ret runtime.Object, err error) {
	createAction := action.(testingcore.CreateAction)
	obj := createAction.GetObject().(*corev1.Pod)
	obj.Status.Phase = corev1.PodRunning
	return false, obj, nil
}

// TODO: Add test to ensure strace Pod and Namespace are cleaned up correctly
func TestCreatePrivilegedPod(t *testing.T) {

	tests := []struct {
		options          PrivilegedPodOptions
		name             string
		expectedFailure  bool
		prependReactions []testingcore.ReactionFunc
		appendReactions  []testingcore.ReactionFunc
	}{{
		options: PrivilegedPodOptions{
			Namespace:     "test-namespace",
			ContainerName: "container-name",
			Image:         "imagename",
			NodeName:      "nodename",
		},
		expectedFailure:  false,
		name:             "Smoke Test",
		prependReactions: []testingcore.ReactionFunc{SetPodStatusPhaseRunning},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.TODO()
			clientset := fake.NewSimpleClientset()

			for _, reactor := range tc.prependReactions {
				clientset.PrependReactor("create", "pods", reactor)
			}
			tracer := KStracer{
				client: clientset,
			}

			pod, err := tracer.CreateStracePod(ctx, tc.options)
			if err != nil {
				t.Errorf("Pod creation failed. %v", err)
			}

			returnedPod, err := clientset.CoreV1().Pods(tc.options.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("Unable to get the Pod. %v", err)
			}
			if returnedPod.Spec.Containers[0].Name != tc.options.ContainerName {
				t.Errorf("Container name mismatch. Expected %q, Got: %q", tc.options.ContainerName, returnedPod.Spec.Containers[0].Name)
			}
			if returnedPod.Spec.NodeName != tc.options.NodeName {
				t.Errorf("Node name mismatch. Expected %q, Got: %q", tc.options.NodeName, returnedPod.Spec.NodeName)
			}
			if returnedPod.Spec.Containers[0].Image != tc.options.Image {
				t.Errorf("Image mismatch. Expected %q, Got: %q", tc.options.Image, returnedPod.Spec.Containers[0].Image)
			}
		})
	}
}
func TestCreateNamespace(t *testing.T) {
	ctx := context.TODO()
	clientset := fake.NewSimpleClientset()
	namespace, err := CreateNamespace(ctx, clientset)
	if err != nil {
		t.Errorf("Unable to create namespace. %v", err)
	}

	resultNamespace, err := clientset.CoreV1().Namespaces().Get(ctx, namespace.Name, metav1.GetOptions{})
	if err != nil || resultNamespace.Name != namespace.Name {
		t.Errorf("Unable to find the created namespace. %v", err)
	}

}
