package common

import (
	"reflect"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

// Runtime defines the component or job's target runtime requirements
type Runtime struct {
	// CPU architecture
	//
	// example: amd64
	Architecture string `json:"architecture"`

	// Defines the node type for the component. It is a node-pool label and taint, where the component's or job's pods will be scheduled.
	// More info: https://www.radix.equinor.com/radix-config#nodetype
	// +kubebuilder:validation:MaxLength=120
	// +kubebuilder:validation:Pattern=^(([a-z0-9][-a-z0-9]*)?[a-z0-9])?$
	// +optional
	NodeType *string `json:"nodeType,omitempty"`
}

// EnvVars Map of environment variables in the form '<envvarname>: <value>'
type EnvVars map[string]string

// MapToRadixRuntime maps the object to a RadixV1 Runtime object
func (runtime *Runtime) MapToRadixRuntime() *radixv1.Runtime {
	if runtime == nil {
		return nil
	}
	return &radixv1.Runtime{
		Architecture: radixv1.RuntimeArchitecture(runtime.Architecture),
		NodeType:     runtime.NodeType,
	}
}

// MapToRadixEnvVarsMap maps the object to a RadixV1 EnvVarsMap
func (envVars EnvVars) MapToRadixEnvVarsMap() radixv1.EnvVarsMap {
	if envVars == nil {
		return nil
	}
	radixEnvVarMap := make(radixv1.EnvVarsMap, len(envVars))
	for name, value := range envVars {
		radixEnvVarMap[name] = value
	}
	return radixEnvVarMap
}

// RuntimeTransformer is a mergo transformer for the Runtime struct
type RuntimeTransformer struct{}

// Transformer implements the mergo.Transformer interface
func (transformer RuntimeTransformer) Transformer(t reflect.Type) func(dst, src reflect.Value) error {
	if t != reflect.TypeOf(new(Runtime)) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		if dst.Kind() != reflect.Ptr || src.Kind() != reflect.Ptr {
			return nil
		}
		if dst.IsNil() && !src.IsNil() {
			dst.Set(src)
		}
		return nil
	}
}
