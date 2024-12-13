package common_test

import (
	"testing"

	"github.com/equinor/radix-job-scheduler/models/common"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func TestXxx(t *testing.T) {
	sut := common.Node{
		Gpu:      "gpu1, gpu2",
		GpuCount: "2",
	}
	expected := &radixv1.RadixNode{
		Gpu:      "gpu1, gpu2",
		GpuCount: "2",
	}
	actual := sut.MapToRadixNode()
	assert.Equal(t, expected, actual)
}
