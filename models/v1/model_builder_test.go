package v1_test

import (
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	v1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_BuildBatchStatus_BatchStatusEnumFromRadixBatchCondition(t *testing.T) {
	tests := map[string]struct {
		condition      radixv1.RadixBatchCondition
		expectedStatus v1.BatchStatusEnum
	}{
		"empty condition defaults to waiting": {
			condition:      radixv1.RadixBatchCondition{},
			expectedStatus: v1.BatchStatusEnumWaiting,
		},
		"waiting condition => waiting": {
			condition:      radixv1.RadixBatchCondition{Type: radixv1.BatchConditionTypeWaiting},
			expectedStatus: v1.BatchStatusEnumWaiting,
		},
		"active condition => active": {
			condition:      radixv1.RadixBatchCondition{Type: radixv1.BatchConditionTypeActive},
			expectedStatus: v1.BatchStatusEnumActive,
		},
		"completed condition => completed": {
			condition:      radixv1.RadixBatchCondition{Type: radixv1.BatchConditionTypeCompleted},
			expectedStatus: v1.BatchStatusEnumCompleted,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			rb := radixv1.RadixBatch{Status: radixv1.RadixBatchStatus{Condition: test.condition}}
			actual := v1.BuildBatchStatus(rb, nil)
			assert.Equal(t, test.expectedStatus, actual.Status)
		})
	}
}

func Test_BuildBatchStatus_BatchStatusPropsSetCorrectly(t *testing.T) {
	createdTime, activeTime, completionTime := time.Now(), time.Now().Add(10*time.Second), time.Now().Add(20*time.Second)

	tests := map[string]struct {
		sourceRb radixv1.RadixBatch
		expected v1.BatchStatus
	}{
		"all non-nil properties set": {
			sourceRb: radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "any-rb-name",
					CreationTimestamp: metav1.NewTime(createdTime),
					Labels:            map[string]string{kube.RadixBatchTypeLabel: "any-rb-type"},
				},
				Spec: radixv1.RadixBatchSpec{
					RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
						LocalObjectReference: radixv1.LocalObjectReference{Name: "any-rd"},
					},
					BatchId: "any-batch-id",
				},
				Status: radixv1.RadixBatchStatus{
					Condition: radixv1.RadixBatchCondition{
						Message: "any-condition-message",
					},
				},
			},
			expected: v1.BatchStatus{
				Name:           "any-rb-name",
				DeploymentName: "any-rd",
				Created:        createdTime,
				BatchType:      "any-rb-type",
				BatchId:        "any-batch-id",
				Message:        "any-condition-message",
				JobStatuses:    []v1.JobStatus{},
				Status:         v1.BatchStatusEnumWaiting,
			},
		},
		"Started set from ActiveTime": {
			sourceRb: radixv1.RadixBatch{
				Status: radixv1.RadixBatchStatus{
					Condition: radixv1.RadixBatchCondition{
						ActiveTime: pointers.Ptr(metav1.NewTime(activeTime)),
					},
				},
			},
			expected: v1.BatchStatus{
				Started:     &activeTime,
				JobStatuses: []v1.JobStatus{},
				Status:      v1.BatchStatusEnumWaiting,
			},
		},
		"Ended set from CompletionTime": {
			sourceRb: radixv1.RadixBatch{
				Status: radixv1.RadixBatchStatus{
					Condition: radixv1.RadixBatchCondition{
						CompletionTime: pointers.Ptr(metav1.NewTime(completionTime)),
					},
				},
			},
			expected: v1.BatchStatus{
				Ended:       &completionTime,
				JobStatuses: []v1.JobStatus{},
				Status:      v1.BatchStatusEnumWaiting,
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			actual := v1.BuildBatchStatus(test.sourceRb, nil)
			assert.Equal(t, test.expected, actual)
		})
	}
}
