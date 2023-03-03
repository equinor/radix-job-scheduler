package radix

import radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// Notifier to notify about RadixBatch events and changes
type Notifier interface {
	// Notify Send notification
	Notify(*radixv1.RadixBatch, []radixv1.RadixBatchJobStatus)
	// Enabled The notifier is enabled and can be used
	Enabled() bool
	// String Describes the notifier
	String() string
}
