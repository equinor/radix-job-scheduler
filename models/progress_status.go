package models

import (
	v1 "k8s.io/api/batch/v1"
)

// ProgressStatus Enumeration of the statuses of a job or step
type ProgressStatus int

const (
	// Running Active
	Running ProgressStatus = iota

	// Succeeded JobStatus/step succeeded
	Succeeded

	// Failed JobStatus/step failed
	Failed

	// Waiting JobStatus/step pending
	Waiting

	// Stopping job
	Stopping

	// Stopped job
	Stopped

	numStatuses
)

func (p ProgressStatus) String() string {
	if p >= numStatuses {
		return "Unsupported"
	}
	return [...]string{"Running", "Succeeded", "Failed", "Waiting", "Stopping", "Stopped"}[p]
}

// GetStatusFromJobStatus Gets status from kubernetes job status
func GetStatusFromJobStatus(jobStatus v1.JobStatus) ProgressStatus {
	var status ProgressStatus
	if jobStatus.Active > 0 {
		status = Running

	} else if jobStatus.Succeeded > 0 {
		status = Succeeded

	} else if jobStatus.Failed > 0 {
		status = Failed
	}

	return status
}
