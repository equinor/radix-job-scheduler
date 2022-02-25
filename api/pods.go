package api

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (model *Model) GetPodsForLabelSelector(labelSelector string) ([]corev1.Pod, error) {
	podList, err := model.KubeClient.
		CoreV1().
		Pods(model.Env.RadixDeploymentNamespace).
		List(
			context.TODO(),
			metav1.ListOptions{LabelSelector: labelSelector},
		)

	if err != nil {
		return nil, err
	}

	return podList.Items, nil
}
