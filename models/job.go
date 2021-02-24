package models

import (
	"github.com/equinor/radix-job-scheduler/api/utils"
	v1 "k8s.io/api/batch/v1"
)

// Job holds general information about job
// swagger:model Job
type Job struct {
	// Name of the job
	//
	// required: false
	// example: radix-pipeline-20181029135644-algpv-6hznh
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

// GetJobFromK8sJob Gets job from a k8s job
func GetJobFromK8sJob(job *v1.Job) *Job {
	return &Job{
		Name:    job.GetName(),
		Started: utils.FormatTime(job.Status.StartTime),
		Ended:   utils.FormatTime(job.Status.CompletionTime),
		Status:  GetStatusFromJobStatus(job.Status).String(),
	}
}
