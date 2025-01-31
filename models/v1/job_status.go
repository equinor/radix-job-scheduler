package v1

import (
	"slices"
	"time"
)

// Status of the job
// swagger:enum JobStatusEnum
type JobStatusEnum string

func (s JobStatusEnum) IsCompleted() bool {
	return slices.Contains([]JobStatusEnum{JobStatusEnumFailed, JobStatusEnumStopped, JobStatusEnumSucceeded}, s)
}

const (
	// Waiting means that the job is waiting to start
	JobStatusEnumWaiting JobStatusEnum = "Waiting"

	// Active means that the job is active, but the container is not yet running
	JobStatusEnumActive JobStatusEnum = "Active"

	// Active means that the job is active and the container is running
	JobStatusEnumRunning JobStatusEnum = "Running"

	// Succeeded means that the job has completed without errors
	JobStatusEnumSucceeded JobStatusEnum = "Succeeded"

	// Failed means that the job has failed
	JobStatusEnumFailed JobStatusEnum = "Failed"

	// Stopping means that a request to stop the job is sent, but not yet processed by Radix
	JobStatusEnumStopping JobStatusEnum = "Stopping"

	// Stopped means that the job has been stopped
	JobStatusEnumStopped JobStatusEnum = "Stopped"
)

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
	// required: false
	// swagger:strfmt date-time
	Created *time.Time `json:"created"`

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

	// Status of the job
	//
	// required: false
	// example: Waiting
	Status JobStatusEnum `json:"status,omitempty"`

	// Message, if any, of the job
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
	// required: false
	// swagger:strfmt date-time
	Created *time.Time `json:"created,omitempty"`

	// The time at which the batch job's pod startedAt
	//
	// required: false
	// swagger:strfmt date-time
	StartTime *time.Time `json:"startTime,omitempty"`

	// The time at which the batch job's pod finishedAt.
	//
	// required: false
	// swagger:strfmt date-time
	EndTime *time.Time `json:"endTime,omitempty"`

	// Container started timestamp
	//
	// required: false
	// swagger:strfmt date-time
	ContainerStarted *time.Time `json:"containerStarted,omitempty"` //TODO: We should deprecate and finally remove this since is has the same value as StartTime

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
	// - Stopped = Job has been stopped
	//
	// required: true
	// enum: Pending,Succeeded,Failing,Failed,Running,Terminated,Starting,Stopped
	// example: Running
	Status string `json:"status"`
}
