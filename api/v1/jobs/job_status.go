package jobs

import (
	"fmt"

	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
)

// GetSingleJobStatusFromRadixBatchJob Gets job status from RadixBatch
func GetSingleJobStatusFromRadixBatchJob(radixBatch *modelsv2.RadixBatch) (*modelsv1.JobStatus, error) {
	if len(radixBatch.JobStatuses) != 1 {
		return nil, fmt.Errorf("batch should have only one job")
	}
	radixBatchJobStatus := radixBatch.JobStatuses[0]
	jobStatus := modelsv1.JobStatus{
		JobId:   radixBatchJobStatus.JobId,
		Name:    radixBatchJobStatus.Name,
		Created: radixBatchJobStatus.CreationTime,
		Started: radixBatchJobStatus.Started,
		Ended:   radixBatchJobStatus.Ended,
		Status:  radixBatchJobStatus.Status,
		Message: radixBatchJobStatus.Message,
	}
	return &jobStatus, nil
}
