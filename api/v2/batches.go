package apiv2

import (
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-job-scheduler/models/common"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// GetRadixBatchStatus Gets the batch status
func GetRadixBatchStatus(radixBatch *radixv1.RadixBatch) (status common.ProgressStatus) {
	status = common.Waiting
	switch {
	case radixBatch.Status.Condition.Type == radixv1.BatchConditionTypeActive:
		status = common.Running
	case radixBatch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted:
		status = common.Succeeded
		if slice.Any(radixBatch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
			return jobStatus.Phase == radixv1.BatchJobPhaseFailed
		}) {
			status = common.Failed
		}
	}
	return
}

// GetRadixBatchJobStatusFromPhase Get batch job status by job phase
func GetRadixBatchJobStatusFromPhase(job radixv1.RadixBatchJob, phase radixv1.RadixBatchJobPhase) (status common.ProgressStatus) {
	status = common.Waiting

	switch phase {
	case radixv1.BatchJobPhaseActive:
		status = common.Running
	case radixv1.BatchJobPhaseSucceeded:
		status = common.Succeeded
	case radixv1.BatchJobPhaseFailed:
		status = common.Failed
	case radixv1.BatchJobPhaseStopped:
		status = common.Stopped
	}

	var stop bool
	if job.Stop != nil {
		stop = *job.Stop
	}

	if stop && (status == common.Waiting || status == common.Running) {
		status = common.Stopping
	}

	return
}
