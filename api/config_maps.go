package api

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//UpdateOwnerReferenceOfConfigMaps Update owner reference of a config-map
func (model *Model) UpdateOwnerReferenceOfConfigMaps(ownerJob *batchv1.Job, configMaps ...*corev1.ConfigMap) error {
	jobOwnerReference := GetJobOwnerReference(ownerJob)
	for _, configMap := range configMaps {
		configMap.OwnerReferences = []metav1.OwnerReference{jobOwnerReference}
	}
	return model.Kube.UpdateConfigMap(ownerJob.ObjectMeta.GetNamespace(), configMaps...)
}
