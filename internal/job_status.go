package internal

import (
	"slices"

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
		for _, r := range activeRadixDeployJobComponent.BatchStatusRules {
			if matchBatchStatusRule(r, radixBatchJobPhases) {
				return r.BatchStatus
			}
		}
	}
	if len(defaultBatchStatus) == 0 {
		return radixv1.RadixBatchJobApiStatusWaiting
	}
	return defaultBatchStatus
}

func matchBatchStatusRule(rule radixv1.BatchStatusRule, phases []radixv1.RadixBatchJobPhase) bool {
	operatorPredicate := func(phase radixv1.RadixBatchJobPhase) bool {
		if rule.Operator == radixv1.OperatorIn {
			return slices.Contains(rule.JobStatuses, phase)
		}
		return !slices.Contains(rule.JobStatuses, phase)
	}

	if rule.Condition == radixv1.ConditionAll {
		return slice.All(phases, operatorPredicate)
	}
	return slice.Any(phases, operatorPredicate)
}
