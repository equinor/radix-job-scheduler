package modelsv2

// RadixBatch holds general information about batch status
// swagger:model RadixBatch
type RadixBatch struct {
	//Name of the Radix batch
	// required: true
	Name string `json:"name"`

	//Radix batch creation timestamp
	//
	// required: true
	CreationTime string `json:"creationTime"`

	// Started timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Started string `json:"started,omitempty"`

	// Ended timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Ended string `json:"ended,omitempty"`

	// Status of the job
	//
	// required: false
	// Enum: Waiting,Running,Succeeded,Stopping,Stopped,Failed,DeadlineExceeded
	// example: Waiting
	Status string `json:"status,omitempty"`

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
}

// RadixBatchJobStatus holds general information about batch job status
// swagger:model RadixBatchJobStatus
type RadixBatchJobStatus struct {
	//Name of the Radix batch job
	// required: true
	Name string `json:"name"`

	//Radix batch job creation timestamp
	//
	// required: true
	CreationTime string `json:"creationTime"`

	// JobId Optional ID of a job
	//
	// required: false
	// example: 'job1'
	JobId string `json:"jobId,omitempty"`

	// Started timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Started string `json:"started,omitempty"`

	// Ended timestamp
	//
	// required: false
	// example: 2006-01-02T15:04:05Z
	Ended string `json:"ended,omitempty"`

	// Status of the job
	//
	// required: false
	// Enum: Waiting,Running,Succeeded,Stopping,Stopped,Failed,DeadlineExceeded
	// example: Waiting
	Status string `json:"status,omitempty"`

	// Status message, if any, of the job
	//
	// required: false
	// example: "Error occurred"
	Message string `json:"message,omitempty"`
}
