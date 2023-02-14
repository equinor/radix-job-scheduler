package apiv2

import (
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	corev1 "k8s.io/api/core/v1"
	kubeLabels "k8s.io/apimachinery/pkg/labels"
)

//GetSecretsForRadixBatch Get secrets for the RadixBatch
func (h *handler) GetSecretsForRadixBatch(batchName string) ([]*corev1.Secret, error) {
	return h.kubeUtil.ListSecretsWithSelector(h.env.RadixDeploymentNamespace, getLabelSelectorForRadixBatchSecret(batchName))
}

//DeleteSecret Delete the service
func (h *handler) DeleteSecret(secret *corev1.Secret) error {
	return h.kubeUtil.DeleteSecret(secret.Namespace, secret.Name)
}

func getLabelSelectorForRadixBatchSecret(batchName string) string {
	return kubeLabels.SelectorFromSet(labels.ForBatchName(batchName)).String()
}
