package v1

// BatchStatus holds general information about batch status
// swagger:model BatchStatus
type BatchStatus struct {
	//JobStatus Batch job status
	JobStatus
	// JobStatuses of the jobs in the batch
	// required: false
	JobStatuses []JobStatus `json:"jobStatuses,omitempty"`

	// BatchType Single job or multiple jobs batch
	//
	// required: false
	// example: "job"
	BatchType string `json:"batchType,omitempty"`
}

/*
// BatchStatus holds general information about batch status
type BatchStatus struct {
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
	CreationTime time.Time `json:"creationTime"`

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
	// Enum: Running,Succeeded,Failed,Waiting,Stopping,Stopped,Active,Completed
	// example: Waiting
	Status v1.RadixBatchJobApiStatus `json:"status,omitempty"`

	// JobStatuses of the Radix batch jobs
	// required: false
	JobStatuses []JobStatus `json:"jobStatuses,omitempty"`

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

// JobStatus holds general information about batch job status
type JobStatus struct {
	// Name of the Radix batch job
	// required: true
	Name string `json:"name"`

	// Radix batch job creation timestamp
	//
	// required: false
	// swagger:strfmt date-time
	CreationTime *time.Time `json:"creationTime"`

	// JobId Optional ID of a job
	//
	// required: false
	// example: 'job1'
	JobId string `json:"jobId,omitempty"`

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
	// Enum: Waiting,Running,Succeeded,Stopping,Stopped,Failed,Completed
	// example: Waiting
	Status v1.RadixBatchJobApiStatus `json:"status,omitempty"`

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
	Created time.Time `json:"created"`

	// The time at which the batch job's pod startedAt
	//
	// required: false
	// swagger:strfmt date-time
	StartTime *time.Time `json:"startTime"`

	// The time at which the batch job's pod finishedAt.
	//
	// required: false
	// swagger:strfmt date-time
	EndTime *time.Time `json:"endTime"`

	// Container started timestamp
	//
	// required: false
	// swagger:strfmt date-time
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
*/
