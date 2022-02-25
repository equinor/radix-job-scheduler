package models

import v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

//RadixJobComponentConfig holds description of RadixJobComponent
type RadixJobComponentConfig struct {
	// Resource describes the compute resource requirements.
	//
	// required: false
	Resources *v1.ResourceRequirements `json:"resources,omitempty"`
	// Node defines node attributes, where container should be scheduled
	//
	// required: false
	Node *v1.RadixNode `json:"node,omitempty"`

	// TimeLimitSeconds defines maximum job run time. Corresponds to ActiveDeadlineSeconds in K8s.
	//
	// required: false
	TimeLimitSeconds *int64 `json:"timeLimitSeconds,omitempty"`
}

// JobScheduleDescription holds description about scheduling job
// swagger:model JobScheduleDescription
type JobScheduleDescription struct {
	// JobId Optional ID of a job
	//
	// required: false
	// example: 'job1'
	JobId string `json:"jobId,omitempty"`

	// Payload holding json data to be mapped to component
	//
	// required: false
	// example: {'data':'value'}
	Payload string `json:"payload"`

	// RadixJobComponentConfig holding data relating to resource configuration
	//
	// required: false
	RadixJobComponentConfig `json:",inline"`
}
