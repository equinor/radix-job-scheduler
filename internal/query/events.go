package query

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
)

func ListEvents(ctx context.Context, namespace string, client kubeclient.Interface) ([]corev1.Event, error) {
	eventsList, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return eventsList.Items, nil
}
