package batch

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/internal"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-job-scheduler/utils/radix/jobs"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// GetRadixBatchStatus Get radix batch
func GetRadixBatchStatus(radixBatch *radixv1.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) modelsv2.RadixBatch {
	return modelsv2.RadixBatch{
		Name:           radixBatch.GetName(),
		BatchType:      radixBatch.Labels[kube.RadixBatchTypeLabel],
		CreationTime:   utils.FormatTime(pointers.Ptr(radixBatch.GetCreationTimestamp())),
		Started:        utils.FormatTime(radixBatch.Status.Condition.ActiveTime),
		Ended:          utils.FormatTime(radixBatch.Status.Condition.CompletionTime),
		Status:         jobs.GetRadixBatchStatus(radixBatch, radixDeployJobComponent),
		Message:        radixBatch.Status.Condition.Message,
		JobStatuses:    getRadixBatchJobStatusesFromRadixBatch(radixBatch, radixBatch.Status.JobStatuses),
		DeploymentName: radixBatch.Spec.RadixDeploymentJobRef.Name,
	}
}

func getRadixBatchJobStatusesFromRadixBatch(radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus) []modelsv2.RadixBatchJobStatus {
	radixBatchJobsStatuses := internal.GetRadixBatchJobsStatusesMap(radixBatchJobStatuses)
	jobStatuses := make([]modelsv2.RadixBatchJobStatus, 0, len(radixBatch.Spec.Jobs))
	for _, radixBatchJob := range radixBatch.Spec.Jobs {
		stopJob := radixBatchJob.Stop != nil && *radixBatchJob.Stop
		jobName := fmt.Sprintf("%s-%s", radixBatch.Name, radixBatchJob.Name) // composed name in models are always consist of a batchName and original jobName
		radixBatchJobStatus := modelsv2.RadixBatchJobStatus{
			Name:  jobName,
			JobId: radixBatchJob.JobId,
		}
		if jobStatus, ok := radixBatchJobsStatuses[radixBatchJob.Name]; ok {
			radixBatchJobStatus.CreationTime = utils.FormatTime(jobStatus.CreationTime)
			radixBatchJobStatus.Started = utils.FormatTime(jobStatus.StartTime)
			radixBatchJobStatus.Ended = utils.FormatTime(jobStatus.EndTime)
			radixBatchJobStatus.Status = jobs.GetScheduledJobStatus(jobStatus, stopJob)
			radixBatchJobStatus.Message = jobStatus.Message
			radixBatchJobStatus.Failed = jobStatus.Failed
			radixBatchJobStatus.Restart = jobStatus.Restart
			radixBatchJobStatus.PodStatuses = getPodStatusByRadixBatchJobPodStatus(jobStatus.RadixBatchJobPodStatuses)
		} else {
			radixBatchJobStatus.Status = jobs.GetScheduledJobStatus(radixv1.RadixBatchJobStatus{Phase: radixv1.RadixBatchJobApiStatusWaiting}, stopJob)
		}
		jobStatuses = append(jobStatuses, radixBatchJobStatus)
	}
	return jobStatuses
}

func getPodStatusByRadixBatchJobPodStatus(podStatuses []radixv1.RadixBatchJobPodStatus) []modelsv2.RadixBatchJobPodStatus {
	return slice.Map(podStatuses, func(status radixv1.RadixBatchJobPodStatus) modelsv2.RadixBatchJobPodStatus {
		return modelsv2.RadixBatchJobPodStatus{
			Name:             status.Name,
			Created:          utils.FormatTime(status.CreationTime),
			StartTime:        utils.FormatTime(status.StartTime),
			EndTime:          utils.FormatTime(status.EndTime),
			ContainerStarted: utils.FormatTime(status.StartTime),
			Status:           modelsv2.ReplicaStatus{Status: string(status.Phase)},
			StatusMessage:    status.Message,
			RestartCount:     status.RestartCount,
			Image:            status.Image,
			ImageId:          status.ImageID,
			PodIndex:         status.PodIndex,
			ExitCode:         status.ExitCode,
			Reason:           status.Reason,
		}
	})
}

// GetRadixBatchStatuses Convert to radix batch statuses
func GetRadixBatchStatuses(radixBatches []*radixv1.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) []modelsv2.RadixBatch {
	return slice.Reduce(radixBatches, make([]modelsv2.RadixBatch, 0, len(radixBatches)), func(acc []modelsv2.RadixBatch, radixBatch *radixv1.RadixBatch) []modelsv2.RadixBatch {
		return append(acc, GetRadixBatchStatus(radixBatch, radixDeployJobComponent))
	})
}

// CopyRadixBatchOrJob Copy the Radix batch or job
func CopyRadixBatchOrJob(ctx context.Context, radixClient versioned.Interface, namespace string, sourceBatchName string, sourceJobName string, radixDeployJobComponent *radixv1.RadixDeployJobComponent, radixDeploymentName string) (*modelsv2.RadixBatch, error) {
	if len(sourceBatchName) == 0 {
		if batchName, jobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(sourceJobName); ok {
			sourceBatchName = batchName
			sourceJobName = jobName
		} else {
			return nil, fmt.Errorf("copy of this job is not supported")
		}
	}
	sourceRadixBatch, err := internal.GetRadixBatch(ctx, radixClient, namespace, sourceBatchName)
	if err != nil {
		return nil, err
	}
	radixComponentName := radixDeployJobComponent.GetName()
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("copy batch %s for namespace: %s", sourceBatchName, namespace)
	radixBatch := radixv1.RadixBatch{
		ObjectMeta: v1.ObjectMeta{
			Name:   internal.GenerateBatchName(radixComponentName),
			Labels: sourceRadixBatch.GetLabels(),
		},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: radixDeploymentName},
				Job:                  radixComponentName,
			},
			Jobs: copyBatchJobs(ctx, sourceRadixBatch, sourceJobName),
		},
	}

	logger.Debug().Msgf("Create the copied Radix Batch %s with %d jobs in the cluster", radixBatch.GetName(), len(radixBatch.Spec.Jobs))
	createdRadixBatch, err := radixClient.RadixV1().RadixBatches(namespace).Create(ctx, &radixBatch, v1.CreateOptions{})
	if err != nil {
		return nil, errors.NewFromError(err)
	}

	logger.Debug().Msgf("copied batch %s from the batch %s for component %s, ein namespace: %s", radixBatch.GetName(), sourceBatchName, radixComponentName, namespace)
	return pointers.Ptr(GetRadixBatchStatus(createdRadixBatch, radixDeployJobComponent)), nil
}

// StopRadixBatch Stop a batch
func StopRadixBatch(ctx context.Context, radixClient versioned.Interface, radixBatch *radixv1.RadixBatch) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("stop batch %s for namespace: %s", radixBatch.GetName(), radixBatch.GetNamespace())
	if !isBatchStoppable(radixBatch.Status.Condition) {
		return errors.NewBadRequest(fmt.Sprintf("cannot stop completed batch %s", radixBatch.GetName()))
	}

	newRadixBatch := radixBatch.DeepCopy()
	radixBatchJobsStatusMap := internal.GetRadixBatchJobsStatusesMap(newRadixBatch.Status.JobStatuses)
	for jobIndex, radixBatchJob := range newRadixBatch.Spec.Jobs {
		if jobStatus, ok := radixBatchJobsStatusMap[radixBatchJob.Name]; ok &&
			(internal.IsRadixBatchJobSucceeded(jobStatus) || internal.IsRadixBatchJobFailed(jobStatus)) {
			continue
		}
		newRadixBatch.Spec.Jobs[jobIndex].Stop = pointers.Ptr(true)
	}
	return updateRadixBatch(ctx, radixClient, radixBatch.GetNamespace(), newRadixBatch)
}

// StopRadixBatchJob Stop a job
func StopRadixBatchJob(ctx context.Context, radixClient versioned.Interface, radixBatch *radixv1.RadixBatch, jobName string) error {
	logger := log.Ctx(ctx)

	logger.Debug().Msgf("stop a job %s in the batch %s for namespace: %s", jobName, radixBatch.GetName(), radixBatch.GetNamespace())
	if !isBatchStoppable(radixBatch.Status.Condition) {
		return errors.NewBadRequest(fmt.Sprintf("cannot stop the job %s in the completed batch %s", jobName, radixBatch.GetName()))
	}

	newRadixBatch := radixBatch.DeepCopy()
	radixBatchJobsStatusMap := internal.GetRadixBatchJobsStatusesMap(newRadixBatch.Status.JobStatuses)
	for jobIndex, radixBatchJob := range newRadixBatch.Spec.Jobs {
		if !strings.EqualFold(radixBatchJob.Name, jobName) {
			continue
		}
		if jobStatus, ok := radixBatchJobsStatusMap[radixBatchJob.Name]; ok &&
			(internal.IsRadixBatchJobSucceeded(jobStatus) || internal.IsRadixBatchJobFailed(jobStatus)) {
			return errors.NewBadRequest(fmt.Sprintf("cannot stop the job %s with the status %s in the batch %s", jobName, string(jobStatus.Phase), radixBatch.GetName()))
		}
		newRadixBatch.Spec.Jobs[jobIndex].Stop = pointers.Ptr(true)
		return updateRadixBatch(ctx, radixClient, radixBatch.GetNamespace(), newRadixBatch)
	}
	return errors.NewNotFound("batch job", jobName)
}

func copyBatchJobs(ctx context.Context, sourceRadixBatch *radixv1.RadixBatch, sourceJobName string) []radixv1.RadixBatchJob {
	logger := log.Ctx(ctx)
	radixBatchJobs := make([]radixv1.RadixBatchJob, 0, len(sourceRadixBatch.Spec.Jobs))
	for _, sourceJob := range sourceRadixBatch.Spec.Jobs {
		if sourceJobName != "" && sourceJob.Name != sourceJobName {
			continue
		}
		job := sourceJob.DeepCopy()
		job.Name = internal.CreateJobName()
		logger.Debug().Msgf("Copy Radxi Batch Job %s", job.Name)
		radixBatchJobs = append(radixBatchJobs, *job)
	}
	return radixBatchJobs
}

// check if batch can be stopped
func isBatchStoppable(condition radixv1.RadixBatchCondition) bool {
	return condition.Type == "" ||
		condition.Type == radixv1.BatchConditionTypeActive ||
		condition.Type == radixv1.BatchConditionTypeWaiting
}

func updateRadixBatch(ctx context.Context, radixClient versioned.Interface, namespace string, radixBatch *radixv1.RadixBatch) error {
	logger := log.Ctx(ctx)
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := radixClient.RadixV1().RadixBatches(namespace).Update(ctx, radixBatch, v1.UpdateOptions{})
		return err
	})
	if err != nil {
		return errors.NewFromError(fmt.Errorf("failed to patch RadixBatch object: %w", err))
	}
	logger.Debug().Msgf("Patched RadixBatch: %s in namespace %s", radixBatch.GetName(), namespace)
	return nil
}
