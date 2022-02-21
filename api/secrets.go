package api

import (
	"context"
	"encoding/json"
	"github.com/equinor/radix-job-scheduler/models"
	"strings"

	"github.com/equinor/radix-job-scheduler/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

//CreatePayloadSecret Create a secret for the job payload
func (model *Model) CreatePayloadSecret(jobName string, jobComponent *v1.RadixDeployJobComponent,
	rd *v1.RadixDeployment, jobScheduleDescription *models.JobScheduleDescription) (*corev1.Secret, error) {
	if !isPayloadDefinedForJobComponent(jobComponent) {
		return nil, nil
	}

	secretName := defaults.GetPayloadSecretName(jobName)
	secret := buildPayloadSecretSpec(secretName, jobScheduleDescription.Payload, jobName, rd.Spec.AppName, jobComponent.Name)
	return model.Kube.ApplySecret(model.Env.RadixDeploymentNamespace, secret)
}

//CreateBatchScheduleDescriptionSecret Create a secret for the batch schedule description
func (model *Model) CreateBatchScheduleDescriptionSecret(batchName string, jobComponent *v1.RadixDeployJobComponent, rd *v1.RadixDeployment, batchScheduleDescription *models.BatchScheduleDescription) (*corev1.Secret, error) {
	secret, err := buildBatchScheduleDescriptionSecretSpec(batchName, rd.Spec.AppName, jobComponent.Name, batchScheduleDescription)
	if err != nil {
		return nil, err
	}
	return model.Kube.ApplySecret(model.Env.RadixDeploymentNamespace, secret)
}

//GetSecretsForJob Get secrets for the job
func (model *Model) GetSecretsForJob(jobName string) (*corev1.SecretList, error) {
	return model.KubeClient.CoreV1().Secrets(model.Env.RadixDeploymentNamespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: getLabelSelectorForSecret(jobName, model.Env.RadixComponentName),
		},
	)
}

//DeleteSecret Delete the service for the job
func (model *Model) DeleteSecret(secret *corev1.Secret) error {
	return model.KubeClient.CoreV1().Secrets(secret.Namespace).Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
}

func getLabelSelectorForSecret(jobName, componentName string) string {
	return labels.SelectorFromSet(map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobNameLabel:   jobName,
	}).String()
}

func buildPayloadSecretSpec(secretName, payload, jobName, appName, componentName string) *corev1.Secret {
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
			defaults.JobPayloadPropertyName: []byte(payload),
		},
	}
	return &secret
}

func isPayloadDefinedForJobComponent(radixJobComponent *v1.RadixDeployJobComponent) bool {
	return radixJobComponent.Payload != nil && strings.TrimSpace(radixJobComponent.Payload.Path) != ""
}

func buildBatchScheduleDescriptionSecretSpec(batchName, appName, componentName string, batchScheduleDescription *models.BatchScheduleDescription) (*corev1.Secret, error) {
	secretName := defaults.GetBatchScheduleDescriptionSecretName(batchName)
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
				kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
				"radix-batch-name":       batchName,
				//TODO: add to radix-operator kube: RadixBatchNameLabel = "radix-batch-name"
			},
		},
		Data: map[string][]byte{
			defaults.BatchScheduleDescriptionPropertyName: descriptionJson,
		},
	}
	return &secret, nil
}
