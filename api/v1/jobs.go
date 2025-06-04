package v1

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-job-scheduler/internal"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
)

// CopyJob Copy a job
func CopyJob(ctx context.Context, handlerApiV2 Handler, jobName, deploymentName string) (*modelsv1.BatchStatus, error) {
	return handlerApiV2.CopyRadixBatchJob(ctx, jobName, deploymentName)
}

// StopJob Stop a job
func StopJob(ctx context.Context, handlerApiV2 Handler, jobName string) error {
	if batchName, batchJobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobName); ok {
		return handlerApiV2.StopRadixBatchJob(ctx, batchName, batchJobName)
	}
	return fmt.Errorf("stop of this job is not supported")
}

// StopAllSingleJobs Stop alls jobs
func StopAllSingleJobs(ctx context.Context, handlerApiV2 Handler, componentName string) error {
	return handlerApiV2.StopAllSingleRadixJobs(ctx)
}

// GetBatchJob Get batch job
func GetBatchJob(ctx context.Context, handlerApiV2 Handler, batchName, jobName string) (*modelsv1.JobStatus, error) {
	radixBatch, err := handlerApiV2.GetRadixBatchStatus(ctx, batchName)
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
