package jobs

import (
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-job-scheduler/models/common"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// GetScheduledJobStatus Gets job status
func GetScheduledJobStatus(jobStatus radixv1.RadixBatchJobStatus, stopJob bool) (status common.ProgressStatus) {
	status = common.Waiting
	switch jobStatus.Phase {
	case radixv1.BatchJobPhaseActive:
		status = common.Active
	case radixv1.BatchJobPhaseRunning:
		status = common.Running
	case radixv1.BatchJobPhaseSucceeded:
		status = common.Succeeded
	case radixv1.BatchJobPhaseFailed:
		status = common.Failed
	case radixv1.BatchJobPhaseStopped:
		status = common.Stopped
	case radixv1.BatchJobPhaseWaiting:
		status = common.Waiting
	}
	if stopJob && (status == common.Waiting || status == common.Active || status == common.Running) {
		return common.Stopping
	}
	if len(jobStatus.RadixBatchJobPodStatuses) > 0 && slice.All(jobStatus.RadixBatchJobPodStatuses, func(jobPodStatus radixv1.RadixBatchJobPodStatus) bool {
		return jobPodStatus.Phase == radixv1.PodFailed
	}) {
		return common.Failed
	}
	return status
}

// GetRadixBatchStatus Gets the batch status
func GetRadixBatchStatus(radixBatch *radixv1.RadixBatch) (status common.ProgressStatus) {
	switch {
	case radixBatch.Status.Condition.Type == radixv1.BatchConditionTypeActive:
		if slice.Any(radixBatch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
			return jobStatus.Phase == radixv1.BatchJobPhaseRunning
		}) {
			return common.Running
		}
		return common.Active
	case radixBatch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted:
		if len(radixBatch.Status.JobStatuses) > 0 && slice.All(radixBatch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
			return jobStatus.Phase == radixv1.BatchJobPhaseFailed
		}) {
			return common.Failed
		}
		return common.Succeeded
	}
	return common.Waiting
}

// GetReplicaStatusByJobPodStatusPhase Get replica status by RadixBatchJobPodPhase
func GetReplicaStatusByJobPodStatusPhase(phase radixv1.RadixBatchJobPodPhase) string {
	switch phase {
	case radixv1.PodFailed:
		return "Failed"
	case radixv1.PodSucceeded:
		return "Succeeded"
	case radixv1.PodRunning:
		return "Running"
	}
	return "Pending"
}
