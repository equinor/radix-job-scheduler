package models

// BatchScheduleDescription holds description about batch scheduling job
// swagger:model BatchScheduleDescription
type BatchScheduleDescription struct {
	// JobScheduleDescriptions descriptions of jobs to schedule within the batch
	//
	// required: true
	JobScheduleDescriptions []JobScheduleDescription `json:"jobScheduleDescriptions" yaml:"jobScheduleDescriptions"`

	// DefaultRadixJobComponentConfig default resources configuration
	//
	// required: false
	DefaultRadixJobComponentConfig *RadixJobComponentConfig `json:"defaultRadixJobComponentConfig,omitempty" yaml:"defaultRadixJobComponentConfig,omitempty"`
}
