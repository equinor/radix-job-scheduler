package status

import (
	"github.com/equinor/radix-job-scheduler/models/common"
	"k8s.io/api/batch/v1"
)

// GetStatusFromJob Gets status from kubernetes job status
func GetStatusFromJob(job *v1.Job) common.ProgressStatus {
	jobStatus := job.Status
	backoffLimit := getJobBackoffLimit(job)
	switch {
	case jobStatus.Active > 0:
		if jobStatus.Ready != nil && jobStatus.Active == *jobStatus.Ready {
			return common.Running
		}
		return common.Active
	case jobStatus.Failed == backoffLimit:
		return common.Failed
	case jobStatus.Succeeded > 0:
		return common.Succeeded
	case jobStatus.Active == 0 && jobStatus.Failed+jobStatus.Succeeded < backoffLimit:
		return common.Waiting
	}
	var status common.ProgressStatus
	return status
}

func getJobBackoffLimit(job *v1.Job) int32 {
	if job.Spec.BackoffLimit == nil {
		return *job.Spec.BackoffLimit
	}
	return 0
}
