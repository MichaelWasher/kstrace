package kstrace

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	testingcore "k8s.io/client-go/testing"
)

func SetPodStatusPhaseRunning(action testingcore.Action) (handled bool, ret runtime.Object, err error) {
	createAction := action.(testingcore.CreateAction)
	obj := createAction.GetObject().(*corev1.Pod)
	obj.Status.Phase = corev1.PodRunning
	return false, obj, nil
}
