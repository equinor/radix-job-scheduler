package v1

import (
	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
)

// GetJobStatusFromRadixBatchJobsStatus Get Job status from RadixBatchJob
func GetJobStatusFromRadixBatchJobsStatus(radixBatch *modelsv2.Batch, jobStatus modelsv2.Job) modelsv1.JobStatus {
	return modelsv1.JobStatus{
		JobId:       jobStatus.JobId,
		BatchName:   getBatchName(radixBatch),
		Name:        jobStatus.Name,
		Created:     jobStatus.CreationTime,
		Started:     jobStatus.Started,
		Ended:       jobStatus.Ended,
		Status:      modelsv1.JobStatusEnum(jobStatus.Status),
		Message:     jobStatus.Message,
		Failed:      jobStatus.Failed,
		Restart:     jobStatus.Restart,
		PodStatuses: GetPodStatus(jobStatus.PodStatuses),
	}
}

// GetJobStatusFromRadixBatchJobsStatuses Get JobStatuses from RadixBatch job statuses V2
func GetJobStatusFromRadixBatchJobsStatuses(radixBatches ...modelsv2.Batch) []modelsv1.JobStatus {
	jobStatuses := make([]modelsv1.JobStatus, 0, len(radixBatches))
	for _, radixBatch := range radixBatches {
		for _, jobStatus := range radixBatch.JobStatuses {
			jobStatuses = append(jobStatuses, GetJobStatusFromRadixBatchJobsStatus(&radixBatch, jobStatus))
		}
	}
	return jobStatuses
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

func getBatchName(radixBatch *modelsv2.Batch) string {
	return utils.TernaryString(radixBatch.BatchType == string(kube.RadixBatchTypeJob), "", radixBatch.Name)
}
