package v1

import (
	"fmt"
	commonUtils "github.com/equinor/radix-common/utils"
	commonErrors "github.com/equinor/radix-common/utils/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//GetJobOwnerReference Gets job as an OwnerReference
func GetJobOwnerReference(job *batchv1.Job) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Name:       job.GetName(),
		UID:        job.UID,
		Controller: commonUtils.BoolPtr(true),
	}
}

//UpdateOwnerReferenceOfSecret Update owner reference of secrets
func (handler *Handler) UpdateOwnerReferenceOfSecret(namespace string, ownerReference metav1.OwnerReference, secrets ...*corev1.Secret) error {
	var errs []error
	for _, secret := range secrets {
		if secret == nil {
			continue
		}
		secret.OwnerReferences = append(secret.OwnerReferences, ownerReference)
		_, err := handler.Kube.ApplySecret(namespace, secret)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed update OwnerReference for the secret %s: %s", secret.Name,
				err.Error()))
		}
	}
	if len(errs) == 0 {
		return nil
	}

	return commonErrors.Concat(errs)
}

//UpdateOwnerReferenceOfService Update owner reference of services
func (handler *Handler) UpdateOwnerReferenceOfService(namespace string, ownerReference metav1.OwnerReference,
	services ...*corev1.Service) error {
	var errs []error
	for _, secret := range services {
		if secret == nil {
			continue
		}
		secret.OwnerReferences = append(secret.OwnerReferences, ownerReference)
		err := handler.Kube.ApplyService(namespace, secret)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed update OwnerReference for the service %s: %s", secret.Name,
				err.Error()))
		}
	}
	if len(errs) == 0 {
		return nil
	}

	return commonErrors.Concat(errs)
}
