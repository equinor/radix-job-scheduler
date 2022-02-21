package models

// BatchScheduleDescription holds description about batch scheduling job
// swagger:model BatchScheduleDescription
type BatchScheduleDescription struct {
	JobScheduleDescriptions []JobScheduleDescription

	// DefaultTimeLimitSeconds defines default maximum job run time in the batch. This can be overridden for a particular jobs. Corresponds to ActiveDeadlineSeconds in K8s.
	//
	// required: false
	DefaultTimeLimitSeconds *int64 `json:"timeLimitSeconds,omitempty"`
}
