package v2

import (
	"time"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// RadixBatch holds general information about batch status
type RadixBatch struct {
	// Name of the Radix batch
	// required: true
	Name string `json:"name"`

	// Defines a user defined ID of the batch.
	//
	// required: false
	// example: 'batch-id-1'
	BatchId string `json:"batchId,omitempty"`

	// Radix batch creation timestamp
	//
	// required: true
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	CreationTime time.Time `json:"creationTime"`

	// Started timestamp
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	Started *time.Time `json:"started"`

	// Ended timestamp
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	Ended *time.Time `json:"ended"`

	// Status of the job
	//
	// required: false
	// Enum: Running,Succeeded,Failed,Waiting,Stopping,Stopped,Active,Completed
	// example: Waiting
	Status radixv1.RadixBatchJobApiStatus `json:"status,omitempty"`

	// JobStatuses of the Radix batch jobs
	// required: false
	JobStatuses []RadixBatchJobStatus `json:"jobStatuses,omitempty"`

	// Status message, if any, of the job
	//
	// required: false
	// example: "Error occurred"
	Message string `json:"message,omitempty"`

	// BatchType Single job or multiple jobs batch
	//
	// required: true
	// example: "job"
	BatchType string `json:"batchType"`

	// DeploymentName of this batch
	//
	// required: false
	DeploymentName string
}

// RadixBatchJobStatus holds general information about batch job status
type RadixBatchJobStatus struct {
	// Name of the Radix batch job
	// required: true
	Name string `json:"name"`

	// Radix batch job creation timestamp
	//
	// required: true
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	CreationTime time.Time `json:"creationTime"`

	// JobId Optional ID of a job
	//
	// required: false
	// example: 'job1'
	JobId string `json:"jobId,omitempty"`

	// Started timestamp
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	Started *time.Time `json:"started"`

	// Ended timestamp
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	Ended *time.Time `json:"ended"`

	// Status of the job
	//
	// required: false
	// Enum: Waiting,Running,Succeeded,Stopping,Stopped,Failed,Completed
	// example: Waiting
	Status radixv1.RadixBatchJobApiStatus `json:"status,omitempty"`

	// Status message, if any, of the job
	//
	// required: false
	// example: "Error occurred"
	Message string `json:"message,omitempty"`

	// The number of times the container for the job has failed.
	// +optional
	Failed int32 `json:"failed,omitempty"`

	// Timestamp of the job restart, if applied.
	// +optional
	Restart string `json:"restart,omitempty"`

	// PodStatuses for each pod of the job
	// required: false
	PodStatuses []RadixBatchJobPodStatus `json:"podStatuses,omitempty"`
}

// RadixBatchJobPodStatus contains details for the current status of the job's pods.
type RadixBatchJobPodStatus struct {
	// Pod name
	//
	// required: true
	// example: server-78fc8857c4-hm76l
	Name string `json:"name"`

	// Created timestamp
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	Created time.Time `json:"created"`

	// The time at which the batch job's pod startedAt
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	StartTime *time.Time `json:"startTime"`

	// The time at which the batch job's pod finishedAt.
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	EndTime *time.Time `json:"endTime"`

	// Container started timestamp
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	ContainerStarted *time.Time `json:"containerStarted"`

	// Status describes the component container status
	//
	// required: false
	Status ReplicaStatus `json:"replicaStatus,omitempty"`

	// StatusMessage provides message describing the status of a component container inside a pod
	//
	// required: false
	StatusMessage string `json:"statusMessage,omitempty"`

	// RestartCount count of restarts of a component container inside a pod
	//
	// required: false
	RestartCount int32 `json:"restartCount,omitempty"`

	// The image the container is running.
	//
	// required: false
	// example: radixdev.azurecr.io/app-server:cdgkg
	Image string `json:"image,omitempty"`

	// ImageID of the container's image.
	//
	// required: false
	// example: radixdev.azurecr.io/app-server@sha256:d40cda01916ef63da3607c03785efabc56eb2fc2e0dab0726b1a843e9ded093f
	ImageId string `json:"imageId,omitempty"`

	// The index of the pod in the re-starts
	PodIndex int `json:"podIndex,omitempty"`

	// Exit status from the last termination of the container
	ExitCode int32 `json:"exitCode"`

	// A brief CamelCase message indicating details about why the job is in this phase
	Reason string `json:"reason,omitempty"`
}

// ReplicaStatus describes the status of a component container inside a pod
type ReplicaStatus struct {
	// Status of the container
	// - Pending = Container in Waiting state and the reason is ContainerCreating
	// - Failed = Container is failed
	// - Failing = Container is failed
	// - Running = Container in Running state
	// - Succeeded = Container in Succeeded state
	// - Terminated = Container in Terminated state
	//
	// required: true
	// enum: Pending,Succeeded,Failing,Failed,Running,Terminated,Starting
	// example: Running
	Status string `json:"status"`
}
