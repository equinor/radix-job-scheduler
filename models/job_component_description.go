package models

// JobScheduleDescription holds description about scheduling job
// swagger:model JobScheduleDescription
type JobScheduleDescription struct {
	// ComponentName which job is scheduling
	// required: true
	// example: componentA
	ComponentName string `json:"componentName"`

	// Environment which job is scheduling component for
	// required: true
	// example: development
	Environment string `json:"environment"`

	// RadixDeploymentName holding details about scheduling component
	//
	// required: true
	// example: server-abc-dfg
	RadixDeploymentName string `json:"radixDeploymentName"`

	// Payload holding json data to be mapped to component
	//
	// required: false
	// example: {'data':'value'}
	Payload string `json:"payload"`
}
