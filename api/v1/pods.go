package v1

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (handler *Handler) GetPodsForLabelSelector(ctx context.Context, labelSelector string) ([]corev1.Pod, error) {
	podList, err := handler.Kube.KubeClient().
		CoreV1().
		Pods(handler.Env.RadixDeploymentNamespace).
		List(
			ctx,
			metav1.ListOptions{LabelSelector: labelSelector},
		)

	if err != nil {
		return nil, err
	}

	return podList.Items, nil
}
