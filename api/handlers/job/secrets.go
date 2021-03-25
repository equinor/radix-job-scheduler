package job

import (
	"fmt"

	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (jh *jobHandler) createPayloadSecret(jobName string, jobComponent *v1.RadixDeployJobComponent, rd *v1.RadixDeployment, jobScheduleDescription *models.JobScheduleDescription) (*corev1.Secret, error) {
	secretName := getPayloadSecretName(jobName)
	secret := buildPayloadSecretSpec(secretName, jobScheduleDescription.Payload, jobName, rd.Spec.AppName, jobComponent.Name)
	return jh.kube.ApplySecret(jh.env.RadixDeploymentNamespace, secret)
}

func (jh *jobHandler) getSecretsForJob(jobName string) (*corev1.SecretList, error) {
	return jh.kubeClient.CoreV1().Secrets(jh.env.RadixDeploymentNamespace).List(metav1.ListOptions{
		LabelSelector: getLabelSelectorForSecret(jobName, jh.env.RadixComponentName),
	})
}

func (jh *jobHandler) deleteSecret(secret *corev1.Secret) error {
	return jh.kubeClient.CoreV1().Secrets(secret.Namespace).Delete(secret.Name, &metav1.DeleteOptions{})
}

func getLabelSelectorForSecret(jobName, componentName string) string {
	return labels.SelectorFromSet(labels.Set(map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobNameLabel:   jobName,
	})).String()
}

func buildPayloadSecretSpec(secretName, payload, jobName, appName, componentName string) *corev1.Secret {
	secret := corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:       appName,
				kube.RadixComponentLabel: componentName,
				kube.RadixJobNameLabel:   jobName,
			},
		},
		Data: map[string][]byte{
			JOB_PAYLOAD_PROPERTY_NAME: []byte(payload),
		},
	}
	return &secret
}

func getPayloadSecretName(jobName string) string {
	return fmt.Sprintf("%s-%s", jobName, JOB_PAYLOAD_PROPERTY_NAME)
}
