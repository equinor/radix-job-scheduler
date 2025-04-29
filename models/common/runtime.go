package common

import radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// Runtime defines the component or job's target runtime requirements
type Runtime struct {
	// CPU architecture target for the component or job. Defaults to amd64.
	// +kubebuilder:validation:Enum=amd64;arm64
	// +kubebuilder:default:=amd64
	// +optional
	Architecture radixv1.RuntimeArchitecture `json:"architecture,omitempty"`

	// Defines the node type for the component. It is a node-pool label and taint, where the component's or job's pods will be scheduled.
	// More info: https://www.radix.equinor.com/radix-config#nodetype
	// +kubebuilder:validation:MaxLength=120
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$
	// +optional
	NodeType *string `json:"nodeType,omitempty"`
}
