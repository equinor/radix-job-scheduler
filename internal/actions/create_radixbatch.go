package actions

import (
	"errors"
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

var (
	ErrPayloadPathNotConfiguredForJob = errors.New("payload path is not configured for job")
)

func BuildRadixBatchResources(batchScheduleDescription common.BatchScheduleDescription, radixBatchType kube.RadixBatchType, appName, radixDeploymentName string, jobComponent radixv1.RadixDeployJobComponent) (*radixv1.RadixBatch, []corev1.Secret, error) {
	batchName := names.NewRadixBatchName(jobComponent.Name)
	builder := radixBatchBuilder{
		batchName:           batchName,
		batchType:           radixBatchType,
		appName:             appName,
		radixDeploymentName: radixDeploymentName,
		jobComponent:        jobComponent,
		payloadSecrets: payloadSecrets{
			secretNamePrefix: batchName,
			secretLabels: radixLabels.Merge(
				radixLabels.ForApplicationName(appName),
				radixLabels.ForComponentName(jobComponent.Name),
				radixLabels.ForBatchName(batchName),
				radixLabels.ForJobScheduleJobType(),
				radixLabels.ForRadixSecretType(kube.RadixSecretJobPayload),
			),
		},
	}
	return builder.build(batchScheduleDescription)
}

type radixBatchBuilder struct {
	appName             string
	radixDeploymentName string
	batchName           string
	batchType           kube.RadixBatchType
	jobComponent        radixv1.RadixDeployJobComponent
	payloadSecrets      payloadSecrets
}

func (b *radixBatchBuilder) build(batchSpec common.BatchScheduleDescription) (*radixv1.RadixBatch, []corev1.Secret, error) {
	jobs, err := b.buildBatchJobs(batchSpec.JobScheduleDescriptions, batchSpec.DefaultRadixJobComponentConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build jobs: %w", err)
	}

	rb := radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name: b.batchName,
			Labels: radixLabels.Merge(
				radixLabels.ForApplicationName(b.appName),
				radixLabels.ForComponentName(b.jobComponent.Name),
				radixLabels.ForBatchType(b.batchType),
			),
		},
		Spec: radixv1.RadixBatchSpec{
			BatchId: batchSpec.BatchId,
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: b.radixDeploymentName},
				Job:                  b.jobComponent.Name,
			},
			Jobs: jobs,
		},
	}

	return &rb, b.payloadSecrets.Secrets(), nil
}

func (b *radixBatchBuilder) buildBatchJobs(jobSpecList []common.JobScheduleDescription, commonJobConfig *common.RadixJobComponentConfig) ([]radixv1.RadixBatchJob, error) {
	jobs := make([]radixv1.RadixBatchJob, 0, len(jobSpecList))

	for idx, jobSpec := range jobSpecList {
		job, err := b.buildBatchJob(jobSpec, commonJobConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build job #%d: %w", idx, err)
		}
		jobs = append(jobs, *job)
	}
	return jobs, nil
}

func (b *radixBatchBuilder) buildBatchJob(jobSpec common.JobScheduleDescription, commonJobConfig *common.RadixJobComponentConfig) (*radixv1.RadixBatchJob, error) {
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
	}

	if len(jobSpec.Payload) > 0 {
		if !b.isPayloadPathConfigured() {
			return nil, ErrPayloadPathNotConfiguredForJob
		}
		payloadSecretName, payloadSecretKey, err := b.payloadSecrets.AddPayload(jobSpec.Payload)
		if err != nil {
			return nil, fmt.Errorf("faIled to store payload: %w", err)
		}
		job.PayloadSecretRef = &radixv1.PayloadSecretKeySelector{
			LocalObjectReference: radixv1.LocalObjectReference{Name: payloadSecretName},
			Key:                  payloadSecretKey,
		}
	}

	return &job, nil
}

func (b *radixBatchBuilder) isPayloadPathConfigured() bool {
	return b.jobComponent.Payload != nil && len(b.jobComponent.Payload.Path) > 0
}
