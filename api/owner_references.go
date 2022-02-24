package api

import (
	commonUtils "github.com/equinor/radix-common/utils"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getJobOwnerReferences(job *batchv1.Job) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: "batch/v1",
			Kind:       "Job",
			Name:       job.GetName(),
			UID:        job.UID,
			Controller: commonUtils.BoolPtr(true),
		},
	}
}
