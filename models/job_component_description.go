package models

// JobScheduleDescription holds description about scheduling job
// swagger:model JobScheduleDescription
type JobScheduleDescription struct {
	// RadixDeploymentName holding details about scheduling component
	//
	// required: true
	// example: server-abc-dfg
	RadixDeploymentName string `json:"radixDeploymentName"`

	// ComponentName which job is scheduling
	// required: true
	// example: componentAbc
	ComponentName string `json:"componentName"`

	// Payload holding json data to be mapped to component
	//
	// required: false
	// example: {'data':'value'}
	Payload string `json:"payload"`
}
