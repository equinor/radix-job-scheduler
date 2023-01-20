package api

import (
	corev1 "k8s.io/api/core/v1"
)

// DeleteConfigMap Deletes a configmap in a namespace
func (handler *Handler) DeleteConfigMap(configMap *corev1.ConfigMap) error {
	if configMap == nil {
		return nil
	}
	return handler.Kube.DeleteConfigMap(configMap.GetNamespace(), configMap.GetName())
}
