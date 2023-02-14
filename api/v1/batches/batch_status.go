package batchesv1

import (
	apiv1 "github.com/equinor/radix-job-scheduler/api/v1"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
)

// GetBatchStatusFromRadixBatch Gets batch status from RadixBatch
func GetBatchStatusFromRadixBatch(radixBatch *modelsv2.RadixBatch) *modelsv1.BatchStatus {
	jobStatus := modelsv1.BatchStatus{
		JobStatus: modelsv1.JobStatus{
			Name:    radixBatch.Name,
			Created: radixBatch.CreationTime,
			Started: radixBatch.Started,
			Ended:   radixBatch.Ended,
			Status:  radixBatch.Status,
			Message: radixBatch.Message,
		},
	}
	var jobStatuses []modelsv1.JobStatus
	for _, jobStatus := range radixBatch.JobStatuses {
		jobName := apiv1.ComposeSingleJobName(radixBatch.Name, jobStatus.Name)
		jobStatuses = append(jobStatuses, apiv1.GetJobStatusFromRadixBatchJobsStatus(radixBatch.Name, jobName, jobStatus))
	}
	jobStatus.JobStatuses = jobStatuses
	return &jobStatus
}
