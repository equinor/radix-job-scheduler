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
	return status
}

// GetRadixBatchJobApiStatus Gets the batch status
func GetRadixBatchJobApiStatus(radixBatch *radixv1.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) (status radixv1.RadixBatchJobApiStatus) {
	isSingleJob := radixBatch.Labels[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeJob) && len(radixBatch.Spec.Jobs) == 1
	if isSingleJob {
		if len(radixBatch.Status.JobStatuses) == 0 {
			return radixv1.RadixBatchJobApiStatusWaiting
		}
		return radixv1.RadixBatchJobApiStatus(radixBatch.Status.JobStatuses[0].Phase)
	}
	return GetStatusFromStatusRules(getBatchJobStatusPhases(radixBatch), radixDeployJobComponent, radixv1.RadixBatchJobApiStatus(radixBatch.Status.Condition.Type))
}

func getBatchJobStatusPhases(radixBatch *radixv1.RadixBatch) []radixv1.RadixBatchJobPhase {
	return slice.Reduce(radixBatch.Status.JobStatuses, make([]radixv1.RadixBatchJobPhase, 0),
		func(acc []radixv1.RadixBatchJobPhase, jobStatus radixv1.RadixBatchJobStatus) []radixv1.RadixBatchJobPhase {
			return append(acc, jobStatus.Phase)
		},
	)
}

// GetStatusFromStatusRules Gets BatchStatus by rules
func GetStatusFromStatusRules(radixBatchJobPhases []radixv1.RadixBatchJobPhase, activeRadixDeployJobComponent *radixv1.RadixDeployJobComponent, defaultBatchStatus radixv1.RadixBatchJobApiStatus) radixv1.RadixBatchJobApiStatus {
	if activeRadixDeployJobComponent != nil {
		for _, rule := range activeRadixDeployJobComponent.BatchStatusRules {
			ruleJobStatusesMap := slice.Reduce(rule.JobStatuses, make(map[radixv1.RadixBatchJobPhase]struct{}), func(acc map[radixv1.RadixBatchJobPhase]struct{}, jobStatus radixv1.RadixBatchJobPhase) map[radixv1.RadixBatchJobPhase]struct{} {
				acc[jobStatus] = struct{}{}
				return acc
			})
			evaluateJobStatusByRule := func(jobStatusPhase radixv1.RadixBatchJobPhase) bool {
				return evaluateCondition(jobStatusPhase, rule.Operator, ruleJobStatusesMap)
			}
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
	}
	if len(defaultBatchStatus) == 0 {
		return radixv1.RadixBatchJobApiStatusWaiting
	}
	return defaultBatchStatus
}

func evaluateCondition(jobStatus radixv1.RadixBatchJobPhase, ruleOperator radixv1.Operator, ruleJobStatusesMap map[radixv1.RadixBatchJobPhase]struct{}) bool {
	_, statusExistInRule := ruleJobStatusesMap[jobStatus]
	return (ruleOperator == radixv1.OperatorNotIn && !statusExistInRule) || (ruleOperator == radixv1.OperatorIn && statusExistInRule)
}
