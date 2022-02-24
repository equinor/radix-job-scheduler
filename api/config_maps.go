package api

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

//UpdateOwnerReferenceOfConfigMaps Update owner reference of a config-map
func (model *Model) UpdateOwnerReferenceOfConfigMaps(ownerJob *batchv1.Job, configMaps ...*corev1.ConfigMap) error {
	jobOwnerReferences := getJobOwnerReferences(ownerJob)
	for _, configMap := range configMaps {
		configMap.OwnerReferences = jobOwnerReferences
	}
	return model.Kube.UpdateConfigMap(ownerJob.ObjectMeta.GetNamespace(), configMaps...)
}
