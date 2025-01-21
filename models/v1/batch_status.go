package v1

import "time"

// BatchStatus holds general information about batch status
// swagger:model BatchStatus
type BatchStatus struct {
	//JobStatus Batch job status
	// JobStatus

	// Name of the batch
	// required: true
	Name string `json:"name"`

	// Defines a user defined ID of the batch.
	//
	// required: false
	// example: 'batch-id-1'
	BatchId string `json:"batchId,omitempty"`

	// Created timestamp
	//
	// required: false
	// swagger:strfmt date-time
	Created time.Time `json:"created"`

	// Started timestamp
	//
	// required: false
	// swagger:strfmt date-time
	Started *time.Time `json:"started"`

	// Ended timestamp
	//
	// required: false
	// swagger:strfmt date-time
	Ended *time.Time `json:"ended"`

	// Status of the batch.
	// - Waiting = All jobs are in waiting status
	// - Active = One or more jobs are active
	// - Completed = All jobs are done (failed, succeeded or stopped)
	//
	// If batchStatusRules is configured for the job in radixconfig,
	// the following statuses may also be returned (in addition to the three statuses above)
	// - Running
	// - Succeeded
	// - Failed
	// - Stopping
	// - Stopped
	//
	// Note that batchStatusRules can override the the meaning of the three standard statuses.
	//
	// required: false
	// Enum: Running,Succeeded,Failed,Waiting,Stopping,Stopped,Active,Completed
	// example: Waiting
	Status string `json:"status,omitempty"`

	// Message describing the reason for the status
	//
	// required: false
	Message string `json:"message,omitempty"`

	// DeploymentName for this batch
	//
	// required: false
	DeploymentName string `json:"DeploymentName,omitempty"`

	// JobStatuses of the jobs in the batch
	//
	// required: false
	JobStatuses []JobStatus `json:"jobStatuses,omitempty"`

	// BatchType Single job or multiple jobs batch
	//
	// required: false
	// example: "job"
	BatchType string `json:"batchType,omitempty"`
}
