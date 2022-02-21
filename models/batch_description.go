package models

// BatchScheduleDescription holds description about batch scheduling job
// swagger:model BatchScheduleDescription
type BatchScheduleDescription struct {
	JobScheduleDescriptions []JobScheduleDescription

	// DefaultRadixJobComponentConfig holding data realating to resources default configuration
	//
	// required: false
	DefaultRadixJobComponentConfig RadixJobComponentConfig `json:",inline"`
}
