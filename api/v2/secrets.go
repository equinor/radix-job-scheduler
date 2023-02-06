package apiv2

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

//GetSecretsForRadixBatch Get secrets for the RadixBatch
func (h *handler) GetSecretsForRadixBatch(batchName string) ([]*corev1.Secret, error) {
	return h.Kube.ListSecretsWithSelector(h.Env.RadixDeploymentNamespace, getLabelSelectorForRadixBatchSecret(batchName))
}

//DeleteSecret Delete the service
func (h *handler) DeleteSecret(secret *corev1.Secret) error {
	return h.Kube.DeleteSecret(secret.Namespace, secret.Name)
}

func getLabelSelectorForRadixBatchSecret(batchName string) string {
	return labels.SelectorFromSet(map[string]string{
		kube.RadixBatchNameLabel: batchName,
	}).String()
}
