package common_test

import (
	"testing"

	"dario.cat/mergo"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-job-scheduler/models/common"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func TestRuntimeTransformer_Merge(t *testing.T) {
	tests := []struct {
		name                     string
		jobConfig                common.RadixJobComponentConfig
		defaultConfig            common.RadixJobComponentConfig
		expectedJobRuntimeConfig common.RadixJobComponentConfig
	}{
		{
			name:          "fills missing Architecture and NodeType",
			defaultConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{Architecture: string(radixv1.RuntimeArchitectureAmd64)}},
			jobConfig:     common.RadixJobComponentConfig{Runtime: &common.Runtime{}},
			expectedJobRuntimeConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureAmd64),
			}},
		},
		{
			name: "preserves existing NodeType",
			defaultConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureAmd64),
			}},
			jobConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				NodeType: pointers.Ptr("gpu-nodes"),
			}},
			expectedJobRuntimeConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				NodeType: pointers.Ptr("gpu-nodes"),
				// Architecture should be cleared
			}},
		},
		{
			name:          "preserves existing Architecture if NodeType not set",
			defaultConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{}},
			jobConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
			expectedJobRuntimeConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
		},
		{
			name: "preserves job Architecture if NodeType not set",
			defaultConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureAmd64),
			}},
			jobConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
			expectedJobRuntimeConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
		},
		{
			name:          "preserves job Architecture when there is no default runtime",
			defaultConfig: common.RadixJobComponentConfig{},
			jobConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
			expectedJobRuntimeConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
		},
		{
			name: "sets job Architecture by default runtime",
			defaultConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
			jobConfig: common.RadixJobComponentConfig{},
			expectedJobRuntimeConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
		},
		{
			name: "no job Architecture if no default runtime and job runtime",
			defaultConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
			jobConfig: common.RadixJobComponentConfig{},
			expectedJobRuntimeConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureArm64),
			}},
		},
		{
			name: "clears Architecture if both present after merge",
			defaultConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				Architecture: string(radixv1.RuntimeArchitectureAmd64),
			}},
			jobConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				NodeType: pointers.Ptr("edge-nodes"),
			}},
			expectedJobRuntimeConfig: common.RadixJobComponentConfig{Runtime: &common.Runtime{
				NodeType: pointers.Ptr("edge-nodes"),
				// Architecture must be cleared
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dstJobConfig := tt.jobConfig
			defaultConfig := tt.defaultConfig
			err := mergo.Merge(&dstJobConfig, &defaultConfig, mergo.WithTransformers(common.RuntimeTransformer{}))
			assert.NoError(t, err)
			if tt.expectedJobRuntimeConfig.Runtime != nil {
				assert.NotNil(t, dstJobConfig.Runtime, "expected runtime to be set")
				assert.Equal(t, tt.expectedJobRuntimeConfig.Runtime.Architecture, dstJobConfig.Runtime.Architecture, "invalid architecture")
				assert.Equal(t, tt.expectedJobRuntimeConfig.Runtime.NodeType, dstJobConfig.Runtime.NodeType, "invalid node type")
			} else {
				assert.Nil(t, dstJobConfig.Runtime, "expected runtime to be nil")
			}
		})
	}
}
