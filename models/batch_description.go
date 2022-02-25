package models

// BatchScheduleDescription holds description about batch scheduling job
// swagger:model BatchScheduleDescription
type BatchScheduleDescription struct {
	// JobScheduleDescriptions descriptions about scheduling batch jobs
	//
	// required: true
	JobScheduleDescriptions []JobScheduleDescription `json:"jobScheduleDescriptions" yaml:"jobScheduleDescriptions"`

	// DefaultRadixJobComponentConfig holding data relating to resources default configuration
	//
	// required: false
	DefaultRadixJobComponentConfig *RadixJobComponentConfig `json:"defaultRadixJobComponentConfig,omitempty" yaml:"defaultRadixJobComponentConfig,omitempty"`
}
