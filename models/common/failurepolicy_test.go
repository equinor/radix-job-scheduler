package common_test

import (
	"testing"

	"github.com/equinor/radix-job-scheduler/models/common"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_MapToRadixFailurePolicy(t *testing.T) {
	sut := common.FailurePolicy{
		Rules: []common.FailurePolicyRule{
			{
				Action: common.FailurePolicyRuleActionCount,
				OnExitCodes: common.FailurePolicyRuleOnExitCodes{
					Operator: common.FailurePolicyRuleOnExitCodesOpIn,
					Values:   []int32{1, 2, 3},
				},
			},
			{
				Action: common.FailurePolicyRuleActionFailJob,
				OnExitCodes: common.FailurePolicyRuleOnExitCodes{
					Operator: common.FailurePolicyRuleOnExitCodesOpIn,
					Values:   []int32{4},
				},
			},
			{
				Action: common.FailurePolicyRuleActionIgnore,
				OnExitCodes: common.FailurePolicyRuleOnExitCodes{
					Operator: common.FailurePolicyRuleOnExitCodesOpNotIn,
					Values:   []int32{5},
				},
			},
		},
	}
	expected := &radixv1.RadixJobComponentFailurePolicy{
		Rules: []radixv1.RadixJobComponentFailurePolicyRule{
			{
				Action: radixv1.RadixJobComponentFailurePolicyActionCount,
				OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{
					Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn,
					Values:   []int32{1, 2, 3},
				},
			},
			{
				Action: radixv1.RadixJobComponentFailurePolicyActionFailJob,
				OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{
					Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpIn,
					Values:   []int32{4},
				},
			},
			{
				Action: radixv1.RadixJobComponentFailurePolicyActionIgnore,
				OnExitCodes: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodes{
					Operator: radixv1.RadixJobComponentFailurePolicyRuleOnExitCodesOpNotIn,
					Values:   []int32{5},
				},
			},
		},
	}
	actual := sut.MapToRadixFailurePolicy()
	assert.Equal(t, expected, actual)
}
