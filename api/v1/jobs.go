package v1

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"github.com/equinor/radix-job-scheduler/internal"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
)

// GetJobStatusFromRadixBatchJobsStatus Get Job status from RadixBatchJob
func GetJobStatusFromRadixBatchJobsStatus(batchName string, jobStatus modelsv2.RadixBatchJobStatus) modelsv1.JobStatus {
	return modelsv1.JobStatus{
		JobId:       jobStatus.JobId,
		BatchName:   batchName,
		Name:        jobStatus.Name,
		Created:     jobStatus.CreationTime,
		Started:     jobStatus.Started,
		Ended:       jobStatus.Ended,
		Status:      jobStatus.Status,
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
		jobStatusBatchName := getJobStatusBatchName(&radixBatch)
		for _, jobStatus := range radixBatch.JobStatuses {
			jobStatuses = append(jobStatuses, GetJobStatusFromRadixBatchJobsStatus(jobStatusBatchName, jobStatus))
		}
	}
	return jobStatuses
}

// CopyJob Copy a job
func CopyJob(ctx context.Context, handlerApiV2 apiv2.Handler, jobName, deploymentName string) (*modelsv2.RadixBatch, error) {
	return handlerApiV2.CopyRadixBatchSingleJob(ctx, jobName, deploymentName)
}

// StopJob Stop a job
func StopJob(ctx context.Context, handlerApiV2 apiv2.Handler, jobName string) error {
	if batchName, jobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobName); ok {
		return handlerApiV2.StopRadixBatchJob(ctx, batchName, jobName)
	}
	return fmt.Errorf("stop of this job is not supported")
}

// GetBatchJob Get batch job
func GetBatchJob(ctx context.Context, handlerApiV2 apiv2.Handler, batchName, jobName string) (*modelsv1.JobStatus, error) {
	radixBatch, err := handlerApiV2.GetRadixBatch(ctx, batchName)
	if err != nil {
		return nil, err
	}
	jobStatusBatchName := getJobStatusBatchName(radixBatch)
	for _, jobStatus := range radixBatch.JobStatuses {
		if !strings.EqualFold(jobStatus.Name, jobName) {
			continue
		}
		jobsStatus := GetJobStatusFromRadixBatchJobsStatus(jobStatusBatchName, jobStatus)
		return &jobsStatus, nil
	}
	return nil, fmt.Errorf("not found")
}

// GetPodStatus Converts RadixBatchJobPodStatuses to PodStatuses
func GetPodStatus(podStatuses []modelsv2.RadixBatchJobPodStatus) []modelsv1.PodStatus {
	return slice.Map(podStatuses, func(status modelsv2.RadixBatchJobPodStatus) modelsv1.PodStatus {
		return modelsv1.PodStatus{
			Name:             status.Name,
			Created:          status.Created,
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

func getJobStatusBatchName(radixBatch *modelsv2.RadixBatch) string {
	return utils.TernaryString(radixBatch.BatchType == string(kube.RadixBatchTypeJob), "", radixBatch.Name)
}
