package modelsv2

import radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// RadixBatch holds general information about batch status
// swagger:model RadixBatch
type RadixBatch struct {
	//Name of the Radix batch
	// required: true
	Name string `json:"name"`

	//Radix batch creation timestamp
	//
	// required: true
	CreationTime string `json:"creationTime"`

	// Status of the Radix batch
	// required: false
	Status radixv1.RadixBatchStatus `json:"status,omitempty"`
}
