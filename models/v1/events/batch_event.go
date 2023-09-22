package events

import "github.com/equinor/radix-job-scheduler/models/v1"

// BatchEvent holds general information about batch event on change of status
// swagger:model BatchEvent
type BatchEvent struct {
	// BatchStatus Batch job status
	v1.BatchStatus

	// Event Event type
	//
	// required: true
	// example: "Created"
	Event Event `json:"event,omitempty"`
}

type Event string

const (
	Created Event = "Created"
	Updated Event = "Updated"
	Deleted Event = "Deleted"
)
