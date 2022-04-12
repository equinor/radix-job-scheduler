package models

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
	// example: 2006-01-02T15:04:05Z
	Created string `json:"created"`

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
