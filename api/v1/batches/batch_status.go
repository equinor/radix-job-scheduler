package batchesv1

import (
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
)

// GetBatchStatusFromRadixBatch Gets batch status from RadixBatch
func GetBatchStatusFromRadixBatch(radixBatch *modelsv2.RadixBatch) *modelsv1.BatchStatus {
	jobStatus := modelsv1.BatchStatus{
		JobStatus: modelsv1.JobStatus{
			BatchName: radixBatch.Name,
			Created:   radixBatch.CreationTime,
			Started:   radixBatch.Started,
			Ended:     radixBatch.Ended,
			Status:    radixBatch.Status,
			Message:   radixBatch.Message,
		},
	}
	return &jobStatus
}

// GetBatchStatusFromRadixBatchStatus Gets batch status from RadixBatchJobStatus
func GetBatchStatusFromRadixBatchStatus(batchName string, radixBatch *modelsv2.RadixBatch) *modelsv1.BatchStatus {
	jobStatus := modelsv1.BatchStatus{
		JobStatus: modelsv1.JobStatus{
			BatchName: batchName,
			Created:   radixBatch.CreationTime,
			Status:    radixBatch.Status,
			Message:   radixBatch.Message,
			Started:   radixBatch.Started,
			Ended:     radixBatch.Ended,
		},
	}
	var jobStatuses []modelsv1.JobStatus
	for _, jobStatus := range radixBatch.JobStatuses {
		jobStatuses = append(jobStatuses, getJobStatusFromRadixBatchJobsStatus(batchName, jobStatus))
	}
	jobStatus.JobStatuses = jobStatuses
	return &jobStatus
}

func getJobStatusFromRadixBatchJobsStatus(batchName string, jobStatus modelsv2.RadixBatchJobStatus) modelsv1.JobStatus {
	return modelsv1.JobStatus{
		JobId:     jobStatus.JobId,
		BatchName: batchName,
		Name:      jobStatus.Name,
		Created:   jobStatus.CreationTime,
		Started:   jobStatus.Started,
		Ended:     jobStatus.Ended,
		Status:    jobStatus.Status,
		Message:   jobStatus.Message,
	}
}
