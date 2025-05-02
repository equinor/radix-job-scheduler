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

func (runtime *Runtime) getArchitecture() string {
	if runtime == nil {
		return ""
	}
	return runtime.Architecture
}

func (runtime *Runtime) getNodeType() *string {
	if runtime == nil {
		return nil
	}
	return runtime.NodeType
}

func (runtime *Runtime) setArchitecture(architecture string) {
	if runtime != nil {
		runtime.Architecture = architecture
	}
}

func (runtime *Runtime) setNodeType(nodeType *string) {
	if runtime != nil {
		runtime.NodeType = nodeType
	}
}

// RuntimeTransformer is a mergo transformer for the Runtime struct
type RuntimeTransformer struct{}

// Transformer implements the mergo.Transformer interface
func (transformer RuntimeTransformer) Transformer(t reflect.Type) func(dst, src reflect.Value) error {
	if t != reflect.TypeOf(Runtime{}) {
		return nil
	}

	return func(dst, src reflect.Value) error {
		var dstRuntime, srcRuntime Runtime
		if src.Kind() == reflect.Ptr && src.IsNil() {
			if dst.Kind() == reflect.Ptr && dst.IsNil() {
				return nil
			}
		} else {
			srcRuntime, _ = src.Interface().(Runtime)
		}
		if !dst.CanSet() {
			dst.Set(reflect.ValueOf(srcRuntime))
			return nil
		} else {
			dstRuntime, _ = dst.Interface().(Runtime)
		}

		if dstRuntime.getArchitecture() == "" && srcRuntime.getArchitecture() != "" {
			dstRuntime.setArchitecture(srcRuntime.Architecture)
		}

		if dstRuntime.getNodeType() == nil && srcRuntime.getNodeType() != nil {
			dstRuntime.setNodeType(srcRuntime.NodeType)
		}

		if dstRuntime.getArchitecture() != "" && dstRuntime.getNodeType() != nil {
			dstRuntime.setArchitecture("")
		}

		if dst.CanSet() {
			dst.Set(reflect.ValueOf(dstRuntime))
		}
		return nil
	}
}
