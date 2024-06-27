package jobs

import (
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// GetScheduledJobStatus Gets job status
func GetScheduledJobStatus(jobStatus radixv1.RadixBatchJobStatus, stopJob bool) (status radixv1.RadixBatchJobApiStatus) {
	status = radixv1.RadixBatchJobApiStatusWaiting
	switch jobStatus.Phase {
	case radixv1.BatchJobPhaseActive:
		status = radixv1.RadixBatchJobApiStatusActive
	case radixv1.BatchJobPhaseRunning:
		status = radixv1.RadixBatchJobApiStatusRunning
	case radixv1.BatchJobPhaseSucceeded:
		status = radixv1.RadixBatchJobApiStatusSucceeded
	case radixv1.BatchJobPhaseFailed:
		status = radixv1.RadixBatchJobApiStatusFailed
	case radixv1.BatchJobPhaseStopped:
		status = radixv1.RadixBatchJobApiStatusStopped
	case radixv1.BatchJobPhaseWaiting:
		status = radixv1.RadixBatchJobApiStatusWaiting
	}
	if stopJob && (status == radixv1.RadixBatchJobApiStatusWaiting || status == radixv1.RadixBatchJobApiStatusActive || status == radixv1.RadixBatchJobApiStatusRunning) {
		return radixv1.RadixBatchJobApiStatusStopping
	}
	if len(jobStatus.RadixBatchJobPodStatuses) > 0 && slice.All(jobStatus.RadixBatchJobPodStatuses, func(jobPodStatus radixv1.RadixBatchJobPodStatus) bool {
		return jobPodStatus.Phase == radixv1.PodFailed
	}) {
		return radixv1.RadixBatchJobApiStatusFailed
	}
	return status
}

// GetRadixBatchStatus Gets the batch status
func GetRadixBatchStatus(radixBatch *radixv1.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) (status radixv1.RadixBatchJobApiStatus) {
	isSingleJob := radixBatch.Labels[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeJob) && len(radixBatch.Spec.Jobs) == 1
	if isSingleJob {
		if len(radixBatch.Status.JobStatuses) == 0 {
			return radixv1.RadixBatchJobApiStatusWaiting
		}
		return radixv1.RadixBatchJobApiStatus(radixBatch.Status.JobStatuses[0].Phase)
	}
	return GetStatusFromStatusRules(getBatchJobStatusPhases(radixBatch), radixDeployJobComponent.BatchStatusRules, radixv1.RadixBatchJobApiStatus(radixBatch.Status.Condition.Type))
}

func getBatchJobStatusPhases(radixBatch *radixv1.RadixBatch) []radixv1.RadixBatchJobPhase {
	return slice.Reduce(radixBatch.Status.JobStatuses, make([]radixv1.RadixBatchJobPhase, 0),
		func(acc []radixv1.RadixBatchJobPhase, jobStatus radixv1.RadixBatchJobStatus) []radixv1.RadixBatchJobPhase { return append(acc, jobStatus.Phase) },
	)
}

// GetStatusFromStatusRules Gets BatchStatus by rules
func GetStatusFromStatusRules(radixBatchJobPhases []radixv1.RadixBatchJobPhase, rules []radixv1.BatchStatusRule, defaultBatchStatus radixv1.RadixBatchJobApiStatus) radixv1.RadixBatchJobApiStatus {
	for _, rule := range rules {
		evaluateJobStatusByRule := func(jobStatusPhase radixv1.RadixBatchJobPhase) bool { return evaluateCondition(jobStatusPhase, rule) }
		switch rule.Condition {
		case radixv1.ConditionAny:
			if slice.Any(radixBatchJobPhases, evaluateJobStatusByRule) {
				return rule.BatchStatus
			}
		case radixv1.ConditionAll:
			if slice.All(radixBatchJobPhases, evaluateJobStatusByRule) {
				return rule.BatchStatus
			}
		}
	}
	if len(defaultBatchStatus) == 0 {
		return radixv1.RadixBatchJobApiStatusWaiting
	}
	return defaultBatchStatus
}

func evaluateCondition(jobStatus radixv1.RadixBatchJobPhase, rule radixv1.BatchStatusRule) bool {
	for _, status := range rule.JobStatuses {
		if jobStatus == status && rule.Operator == radixv1.OperatorNotIn {
			return false
		}
		if jobStatus != status && rule.Operator == radixv1.OperatorIn {
			return false
		}
	}
	return true
}
