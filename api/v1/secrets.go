package v1

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/equinor/radix-job-scheduler/models"
	defaultsv1 "github.com/equinor/radix-job-scheduler/models/v1/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

//CreatePayloadSecret Create a secret for the job payload
func (handler *Handler) CreatePayloadSecret(appName, jobName string, jobComponent *v1.RadixDeployJobComponent,
	jobScheduleDescription *models.JobScheduleDescription) (*corev1.Secret, error) {
	if !isPayloadDefinedForJobComponent(jobComponent) {
		return nil, nil
	}

	secretName := defaultsv1.GetPayloadSecretName(jobName)
	secret := buildPayloadSecretSpec(appName, jobName, jobComponent.Name, secretName, jobScheduleDescription.Payload)
	savedSecret, err := handler.Kube.ApplySecret(handler.Env.RadixDeploymentNamespace, secret)
	return savedSecret, err
}

//CreateBatchScheduleDescriptionSecret Create a secret for the batch schedule description
func (handler *Handler) CreateBatchScheduleDescriptionSecret(appName, batchName, jobComponentName string,
	batchScheduleDescription *models.BatchScheduleDescription) (*corev1.Secret, error) {
	secret, err := buildBatchScheduleDescriptionSecretSpec(batchName, appName, jobComponentName, batchScheduleDescription)
	if err != nil {
		return nil, err
	}
	return handler.Kube.ApplySecret(handler.Env.RadixDeploymentNamespace, secret)
}

//GetSecretsForJob Get secrets for the job
func (handler *Handler) GetSecretsForJob(jobName string) (*corev1.SecretList, error) {
	return handler.KubeClient.CoreV1().Secrets(handler.Env.RadixDeploymentNamespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: getLabelSelectorForSecret(jobName, handler.Env.RadixComponentName),
		},
	)
}

//DeleteSecret Delete the service for the job
func (handler *Handler) DeleteSecret(secret *corev1.Secret) error {
	return handler.KubeClient.CoreV1().Secrets(secret.Namespace).Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
}

func getLabelSelectorForSecret(jobName, componentName string) string {
	return labels.SelectorFromSet(map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobNameLabel:   jobName,
	}).String()
}

func buildPayloadSecretSpec(appName, jobName, componentName, secretName, payload string) *corev1.Secret {
	secret := corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:       appName,
				kube.RadixComponentLabel: componentName,
				kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
				kube.RadixJobNameLabel:   jobName,
			},
		},
		Data: map[string][]byte{
			defaultsv1.JobPayloadPropertyName: []byte(payload),
		},
	}
	return &secret
}

func isPayloadDefinedForJobComponent(radixJobComponent *v1.RadixDeployJobComponent) bool {
	return radixJobComponent.Payload != nil && strings.TrimSpace(radixJobComponent.Payload.Path) != ""
}

func buildBatchScheduleDescriptionSecretSpec(batchName, appName, componentName string, batchScheduleDescription *models.BatchScheduleDescription) (*corev1.Secret, error) {
	secretName := defaultsv1.GetBatchScheduleDescriptionSecretName(batchName)
	descriptionJson, err := json.Marshal(batchScheduleDescription)
	if err != nil {
		return nil, err
	}

	secret := corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				kube.RadixAppLabel:       appName,
				kube.RadixComponentLabel: componentName,
				kube.RadixJobTypeLabel:   kube.RadixJobTypeBatchSchedule,
				kube.RadixBatchNameLabel: batchName,
			},
		},
		Data: map[string][]byte{
			defaultsv1.BatchScheduleDescriptionPropertyName: descriptionJson,
		},
	}
	return &secret, nil
}
