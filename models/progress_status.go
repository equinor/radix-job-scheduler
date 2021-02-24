package models

import (
	"fmt"
	v1 "k8s.io/api/batch/v1"
)

// ProgressStatus Enumeration of the statuses of a job or step
type ProgressStatus int

const (
	// Running Active
	Running ProgressStatus = iota

	// Succeeded Job/step succeeded
	Succeeded

	// Failed Job/step failed
	Failed

	// Waiting Job/step pending
	Waiting

	// Stopping job
	Stopping

	// Stopped job
	Stopped

	numStatuses
)

func (p ProgressStatus) String() string {
	return [...]string{"Running", "Succeeded", "Failed", "Waiting", "Stopping", "Stopped"}[p]
}

// GetStatusFromName Gets status from name
func GetStatusFromName(name string) (ProgressStatus, error) {
	for status := Running; status < numStatuses; status++ {
		if status.String() == name {
			return status, nil
		}
	}

	return numStatuses, fmt.Errorf("No progress status found by name %s", name)
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
