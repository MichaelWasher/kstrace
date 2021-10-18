package kstrace

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreatePrivilegedPod(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	tests := []struct {
		options         PrivilegedPodOptions
		name            string
		expectedFailure bool
	}{{
		options: PrivilegedPodOptions{
			Namespace:     "test-namespace",
			ContainerName: "container-name",
			Image:         "imagename",
			NodeName:      "nodename",
		},
		expectedFailure: false,
		name:            "Smoke Test",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pod, err := CreateStracePod(tc.options, clientset)
			if err != nil {
				t.Errorf("Pod creation failed. %v", err)
			}

			ctx := context.TODO()
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
