package v1

import (
	"time"
)

// Status of the batch
// swagger:enum BatchStatusEnum
type BatchStatusEnum string

const (
	// Waiting means that all jobs are waiting to start
	//
	// This status can also be defined with batchStatusRules in radixconfig, which will override the original meaning
	BatchStatusEnumWaiting BatchStatusEnum = "Waiting"

	// Active means that one or more jobs are active
	//
	// This status can also be defined with batchStatusRules in radixconfig, which will override the original meaning
	BatchStatusEnumActive BatchStatusEnum = "Active"

	// Completed means that all jobs have completed (status stopped, succeeded or failed)
	//
	// This status can also be defined with batchStatusRules in radixconfig, which will override the original meaning
	BatchStatusEnumCompleted BatchStatusEnum = "Completed"

	// Active is a custom status that can only be defined with batchStatusRules in radixconfig
	BatchStatusEnumRunning BatchStatusEnum = "Running"

	// Succeeded is a custom status that can only be defined with batchStatusRules in radixconfig
	BatchStatusEnumSucceeded BatchStatusEnum = "Succeeded"

	// Failed is a custom status that can only be defined with batchStatusRules in radixconfig
	BatchStatusEnumFailed BatchStatusEnum = "Failed"

	// Stopping is a custom status that can only be defined with batchStatusRules in radixconfig
	BatchStatusEnumStopping BatchStatusEnum = "Stopping"

	// Stopped is a custom status that can only be defined with batchStatusRules in radixconfig
	BatchStatusEnumStopped BatchStatusEnum = "Stopped"
)

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
	//
	// required: false
	// example: Waiting
	Status BatchStatusEnum `json:"status,omitempty"`

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
