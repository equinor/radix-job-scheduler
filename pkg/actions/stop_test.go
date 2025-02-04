package actions_test

import (
	"testing"

	"github.com/equinor/radix-job-scheduler/pkg/actions"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
)

func Test_StopAllRadixBatchJobs_BatchCompletedError(t *testing.T) {
	rb := radixv1.RadixBatch{
		Status: radixv1.RadixBatchStatus{Condition: radixv1.RadixBatchCondition{Type: radixv1.BatchConditionTypeActive}},
	}
	_, err := actions.StopAllRadixBatchJobs(&rb)
	assert.NotErrorIs(t, err, actions.ErrStopCompletedRadixBatch)
}
