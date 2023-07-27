package v1

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetSecretsForJob Get secrets for the job
func (handler *Handler) GetSecretsForJob(ctx context.Context, jobName string) (*corev1.SecretList, error) {
	return handler.Kube.KubeClient().CoreV1().Secrets(handler.Env.RadixDeploymentNamespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: getLabelSelectorForSecret(jobName, handler.Env.RadixComponentName),
		},
	)
}

// DeleteSecret Delete the service for the job
func (handler *Handler) DeleteSecret(ctx context.Context, secret *corev1.Secret) error {
	return handler.Kube.KubeClient().CoreV1().Secrets(secret.Namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{})
}

func getLabelSelectorForSecret(jobName, componentName string) string {
	return labels.SelectorFromSet(map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobNameLabel:   jobName,
	}).String()
}
