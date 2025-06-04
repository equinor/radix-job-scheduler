package internal

import (
	"fmt"
	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-job-scheduler/pkg/notifications"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

func GetBatchAndJobStatuses(radixBatch *radixv1.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent, radixBatchJobStatuses []radixv1.RadixBatchJobStatus) (v1.JobStatus, []v1.JobStatus) {
	var startedTime, endedTime *time.Time
	if radixBatch.Status.Condition.ActiveTime != nil {
		startedTime = &radixBatch.Status.Condition.ActiveTime.Time
	}
	if radixBatch.Status.Condition.CompletionTime != nil {
		endedTime = &radixBatch.Status.Condition.CompletionTime.Time
	}

	batchStatus := v1.JobStatus{
		Name:    radixBatch.GetName(),
		BatchId: notifications.getBatchId(radixBatch),
		Created: pointers.Ptr(radixBatch.GetCreationTimestamp().Time),
		Started: startedTime,
		Ended:   endedTime,
		Status:  string(GetRadixBatchStatus(radixBatch, radixDeployJobComponent)),
		Message: radixBatch.Status.Condition.Message,
		Updated: pointers.Ptr(time.Now()),
	}
	jobStatuses := getRadixBatchJobStatusesFromRadixBatch(radixBatch, radixBatchJobStatuses)
	return batchStatus, jobStatuses
}

func getRadixBatchJobStatusesFromRadixBatch(radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus) []v1.JobStatus {
	batchName := notifications.getBatchName(radixBatch)
	radixBatchJobsMap := notifications.getRadixBatchJobsMap(radixBatch.Spec.Jobs)
	jobStatuses := make([]v1.JobStatus, 0, len(radixBatchJobStatuses))
	for _, radixBatchJobStatus := range radixBatchJobStatuses {
		var started, ended, created *time.Time
		if radixBatchJobStatus.CreationTime != nil {
			created = &radixBatchJobStatus.CreationTime.Time
		}
		if radixBatchJobStatus.StartTime != nil {
			started = &radixBatchJobStatus.StartTime.Time
		}

		if radixBatchJobStatus.EndTime != nil {
			ended = &radixBatchJobStatus.EndTime.Time
		}

		radixBatchJob, ok := radixBatchJobsMap[radixBatchJobStatus.Name]
		if !ok {
			continue
		}
		stopJob := radixBatchJob.Stop != nil && *radixBatchJob.Stop
		jobName := fmt.Sprintf("%s-%s", radixBatch.Name, radixBatchJobStatus.Name) // composed name in models are always consist of a batchName and original jobName
		jobStatus := v1.JobStatus{
			BatchName:   batchName,
			Name:        jobName,
			JobId:       radixBatchJob.JobId,
			Created:     created,
			Started:     started,
			Ended:       ended,
			Status:      string(GetScheduledJobStatus(radixBatchJobStatus, stopJob)),
			Failed:      radixBatchJobStatus.Failed,
			Restart:     radixBatchJobStatus.Restart,
			Message:     radixBatchJobStatus.Message,
			Updated:     pointers.Ptr(time.Now()),
			PodStatuses: GetPodStatusByRadixBatchJobPodStatus(radixBatch, radixBatchJobStatus.RadixBatchJobPodStatuses),
		}
		jobStatuses = append(jobStatuses, jobStatus)
	}
	return jobStatuses
}

func GetBatchName(radixBatch *radixv1.RadixBatch) string {
	return utils.TernaryString(radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeJob), "", radixBatch.GetName())
}

func GetBatchId(radixBatch *radixv1.RadixBatch) string {
	return utils.TernaryString(radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeJob), "", radixBatch.Spec.BatchId)
}
