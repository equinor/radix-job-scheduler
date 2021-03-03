package models

import (
	"github.com/equinor/radix-job-scheduler/api/utils"
	v1 "k8s.io/api/batch/v1"
)

// JobStatus holds general information about job status
// swagger:model JobStatus
type JobStatus struct {
	// Name of the job
	// required: true
	// example: radix-component-algpv-6hznh
	Name string `json:"name"`

	// Started timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Started string `json:"started"`

	// Ended timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Ended string `json:"ended"`

	// Status of the job
	//
	// required: false
	// Enum: Waiting,Running,Succeeded,Stopping,Stopped,Failed
	// example: Waiting
	Status string `json:"status"`
}

// GetJobStatusFromJob Gets job from a k8s job
func GetJobStatusFromJob(job *v1.Job) *JobStatus {
	return &JobStatus{
		Name:    job.GetName(),
		Started: utils.FormatTime(job.Status.StartTime),
		Ended:   utils.FormatTime(job.Status.CompletionTime),
		Status:  GetStatusFromJobStatus(job.Status).String(),
	}
}
