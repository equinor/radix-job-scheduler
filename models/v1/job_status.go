package v1

import "time"

// JobStatus holds general information about job status
// swagger:model JobStatus
type JobStatus struct {
	// JobId Optional ID of a job
	//
	// required: false
	// example: 'job1'
	JobId string `json:"jobId,omitempty"`
	// BatchName Optional Batch ID of a job
	//
	// required: false
	// example: 'batch1'
	BatchName string `json:"batchName,omitempty"`

	// Name of the job
	// required: true
	// example: calculator
	Name string `json:"name"`

	// Created timestamp
	//
	// required: true
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	Created time.Time `json:"created"`

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
	// Enum: Waiting,Running,Succeeded,Stopping,Stopped,Failed,DeadlineExceeded
	// example: Waiting
	Status string `json:"status,omitempty"`

	// Message, if any, of the job
	//
	// required: false
	// example: "Error occurred"
	Message string `json:"message,omitempty"`

	// Updated timestamp when the status was updated
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Updated string `json:"updated,omitempty"`

	// PodStatuses for each pod of the job
	// required: false
	PodStatuses []PodStatus `json:"podStatuses,omitempty"`
}

// PodStatus contains details for the current status of the job's pods.
// swagger:model PodStatus
type PodStatus struct {
	// Pod name
	//
	// required: true
	// example: server-78fc8857c4-hm76l
	Name string `json:"name"`

	// Created timestamp
	//
	// required: true
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	Created time.Time `json:"created,omitempty"`

	// The time at which the batch job's pod startedAt
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	StartTime *time.Time `json:"startTime,omitempty"`

	// The time at which the batch job's pod finishedAt.
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	EndTime *time.Time `json:"endTime,omitempty"`

	// Container started timestamp
	//
	// required: false
	// swagger:strfmt date-time
	// example: 2006-01-02T15:04:05Z
	ContainerStarted *time.Time `json:"containerStarted,omitempty"`

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
// swagger:model ReplicaStatus
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
