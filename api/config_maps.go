package api

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteConfigMaps Deletes a configmap in a namespace
func (handler *Handler) DeleteConfigMaps(configMap *corev1.ConfigMap) error {
	return handler.KubeClient.CoreV1().ConfigMaps(configMap.Namespace).Delete(context.Background(), configMap.Name,
		metav1.DeleteOptions{})
}
