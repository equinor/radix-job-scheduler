package internal

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-job-scheduler/api/v1"
	internal "github.com/equinor/radix-job-scheduler/internal"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
)

// GetBatchJob Get batch job
func GetBatchJob(ctx context.Context, handler v1.Handler, batchName, jobName string) (*modelsv1.JobStatus, error) {
	radixBatch, err := handler.GetRadixBatchStatus(ctx, batchName)
	if err != nil {
		return nil, err
	}
	for _, jobStatus := range radixBatch.JobStatuses {
		if !strings.EqualFold(jobStatus.Name, jobName) {
			continue
		}
		return &jobStatus, nil
	}
	return nil, fmt.Errorf("not found")
}

// StopJob Stop a job
func StopJob(ctx context.Context, handler v1.Handler, jobName string) error {
	if batchName, batchJobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobName); ok {
		return handler.StopRadixBatchJob(ctx, batchName, batchJobName)
	}
	return fmt.Errorf("stop of this job is not supported")
}
