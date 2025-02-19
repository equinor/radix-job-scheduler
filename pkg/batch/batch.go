package batch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/internal"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-job-scheduler/utils/radix/jobs"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// CompletedRadixBatchNames Completed RadixBatch lists
type CompletedRadixBatchNames struct {
	SucceededRadixBatches    []string
	NotSucceededRadixBatches []string
	SucceededSingleJobs      []string
	NotSucceededSingleJobs   []string
}

// GetRadixBatchStatus Get radix batch
func GetRadixBatchStatus(radixBatch *radixv1.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) modelsv2.RadixBatch {
	var started, ended *time.Time
	if radixBatch.Status.Condition.ActiveTime != nil {
		started = &radixBatch.Status.Condition.ActiveTime.Time
	}

	if radixBatch.Status.Condition.CompletionTime != nil {
		ended = &radixBatch.Status.Condition.CompletionTime.Time
	}

	return modelsv2.RadixBatch{
		Name:           radixBatch.GetName(),
		BatchId:        radixBatch.Spec.BatchId,
		BatchType:      radixBatch.Labels[kube.RadixBatchTypeLabel],
		CreationTime:   radixBatch.GetCreationTimestamp().Time,
		Started:        started,
		Ended:          ended,
		Status:         jobs.GetRadixBatchStatus(radixBatch, radixDeployJobComponent),
		Message:        radixBatch.Status.Condition.Message,
		JobStatuses:    getRadixBatchJobStatusesFromRadixBatch(radixBatch, radixBatch.Status.JobStatuses),
		DeploymentName: radixBatch.Spec.RadixDeploymentJobRef.Name,
	}
}

// DeleteRadixBatchByName Delete a batch by name
func DeleteRadixBatchByName(ctx context.Context, radixClient versioned.Interface, namespace, batchName string) error {
	radixBatch, err := internal.GetRadixBatch(ctx, radixClient, namespace, batchName)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return DeleteRadixBatch(ctx, radixClient, radixBatch)
}

func getRadixBatchJobStatusesFromRadixBatch(radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus) []modelsv2.RadixBatchJobStatus {
	radixBatchJobsStatuses := getRadixBatchJobsStatusesMap(radixBatchJobStatuses)
	jobStatuses := make([]modelsv2.RadixBatchJobStatus, 0, len(radixBatch.Spec.Jobs))
	for _, radixBatchJob := range radixBatch.Spec.Jobs {
		stopJob := radixBatchJob.Stop != nil && *radixBatchJob.Stop
		jobName := fmt.Sprintf("%s-%s", radixBatch.Name, radixBatchJob.Name) // composed name in models are always consist of a batchName and original jobName
		radixBatchJobStatus := modelsv2.RadixBatchJobStatus{
			Name:  jobName,
			JobId: radixBatchJob.JobId,
		}

		if jobStatus, ok := radixBatchJobsStatuses[radixBatchJob.Name]; ok {
			if jobStatus.CreationTime != nil {
				radixBatchJobStatus.CreationTime = &jobStatus.CreationTime.Time
			}
			if jobStatus.StartTime != nil {
				radixBatchJobStatus.Started = &jobStatus.StartTime.Time
			}
			if jobStatus.EndTime != nil {
				radixBatchJobStatus.Ended = &jobStatus.EndTime.Time
			}
			radixBatchJobStatus.Status = jobs.GetScheduledJobStatus(jobStatus, stopJob)
			radixBatchJobStatus.Message = jobStatus.Message
			radixBatchJobStatus.Failed = jobStatus.Failed
			radixBatchJobStatus.Restart = jobStatus.Restart
			radixBatchJobStatus.PodStatuses = getPodStatusByRadixBatchJobPodStatus(jobStatus.RadixBatchJobPodStatuses, radixBatch.CreationTimestamp.Time)
		} else {
			radixBatchJobStatus.Status = jobs.GetScheduledJobStatus(radixv1.RadixBatchJobStatus{Phase: radixv1.RadixBatchJobApiStatusWaiting}, stopJob)
		}
		jobStatuses = append(jobStatuses, radixBatchJobStatus)
	}
	return jobStatuses
}

func getRadixBatchJobsStatusesMap(radixBatchJobStatuses []radixv1.RadixBatchJobStatus) map[string]radixv1.RadixBatchJobStatus {
	radixBatchJobsStatuses := make(map[string]radixv1.RadixBatchJobStatus)
	for _, jobStatus := range radixBatchJobStatuses {
		jobStatus := jobStatus
		radixBatchJobsStatuses[jobStatus.Name] = jobStatus
	}
	return radixBatchJobsStatuses
}

func getPodStatusByRadixBatchJobPodStatus(podStatuses []radixv1.RadixBatchJobPodStatus, created time.Time) []modelsv2.RadixBatchJobPodStatus {
	return slice.Map(podStatuses, func(status radixv1.RadixBatchJobPodStatus) modelsv2.RadixBatchJobPodStatus {
		if status.CreationTime != nil {
			created = status.CreationTime.Time
		}
		var started, ended *time.Time
		if status.StartTime != nil {
			started = &status.StartTime.Time
		}

		if status.EndTime != nil {
			ended = &status.EndTime.Time
		}

		return modelsv2.RadixBatchJobPodStatus{
			Name:             status.Name,
			Created:          created,
			StartTime:        started,
			EndTime:          ended,
			ContainerStarted: started,
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
func CopyRadixBatchOrJob(ctx context.Context, radixClient versioned.Interface, sourceRadixBatch *radixv1.RadixBatch, sourceJobName string, radixDeployJobComponent *radixv1.RadixDeployJobComponent, radixDeploymentName string) (*modelsv2.RadixBatch, error) {
	radixComponentName := radixDeployJobComponent.GetName()
	logger := log.Ctx(ctx)
	logger.Info().Msgf("copy a jobs %s of the batch %s", sourceJobName, sourceRadixBatch.GetName())
	radixBatch := radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name:   internal.GenerateBatchName(radixComponentName),
			Labels: sourceRadixBatch.GetLabels(),
		},
		Spec: radixv1.RadixBatchSpec{
			BatchId: sourceRadixBatch.Spec.BatchId,
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: radixDeploymentName},
				Job:                  radixComponentName,
			},
			Jobs: copyBatchJobs(ctx, sourceRadixBatch, sourceJobName),
		},
	}

	logger.Debug().Msgf("Create the copied Radix Batch %s with %d jobs in the cluster", radixBatch.GetName(), len(radixBatch.Spec.Jobs))
	createdRadixBatch, err := radixClient.RadixV1().RadixBatches(sourceRadixBatch.GetNamespace()).Create(ctx, &radixBatch, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	logger.Debug().Msgf("copied the batch %s for the component %s", radixBatch.GetName(), radixComponentName)
	return pointers.Ptr(GetRadixBatchStatus(createdRadixBatch, radixDeployJobComponent)), nil
}

// StopRadixBatch Stop a batch
func StopRadixBatch(ctx context.Context, radixClient versioned.Interface, appName, envName, jobComponentName, batchName string) error {
	return stopRadixBatchJob(ctx, radixClient, appName, envName, jobComponentName, kube.RadixBatchTypeBatch, batchName, "")
}

func stopRadixBatchJob(ctx context.Context, radixClient versioned.Interface, appName, envName, jobComponentName string, batchType kube.RadixBatchType, batchName, jobName string) error {
	namespace := operatorUtils.GetEnvironmentNamespace(appName, envName)
	logger := log.Ctx(ctx)
	logger.Info().Msgf("stop the batch %s for the application %s, environment %s", batchName, appName, envName)
	selector := radixLabels.ForComponentName(jobComponentName)
	if batchType != "" {
		selector[kube.RadixBatchTypeLabel] = string(batchType)
	}
	var foundBatch, foundJob, appliedChanges bool
	var externalError error
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		radixBatches, err := internal.GetRadixBatches(ctx, radixClient, namespace, selector)
		if err != nil {
			return err
		}
		for _, radixBatch := range radixBatches {
			if len(batchName) > 0 {
				if strings.EqualFold(radixBatch.GetName(), batchName) {
					foundBatch = true
				} else {
					continue
				}
			}
			if !isBatchStoppable(radixBatch.Status.Condition) {
				externalError = apiErrors.NewBadRequest(fmt.Sprintf("cannot stop completed batch %s", batchName))
				return nil
			}
			newRadixBatch := radixBatch.DeepCopy()
			for jobIndex, radixBatchJob := range newRadixBatch.Spec.Jobs {
				if len(jobName) > 0 {
					if strings.EqualFold(radixBatchJob.Name, jobName) {
						foundJob = true
					} else {
						continue
					}
				}
				if jobStatus, ok := slice.FindFirst(newRadixBatch.Status.JobStatuses, func(status radixv1.RadixBatchJobStatus) bool {
					return status.Name == radixBatchJob.Name
				}); ok &&
					(internal.IsRadixBatchJobSucceeded(jobStatus) || internal.IsRadixBatchJobFailed(jobStatus)) {
					if len(batchName) > 0 && radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeJob) {
						externalError = apiErrors.NewBadRequest(fmt.Sprintf("cannot stop the job %s with the status %s", radixBatch.GetName(), jobStatus.Phase))
						return nil
					}
					if len(jobName) > 0 {
						externalError = apiErrors.NewBadRequest(fmt.Sprintf("cannot stop the job %s with the status %s in the batch %s", jobName, jobStatus.Phase, batchName))
						return nil
					}
					continue
				}
				newRadixBatch.Spec.Jobs[jobIndex].Stop = pointers.Ptr(true)
				appliedChanges = true
				if foundJob {
					break
				}
			}
			if appliedChanges {
				_, err = radixClient.RadixV1().RadixBatches(namespace).Update(ctx, newRadixBatch, metav1.UpdateOptions{})
				return err
			}
		}
		if len(batchName) > 0 && !foundBatch {
			externalError = apiErrors.NewNotFound("batch", batchName)
			return nil
		}
		if len(jobName) > 0 && !foundJob {
			externalError = apiErrors.NewNotFound("job", jobName)
			return nil
		}
		return nil
	})
	if err != nil {
		return apiErrors.NewFromError(fmt.Errorf("failed to update the batch %s: %w", batchName, err))
	}
	if externalError != nil {
		return externalError
	}
	logger.Debug().Msgf("Patched RadixBatch: %s in namespace %s", batchName, namespace)
	return nil
}

// StopAllRadixBatches Stop all batches
func StopAllRadixBatches(ctx context.Context, radixClient versioned.Interface, appName, envName, jobComponentName string, batchType kube.RadixBatchType) error {
	return stopRadixBatchJob(ctx, radixClient, appName, envName, jobComponentName, batchType, "", "")
}

// StopRadixBatchJob Stop a job
func StopRadixBatchJob(ctx context.Context, radixClient versioned.Interface, appName, envName, jobComponentName, batchName, jobName string) error {
	return stopRadixBatchJob(ctx, radixClient, appName, envName, jobComponentName, "", batchName, jobName)
}

// RestartRadixBatch Restart a batch
func RestartRadixBatch(ctx context.Context, radixClient versioned.Interface, radixBatch *radixv1.RadixBatch) error {
	logger := log.Ctx(ctx)
	logger.Info().Msgf("restart the batch %s", radixBatch.GetName())
	restartTimestamp := utils.FormatTimestamp(time.Now())
	for jobIdx := 0; jobIdx < len(radixBatch.Spec.Jobs); jobIdx++ {
		setRestartJobTimeout(radixBatch, jobIdx, restartTimestamp)
	}
	if _, err := radixClient.RadixV1().RadixBatches(radixBatch.GetNamespace()).Update(ctx, radixBatch, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}

// RestartRadixBatchJob Restart a job
func RestartRadixBatchJob(ctx context.Context, radixClient versioned.Interface, radixBatch *radixv1.RadixBatch, jobName string) error {
	logger := log.Ctx(ctx)
	logger.Info().Msgf("restart a job %s in the batch %s", jobName, radixBatch.GetName())
	jobIdx := slice.FindIndex(radixBatch.Spec.Jobs, func(job radixv1.RadixBatchJob) bool { return job.Name == jobName })
	if jobIdx == -1 {
		return fmt.Errorf("job %s not found", jobName)
	}
	setRestartJobTimeout(radixBatch, jobIdx, utils.FormatTimestamp(time.Now()))
	if _, err := radixClient.RadixV1().RadixBatches(radixBatch.GetNamespace()).Update(ctx, radixBatch, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}

// DeleteRadixBatch Delete a batch
func DeleteRadixBatch(ctx context.Context, radixClient versioned.Interface, radixBatch *radixv1.RadixBatch) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("delete batch %s", radixBatch.GetName())
	if err := radixClient.RadixV1().RadixBatches(radixBatch.GetNamespace()).Delete(ctx, radixBatch.GetName(), metav1.DeleteOptions{PropagationPolicy: pointers.Ptr(metav1.DeletePropagationBackground)}); err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	return nil
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

func setRestartJobTimeout(batch *radixv1.RadixBatch, jobIdx int, restartTimestamp string) {
	batch.Spec.Jobs[jobIdx].Stop = nil
	batch.Spec.Jobs[jobIdx].Restart = restartTimestamp
}
