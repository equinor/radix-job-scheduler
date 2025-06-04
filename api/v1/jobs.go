package v1

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-job-scheduler/internal"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
)

// GetJobStatusFromRadixBatchJobsStatus Get Job status from RadixBatchJob
func GetJobStatusFromRadixBatchJobsStatus(radixBatch *modelsv2.RadixBatch, jobStatus modelsv2.RadixBatchJobStatus) modelsv1.JobStatus {
	return modelsv1.JobStatus{
		JobId:       jobStatus.JobId,
		BatchName:   getBatchName(radixBatch),
		Name:        jobStatus.Name,
		Created:     jobStatus.CreationTime,
		Started:     jobStatus.Started,
		Ended:       jobStatus.Ended,
		Status:      string(jobStatus.Status),
		Message:     jobStatus.Message,
		Failed:      jobStatus.Failed,
		Restart:     jobStatus.Restart,
		PodStatuses: GetPodStatus(jobStatus.PodStatuses),
	}
}

// GetJobStatusFromRadixBatchJobsStatuses Get JobStatuses from RadixBatch job statuses V2
func GetJobStatusFromRadixBatchJobsStatuses(radixBatches ...modelsv2.RadixBatch) []modelsv1.JobStatus {
	jobStatuses := make([]modelsv1.JobStatus, 0, len(radixBatches))
	for _, radixBatch := range radixBatches {
		for _, jobStatus := range radixBatch.JobStatuses {
			jobStatuses = append(jobStatuses, GetJobStatusFromRadixBatchJobsStatus(&radixBatch, jobStatus))
		}
	}
	return jobStatuses
}

// CopyJob Copy a job
func CopyJob(ctx context.Context, handlerApiV2 Handler, jobName, deploymentName string) (*modelsv2.RadixBatch, error) {
	return handlerApiV2.CopyRadixBatchJob(ctx, jobName, deploymentName)
}

// StopJob Stop a job
func StopJob(ctx context.Context, handlerApiV2 Handler, jobName string) error {
	if batchName, batchJobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobName); ok {
		return handlerApiV2.StopRadixBatchJob(ctx, batchName, batchJobName)
	}
	return fmt.Errorf("stop of this job is not supported")
}

// StopAllSingleJobs Stop alls jobs
func StopAllSingleJobs(ctx context.Context, handlerApiV2 Handler, componentName string) error {
	return handlerApiV2.StopAllSingleRadixJobs(ctx)
}

// GetBatchJob Get batch job
func GetBatchJob(ctx context.Context, handlerApiV2 Handler, batchName, jobName string) (*modelsv1.JobStatus, error) {
	radixBatch, err := handlerApiV2.GetRadixBatch(ctx, batchName)
	if err != nil {
		return nil, err
	}
	for _, jobStatus := range radixBatch.JobStatuses {
		if !strings.EqualFold(jobStatus.Name, jobName) {
			continue
		}
		jobsStatus := GetJobStatusFromRadixBatchJobsStatus(radixBatch, jobStatus)
		return &jobsStatus, nil
	}
	return nil, fmt.Errorf("not found")
}

// GetPodStatus Converts RadixBatchJobPodStatuses to PodStatuses
func GetPodStatus(podStatuses []modelsv2.RadixBatchJobPodStatus) []modelsv1.PodStatus {
	return slice.Map(podStatuses, func(status modelsv2.RadixBatchJobPodStatus) modelsv1.PodStatus {
		return modelsv1.PodStatus{
			Name:             status.Name,
			Created:          &status.Created,
			StartTime:        status.StartTime,
			EndTime:          status.EndTime,
			ContainerStarted: status.StartTime,
			Status:           modelsv1.ReplicaStatus{Status: status.Status.Status},
			StatusMessage:    status.StatusMessage,
			RestartCount:     status.RestartCount,
			Image:            status.Image,
			ImageId:          status.ImageId,
			PodIndex:         status.PodIndex,
			ExitCode:         status.ExitCode,
			Reason:           status.Reason,
		}
	})
}

func getBatchName(radixBatch *modelsv2.RadixBatch) string {
	return utils.TernaryString(radixBatch.BatchType == string(kube.RadixBatchTypeJob), "", radixBatch.Name)
}
