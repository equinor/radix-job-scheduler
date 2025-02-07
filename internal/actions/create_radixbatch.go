package actions

import (
	"fmt"

	"dario.cat/mergo"
	"github.com/equinor/radix-job-scheduler/internal/names"
	"github.com/equinor/radix-job-scheduler/models/common"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Max size of the secret description, including description, metadata, base64 encodes secret values, etc.
	maxPayloadSecretSize = 1024 * 512 // 0.5MB
	// Standard secret description, metadata, etc.
	payloadSecretHeaderSize = 600
	// Each entry in a secret Data has name, etc.
	payloadSecretDataHeaderSize = 128
)

func BuildRadixBatchResources(batchScheduleDescription common.BatchScheduleDescription, radixBatchType kube.RadixBatchType, rd radixv1.RadixDeployment, jobComponentName string) (*radixv1.RadixBatch, []corev1.Secret, error) {
	builder := radixBatchBuilder{
		batchName:        names.NewRadixBatchName(jobComponentName),
		batchType:        radixBatchType,
		rd:               rd,
		jobComponentName: jobComponentName,
	}
	return builder.build(batchScheduleDescription)
}

type jobIndexPayloadSecretRefMap map[int]*radixv1.PayloadSecretKeySelector

type radixBatchBuilder struct {
	batchName        string
	batchType        kube.RadixBatchType
	rd               radixv1.RadixDeployment
	jobComponentName string
}

func (b *radixBatchBuilder) build(batchSpec common.BatchScheduleDescription) (*radixv1.RadixBatch, []corev1.Secret, error) {

	jobs, secrets, err := b.buildBatchJobs(batchSpec.JobScheduleDescriptions, batchSpec.DefaultRadixJobComponentConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build jobs: %w", err)
	}

	rb := radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name: b.batchName,
			Labels: radixLabels.Merge(
				radixLabels.ForApplicationName(b.rd.Spec.AppName),
				radixLabels.ForComponentName(b.jobComponentName),
				radixLabels.ForBatchType(b.batchType),
			),
		},
		Spec: radixv1.RadixBatchSpec{
			BatchId: batchSpec.BatchId,
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: b.rd.Name},
				Job:                  b.jobComponentName,
			},
			Jobs: jobs,
		},
	}

	return &rb, secrets, nil
}

func (b *radixBatchBuilder) buildBatchJobs(jobSpecList []common.JobScheduleDescription, commonJobConfig *common.RadixJobComponentConfig) ([]radixv1.RadixBatchJob, []corev1.Secret, error) {
	jobs := make([]radixv1.RadixBatchJob, 0, len(jobSpecList))
	secrets, payloadSecretRefMap := b.buildPayloadSecrets(jobSpecList)

	for idx, jobSpec := range jobSpecList {
		job, err := b.buildBatchJob(jobSpec, commonJobConfig, payloadSecretRefMap[idx])
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build job: %w", err)
		}
		jobs = append(jobs, *job)
	}
	return jobs, secrets, nil
}

func (b *radixBatchBuilder) buildBatchJob(jobSpec common.JobScheduleDescription, commonJobConfig *common.RadixJobComponentConfig, payloadSecretRef *radixv1.PayloadSecretKeySelector) (*radixv1.RadixBatchJob, error) {
	if commonJobConfig != nil {
		if err := mergo.Merge(&jobSpec.RadixJobComponentConfig, commonJobConfig); err != nil {
			return nil, fmt.Errorf("failed to apply common config: %w", err)
		}
	}

	job := radixv1.RadixBatchJob{
		Name:             names.NewRadixBatchJobName(),
		JobId:            jobSpec.JobId,
		Resources:        jobSpec.Resources.MapToRadixResourceRequirements(),
		Node:             jobSpec.Node.MapToRadixNode(),
		TimeLimitSeconds: jobSpec.TimeLimitSeconds,
		BackoffLimit:     jobSpec.BackoffLimit,
		ImageTagName:     jobSpec.ImageTagName,
		FailurePolicy:    jobSpec.FailurePolicy.MapToRadixFailurePolicy(),
		PayloadSecretRef: payloadSecretRef,
	}

	return &job, nil
}

func (b *radixBatchBuilder) buildPayloadSecrets(jobSpecList []common.JobScheduleDescription) ([]corev1.Secret, jobIndexPayloadSecretRefMap) {
	payloadRef := make(jobIndexPayloadSecretRefMap, len(jobSpecList))
	var secrets []corev1.Secret

	for idx, jobSpec := range jobSpecList {
		payloadRef[idx] = &radixv1.PayloadSecretKeySelector{
			LocalObjectReference: radixv1.LocalObjectReference{
				Name: "",
			},
			Key: "",
		}
	}
	return secrets, payloadRef
}

func (b *radixBatchBuilder) newPayloadSecret(suffix string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-payloads-%s", b.batchName, suffix),
			Labels: radixLabels.Merge(
				radixLabels.ForApplicationName(b.rd.Spec.AppName),
				radixLabels.ForComponentName(b.jobComponentName),
				radixLabels.ForBatchName(b.batchName),
				radixLabels.ForJobScheduleJobType(),
				radixLabels.ForRadixSecretType(kube.RadixSecretJobPayload),
			),
		},
		StringData: make(map[string]string),
	}
}
