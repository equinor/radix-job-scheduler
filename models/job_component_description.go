package models

// JobScheduleDescription holds description about scheduling job
// swagger:model JobScheduleDescription
type JobScheduleDescription struct {
	// Payload holding json data to be mapped to component
	//
	// required: false
	// example: {'data':'value'}
	Payload string `json:"payload"`
}
