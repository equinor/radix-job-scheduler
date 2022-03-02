package models

// BatchStatus holds general information about batch status
// swagger:model BatchStatus
type BatchStatus struct {
	//JobStatus Batch job status
	JobStatus
	// JobStatuses of the jobs in the batch
	// required: false
	JobStatuses []JobStatus `json:"jobStatuses,omitempty"`
}
