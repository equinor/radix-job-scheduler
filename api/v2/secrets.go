package apiv2

import (
	"context"

	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	corev1 "k8s.io/api/core/v1"
	kubeLabels "k8s.io/apimachinery/pkg/labels"
)

// GetSecretsForRadixBatch Get secrets for the RadixBatch
func (h *handler) GetSecretsForRadixBatch(ctx context.Context, batchName string) ([]*corev1.Secret, error) {
	selector, err := h.kubeUtil.ListSecretsWithSelector(ctx, h.env.RadixDeploymentNamespace, getLabelSelectorForRadixBatchSecret(batchName))
	if err != nil {
		return nil, apiErrors.NewFromError(err)
	}
	return selector, nil
}

// DeleteSecret Delete the service
func (h *handler) DeleteSecret(ctx context.Context, secret *corev1.Secret) error {
	err := h.kubeUtil.DeleteSecret(ctx, secret.Namespace, secret.Name)
	if err != nil {
		return apiErrors.NewFromError(err)
	}
	return nil
}

func getLabelSelectorForRadixBatchSecret(batchName string) string {
	return kubeLabels.SelectorFromSet(labels.ForBatchName(batchName)).String()
}
