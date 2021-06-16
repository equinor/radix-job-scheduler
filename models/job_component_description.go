package models

import v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

type RadixJobComponentConfig struct {
	// Resource describes the compute resource requirements.
	//
	// required: false
	Resources *v1.ResourceRequirements `json:"resources,omitempty"`
	// Node defines node attributes, where container should be scheduled
	//
	// required: false
	Node *v1.RadixNode `json:"node,omitempty"`
}

// JobScheduleDescription holds description about scheduling job
// swagger:model JobScheduleDescription
type JobScheduleDescription struct {
	// Payload holding json data to be mapped to component
	//
	// required: false
	// example: {'data':'value'}
	Payload string `json:"payload"`
	// Payload holding data realating to resource configuration
	//
	// required: false
	RadixJobComponentConfig `json:",inline"`
}
