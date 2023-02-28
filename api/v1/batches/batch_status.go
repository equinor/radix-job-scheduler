package batchesv1

import (
	apiv1 "github.com/equinor/radix-job-scheduler/api/v1"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
)

// GetBatchStatusFromRadixBatch Gets batch status from RadixBatch
func GetBatchStatusFromRadixBatch(radixBatch *modelsv2.RadixBatch) *modelsv1.BatchStatus {
	return &modelsv1.BatchStatus{
		JobStatus: modelsv1.JobStatus{
			Name:    radixBatch.Name,
			Created: radixBatch.CreationTime,
			Started: radixBatch.Started,
			Ended:   radixBatch.Ended,
			Status:  radixBatch.Status,
			Message: radixBatch.Message,
		},
		JobStatuses: apiv1.GetJobStatusFromRadixBatchJobsStatuses(*radixBatch),
		BatchType:   radixBatch.BatchType,
	}
}
