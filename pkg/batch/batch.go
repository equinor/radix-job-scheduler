package batch

import (
	"fmt"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-job-scheduler/internal"
	"github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-job-scheduler/utils/radix/jobs"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// GetRadixBatch Get radix batch
func GetRadixBatch(radixBatch *v1.RadixBatch, radixDeployJobComponent *v1.RadixDeployJobComponent) v2.RadixBatch {
	return v2.RadixBatch{
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

func getRadixBatchJobStatusesFromRadixBatch(radixBatch *v1.RadixBatch, radixBatchJobStatuses []v1.RadixBatchJobStatus) []v2.RadixBatchJobStatus {
	radixBatchJobsStatuses := internal.GetRadixBatchJobsStatusesMap(radixBatchJobStatuses)
	jobStatuses := make([]v2.RadixBatchJobStatus, 0, len(radixBatch.Spec.Jobs))
	for _, radixBatchJob := range radixBatch.Spec.Jobs {
		stopJob := radixBatchJob.Stop != nil && *radixBatchJob.Stop
		jobName := fmt.Sprintf("%s-%s", radixBatch.Name, radixBatchJob.Name) // composed name in models are always consist of a batchName and original jobName
		radixBatchJobStatus := v2.RadixBatchJobStatus{
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
			radixBatchJobStatus.Status = jobs.GetScheduledJobStatus(v1.RadixBatchJobStatus{Phase: v1.RadixBatchJobApiStatusWaiting}, stopJob)
		}
		jobStatuses = append(jobStatuses, radixBatchJobStatus)
	}
	return jobStatuses
}

func getPodStatusByRadixBatchJobPodStatus(podStatuses []v1.RadixBatchJobPodStatus) []v2.RadixBatchJobPodStatus {
	return slice.Map(podStatuses, func(status v1.RadixBatchJobPodStatus) v2.RadixBatchJobPodStatus {
		return v2.RadixBatchJobPodStatus{
			Name:             status.Name,
			Created:          utils.FormatTime(status.CreationTime),
			StartTime:        utils.FormatTime(status.StartTime),
			EndTime:          utils.FormatTime(status.EndTime),
			ContainerStarted: utils.FormatTime(status.StartTime),
			Status:           v2.ReplicaStatus{Status: string(status.Phase)},
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
