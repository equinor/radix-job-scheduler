package api

import (
	"context"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

//GetSecretsForJob Get secrets for the job
func (h *handler) GetSecretsForJob(jobName string) (*corev1.SecretList, error) {
	return h.KubeClient.CoreV1().Secrets(h.Env.RadixDeploymentNamespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: getLabelSelectorForSecret(jobName, h.Env.RadixComponentName),
		},
	)
}

//DeleteSecret Delete the service for the job
func (h *handler) DeleteSecret(secret *corev1.Secret) error {
	return h.KubeClient.CoreV1().Secrets(secret.Namespace).Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
}

func getLabelSelectorForSecret(jobName, componentName string) string {
	return labels.SelectorFromSet(map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobNameLabel:   jobName,
	}).String()
}
