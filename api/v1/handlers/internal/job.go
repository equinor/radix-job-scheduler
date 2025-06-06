package internal

import (
	"context"
	"fmt"
	"strings"

	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
)

// GetBatchJobStatus Get batch job
func GetBatchJobStatus(ctx context.Context, batchStatus *modelsv1.BatchStatus, batchName, jobName string) (*modelsv1.JobStatus, error) {
	for _, jobStatus := range batchStatus.JobStatuses {
		if !strings.EqualFold(jobStatus.Name, jobName) {
			continue
		}
		return &jobStatus, nil
	}
	return nil, fmt.Errorf("not found")
}
