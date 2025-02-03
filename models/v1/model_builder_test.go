package v1_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	v1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_BuildBatchStatus_BatchStatusProps(t *testing.T) {
	createdTime, activeTime, completionTime := time.Now(), time.Now().Add(10*time.Second), time.Now().Add(20*time.Second)

	tests := map[string]struct {
		rb       radixv1.RadixBatch
		expected v1.BatchStatus
	}{
		"all non-nil properties set": {
			rb: radixv1.RadixBatch{
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
			rb: radixv1.RadixBatch{
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
			rb: radixv1.RadixBatch{
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
			actual := v1.BuildBatchStatus(test.rb, nil, nil)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func Test_BuildBatchStatus_BatchStatusEnum_FromRadixBatchCondition(t *testing.T) {
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
			actual := v1.BuildBatchStatus(rb, nil, nil)
			assert.Equal(t, test.expectedStatus, actual.Status)
		})
	}
}

func Test_BuildBatchStatus_BatchStatusEnum_FromRules(t *testing.T) {
	tests := map[string]struct {
		conditionType  radixv1.RadixBatchConditionType
		jobPhases      []radixv1.RadixBatchJobPhase
		rules          []radixv1.BatchStatusRule
		expectedStatus v1.BatchStatusEnum
	}{
		"matching all+in rule": {
			conditionType: radixv1.BatchConditionTypeActive,
			jobPhases:     []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning},
			rules: []radixv1.BatchStatusRule{
				{
					Condition:   radixv1.ConditionAll,
					Operator:    radixv1.OperatorIn,
					JobStatuses: []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning, radixv1.BatchJobPhaseFailed},
					BatchStatus: radixv1.RadixBatchJobApiStatusSucceeded,
				},
			},
			expectedStatus: v1.BatchStatusEnumSucceeded,
		},
		"non-matching all+in rule": {
			conditionType: radixv1.BatchConditionTypeActive,
			jobPhases:     []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning},
			rules: []radixv1.BatchStatusRule{
				{
					Condition:   radixv1.ConditionAll,
					Operator:    radixv1.OperatorIn,
					JobStatuses: []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseFailed},
					BatchStatus: radixv1.RadixBatchJobApiStatusSucceeded,
				},
			},
			expectedStatus: v1.BatchStatusEnumActive,
		},
		"matching all+notin rule": {
			conditionType: radixv1.BatchConditionTypeActive,
			jobPhases:     []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning},
			rules: []radixv1.BatchStatusRule{
				{
					Condition:   radixv1.ConditionAll,
					Operator:    radixv1.OperatorNotIn,
					JobStatuses: []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseFailed, radixv1.BatchJobPhaseSucceeded},
					BatchStatus: radixv1.RadixBatchJobApiStatusSucceeded,
				},
			},
			expectedStatus: v1.BatchStatusEnumSucceeded,
		},
		"non-matching all+notin rule": {
			conditionType: radixv1.BatchConditionTypeActive,
			jobPhases:     []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseFailed, radixv1.BatchJobPhaseRunning},
			rules: []radixv1.BatchStatusRule{
				{
					Condition:   radixv1.ConditionAll,
					Operator:    radixv1.OperatorNotIn,
					JobStatuses: []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseFailed, radixv1.BatchJobPhaseSucceeded},
					BatchStatus: radixv1.RadixBatchJobApiStatusSucceeded,
				},
			},
			expectedStatus: v1.BatchStatusEnumActive,
		},
		"matching any+in rule": {
			conditionType: radixv1.BatchConditionTypeActive,
			jobPhases:     []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning},
			rules: []radixv1.BatchStatusRule{
				{
					Condition:   radixv1.ConditionAny,
					Operator:    radixv1.OperatorIn,
					JobStatuses: []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseFailed},
					BatchStatus: radixv1.RadixBatchJobApiStatusSucceeded,
				},
			},
			expectedStatus: v1.BatchStatusEnumSucceeded,
		},
		"non-matching any+in rule": {
			conditionType: radixv1.BatchConditionTypeActive,
			jobPhases:     []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning},
			rules: []radixv1.BatchStatusRule{
				{
					Condition:   radixv1.ConditionAny,
					Operator:    radixv1.OperatorIn,
					JobStatuses: []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseFailed},
					BatchStatus: radixv1.RadixBatchJobApiStatusSucceeded,
				},
			},
			expectedStatus: v1.BatchStatusEnumActive,
		},
		"matching any+notin rule": {
			conditionType: radixv1.BatchConditionTypeActive,
			jobPhases:     []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseFailed},
			rules: []radixv1.BatchStatusRule{
				{
					Condition:   radixv1.ConditionAny,
					Operator:    radixv1.OperatorNotIn,
					JobStatuses: []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning},
					BatchStatus: radixv1.RadixBatchJobApiStatusSucceeded,
				},
			},
			expectedStatus: v1.BatchStatusEnumSucceeded,
		},
		"non-matching any+notin rule": {
			conditionType: radixv1.BatchConditionTypeActive,
			jobPhases:     []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning},
			rules: []radixv1.BatchStatusRule{
				{
					Condition:   radixv1.ConditionAny,
					Operator:    radixv1.OperatorNotIn,
					JobStatuses: []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning},
					BatchStatus: radixv1.RadixBatchJobApiStatusSucceeded,
				},
			},
			expectedStatus: v1.BatchStatusEnumActive,
		},
		"use first matching rule": {
			conditionType: radixv1.BatchConditionTypeActive,
			jobPhases:     []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive},
			rules: []radixv1.BatchStatusRule{
				{
					Condition:   radixv1.ConditionAll,
					Operator:    radixv1.OperatorIn,
					JobStatuses: []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseRunning},
					BatchStatus: radixv1.RadixBatchJobApiStatusSucceeded,
				},
				{
					Condition:   radixv1.ConditionAll,
					Operator:    radixv1.OperatorIn,
					JobStatuses: []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive},
					BatchStatus: radixv1.RadixBatchJobApiStatusFailed,
				},
				{
					Condition:   radixv1.ConditionAll,
					Operator:    radixv1.OperatorIn,
					JobStatuses: []radixv1.RadixBatchJobPhase{radixv1.BatchJobPhaseActive},
					BatchStatus: radixv1.RadixBatchJobApiStatusStopped,
				},
			},
			expectedStatus: v1.BatchStatusEnumFailed,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			var jobs []radixv1.RadixBatchJob
			var jobStatuses []radixv1.RadixBatchJobStatus
			for i, phase := range test.jobPhases {
				jobName := fmt.Sprintf("job%v", i)
				jobs = append(jobs, radixv1.RadixBatchJob{Name: jobName})
				jobStatuses = append(jobStatuses, radixv1.RadixBatchJobStatus{Name: jobName, Phase: phase})
			}
			rb := radixv1.RadixBatch{
				Spec: radixv1.RadixBatchSpec{
					Jobs: jobs,
				},
				Status: radixv1.RadixBatchStatus{
					Condition:   radixv1.RadixBatchCondition{Type: test.conditionType},
					JobStatuses: jobStatuses,
				},
			}
			actual := v1.BuildBatchStatus(rb, test.rules, nil)
			assert.Equal(t, test.expectedStatus, actual.Status)
		})
	}
}

func Test_BuildBatchStatus_JobStatusProps(t *testing.T) {
	anyTime := time.Now()
	tests := map[string]struct {
		rb                  radixv1.RadixBatch
		expectedJobStatuses []v1.JobStatus
	}{
		"all non-nil job props set": {
			rb: radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{Name: "batch1"},
				Spec: radixv1.RadixBatchSpec{
					Jobs: []radixv1.RadixBatchJob{{
						Name:  "job1",
						JobId: "jobid1",
					}},
				},
				Status: radixv1.RadixBatchStatus{
					JobStatuses: []radixv1.RadixBatchJobStatus{{
						Name:    "job1",
						Message: "anymessage",
						Failed:  4,
						Restart: "anyrestart",
					}},
				},
			},
			expectedJobStatuses: []v1.JobStatus{{
				Name:        "batch1-job1",
				BatchName:   "batch1",
				JobId:       "jobid1",
				Message:     "anymessage",
				Failed:      4,
				Restart:     "anyrestart",
				Status:      v1.JobStatusEnumWaiting,
				PodStatuses: []v1.PodStatus{},
			}},
		},
		"Created set from CreationTime": {
			rb: radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{Name: "batch1"},
				Spec: radixv1.RadixBatchSpec{
					Jobs: []radixv1.RadixBatchJob{{Name: "job1"}},
				},
				Status: radixv1.RadixBatchStatus{
					JobStatuses: []radixv1.RadixBatchJobStatus{{
						Name:         "job1",
						CreationTime: pointers.Ptr(metav1.NewTime(anyTime)),
					}},
				},
			},
			expectedJobStatuses: []v1.JobStatus{{
				Created:     &anyTime,
				Name:        "batch1-job1",
				BatchName:   "batch1",
				Status:      v1.JobStatusEnumWaiting,
				PodStatuses: []v1.PodStatus{},
			}},
		},
		"Started set from StartTime": {
			rb: radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{Name: "batch1"},
				Spec: radixv1.RadixBatchSpec{
					Jobs: []radixv1.RadixBatchJob{{Name: "job1"}},
				},
				Status: radixv1.RadixBatchStatus{
					JobStatuses: []radixv1.RadixBatchJobStatus{{
						Name:      "job1",
						StartTime: pointers.Ptr(metav1.NewTime(anyTime)),
					}},
				},
			},
			expectedJobStatuses: []v1.JobStatus{{
				Started:     &anyTime,
				Name:        "batch1-job1",
				BatchName:   "batch1",
				Status:      v1.JobStatusEnumWaiting,
				PodStatuses: []v1.PodStatus{},
			}},
		},
		"Ended set from EndTime": {
			rb: radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{Name: "batch1"},
				Spec: radixv1.RadixBatchSpec{
					Jobs: []radixv1.RadixBatchJob{{Name: "job1"}},
				},
				Status: radixv1.RadixBatchStatus{
					JobStatuses: []radixv1.RadixBatchJobStatus{{
						Name:    "job1",
						EndTime: pointers.Ptr(metav1.NewTime(anyTime)),
					}},
				},
			},
			expectedJobStatuses: []v1.JobStatus{{
				Ended:       &anyTime,
				Name:        "batch1-job1",
				BatchName:   "batch1",
				Status:      v1.JobStatusEnumWaiting,
				PodStatuses: []v1.PodStatus{},
			}},
		},
		"BatchName not set when batch type is Job": {
			rb: radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: map[string]string{kube.RadixBatchTypeLabel: string(kube.RadixBatchTypeJob)}},
				Spec: radixv1.RadixBatchSpec{
					Jobs: []radixv1.RadixBatchJob{{Name: "job1"}},
				},
			},
			expectedJobStatuses: []v1.JobStatus{{
				Name:   "batch1-job1",
				Status: v1.JobStatusEnumWaiting,
			}},
		},
		"all podstatuses mapped": {
			rb: radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{Name: "batch1"},
				Spec: radixv1.RadixBatchSpec{
					Jobs: []radixv1.RadixBatchJob{{Name: "job1"}},
				},
				Status: radixv1.RadixBatchStatus{
					JobStatuses: []radixv1.RadixBatchJobStatus{{Name: "job1", RadixBatchJobPodStatuses: []radixv1.RadixBatchJobPodStatus{
						{Name: "pod1"},
						{Name: "pod2"},
					}}},
				},
			},
			expectedJobStatuses: []v1.JobStatus{{
				Name:        "batch1-job1",
				BatchName:   "batch1",
				Status:      v1.JobStatusEnumWaiting,
				PodStatuses: []v1.PodStatus{{Name: "pod1"}, {Name: "pod2"}},
			}},
		},
		"all jobs mapped": {
			rb: radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{Name: "batch1"},
				Spec: radixv1.RadixBatchSpec{
					Jobs: []radixv1.RadixBatchJob{{Name: "job1"}, {Name: "job2"}},
				},
			},
			expectedJobStatuses: []v1.JobStatus{
				{Name: "batch1-job1", BatchName: "batch1", Status: v1.JobStatusEnumWaiting},
				{Name: "batch1-job2", BatchName: "batch1", Status: v1.JobStatusEnumWaiting},
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actual := v1.BuildBatchStatus(test.rb, nil, nil)
			assert.ElementsMatch(t, test.expectedJobStatuses, actual.JobStatuses)
		})
	}
}

func Test_BuildBatchStatus_JobStatus_MessageFromEvents(t *testing.T) {
	namespace := "anyns"

	tests := map[string]struct {
		jobStatusMessage string
		jobPodNames      []string
		events           []corev1.Event
		expectedMessage  string
	}{
		"message from event matching pod when jobstatus message is empty": {
			jobStatusMessage: "",
			jobPodNames:      []string{"jobpod"},
			events: []corev1.Event{
				{
					InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: namespace, Name: "jobpod"},
					LastTimestamp:  metav1.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
					Message:        "jobpod first event",
				},
			},
			expectedMessage: "jobpod first event",
		},
		"ignore event message when jobstatus message is set": {
			jobStatusMessage: "existing message",
			jobPodNames:      []string{"jobpod"},
			events: []corev1.Event{
				{
					InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: namespace, Name: "jobpod"},
					LastTimestamp:  metav1.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
					Message:        "jobpod first event",
				},
			},
			expectedMessage: "existing message",
		},
		"get message from last matching pod event": {
			jobStatusMessage: "",
			jobPodNames:      []string{"pod1", "pod2", "pod3"},
			events: []corev1.Event{
				{
					InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: namespace, Name: "pod1"},
					LastTimestamp:  metav1.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
					Message:        "pod1 first event",
				},
				{
					InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: namespace, Name: "pod1"},
					LastTimestamp:  metav1.Date(2020, 1, 1, 0, 10, 0, 0, time.UTC),
					Message:        "pod1 second event",
				},
				{
					InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: namespace, Name: "pod2"},
					LastTimestamp:  metav1.Date(2020, 1, 1, 0, 1, 0, 0, time.UTC),
					Message:        "pod2 first event",
				},
				{
					InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: namespace, Name: "pod2"},
					LastTimestamp:  metav1.Date(2020, 1, 1, 0, 15, 0, 0, time.UTC),
					Message:        "pod2 second event",
				},
				{
					InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: namespace, Name: "pod3"},
					LastTimestamp:  metav1.Date(2020, 1, 1, 0, 2, 0, 0, time.UTC),
					Message:        "pod3 first event",
				},
				{
					InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: namespace, Name: "pod3"},
					LastTimestamp:  metav1.Date(2020, 1, 1, 0, 14, 0, 0, time.UTC),
					Message:        "pod3 second event",
				},
				{
					InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: namespace, Name: "otherpod"},
					LastTimestamp:  metav1.Date(2020, 1, 1, 0, 50, 0, 0, time.UTC),
					Message:        "otherpod first event",
				},
				{
					InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: "otherns", Name: "pod1"},
					LastTimestamp:  metav1.Date(2020, 1, 1, 0, 50, 0, 0, time.UTC),
					Message:        "pod1 in otherns first event",
				},
				{
					InvolvedObject: corev1.ObjectReference{Kind: "OtherKind", Namespace: namespace, Name: "pod1"},
					LastTimestamp:  metav1.Date(2020, 1, 1, 0, 50, 0, 0, time.UTC),
					Message:        "pod1 otherkind first event",
				},
			},
			expectedMessage: "pod2 second event",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			rb := radixv1.RadixBatch{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
				Spec:       radixv1.RadixBatchSpec{Jobs: []radixv1.RadixBatchJob{{Name: "anyjob"}}},
				Status: radixv1.RadixBatchStatus{JobStatuses: []radixv1.RadixBatchJobStatus{
					{
						Name:                     "anyjob",
						Message:                  test.jobStatusMessage,
						RadixBatchJobPodStatuses: slice.Map(test.jobPodNames, func(n string) radixv1.RadixBatchJobPodStatus { return radixv1.RadixBatchJobPodStatus{Name: n} }),
					},
				}},
			}
			actual := v1.BuildBatchStatus(rb, nil, test.events)
			require.Len(t, actual.JobStatuses, 1)
			assert.Equal(t, test.expectedMessage, actual.JobStatuses[0].Message)
		})
	}
}

func Test_BuildBatchStatus_JobStatusEnum(t *testing.T) {
	tests := map[string]struct {
		stopJob        *bool
		jobPhase       radixv1.RadixBatchJobPhase
		expectedStatus v1.JobStatusEnum
	}{
		"phase not set and stopJob not set => waiting": {
			stopJob:        nil,
			jobPhase:       "",
			expectedStatus: v1.JobStatusEnumWaiting,
		},
		"phase not set and stopJob false => waiting": {
			stopJob:        pointers.Ptr(false),
			jobPhase:       "",
			expectedStatus: v1.JobStatusEnumWaiting,
		},
		"phase not set and stopJob true => stopping": {
			stopJob:        pointers.Ptr(true),
			jobPhase:       "",
			expectedStatus: v1.JobStatusEnumStopping,
		},
		"waiting and stopJob not set => waiting": {
			stopJob:        nil,
			jobPhase:       radixv1.BatchJobPhaseWaiting,
			expectedStatus: v1.JobStatusEnumWaiting,
		},
		"waiting and stopJob false => waiting": {
			stopJob:        pointers.Ptr(false),
			jobPhase:       radixv1.BatchJobPhaseWaiting,
			expectedStatus: v1.JobStatusEnumWaiting,
		},
		"waiting and stopJob true => stopping": {
			stopJob:        pointers.Ptr(true),
			jobPhase:       radixv1.BatchJobPhaseWaiting,
			expectedStatus: v1.JobStatusEnumStopping,
		},
		"active and stopJob not set => waiting": {
			stopJob:        nil,
			jobPhase:       radixv1.BatchJobPhaseActive,
			expectedStatus: v1.JobStatusEnumActive,
		},
		"active and stopJob false => waiting": {
			stopJob:        pointers.Ptr(false),
			jobPhase:       radixv1.BatchJobPhaseActive,
			expectedStatus: v1.JobStatusEnumActive,
		},
		"active and stopJob true => stopping": {
			stopJob:        pointers.Ptr(true),
			jobPhase:       radixv1.BatchJobPhaseActive,
			expectedStatus: v1.JobStatusEnumStopping,
		},
		"running and stopJob true => stopping": {
			stopJob:        pointers.Ptr(true),
			jobPhase:       radixv1.BatchJobPhaseRunning,
			expectedStatus: v1.JobStatusEnumStopping,
		},
		"stopped and stopJob true => stopped": {
			stopJob:        pointers.Ptr(true),
			jobPhase:       radixv1.BatchJobPhaseStopped,
			expectedStatus: v1.JobStatusEnumStopped,
		},
		"succeeded and stopJob true => succeeded": {
			stopJob:        pointers.Ptr(true),
			jobPhase:       radixv1.BatchJobPhaseSucceeded,
			expectedStatus: v1.JobStatusEnumSucceeded,
		},
		"failed and stopJob true => failed": {
			stopJob:        pointers.Ptr(true),
			jobPhase:       radixv1.BatchJobPhaseFailed,
			expectedStatus: v1.JobStatusEnumFailed,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			rb := radixv1.RadixBatch{
				Spec: radixv1.RadixBatchSpec{
					Jobs: []radixv1.RadixBatchJob{
						{Name: "job", Stop: test.stopJob},
					},
				},
				Status: radixv1.RadixBatchStatus{
					JobStatuses: []radixv1.RadixBatchJobStatus{
						{Name: "job", Phase: test.jobPhase},
					},
				},
			}
			actual := v1.BuildBatchStatus(rb, nil, nil)
			assert.Equal(t, test.expectedStatus, actual.JobStatuses[0].Status)
		})
	}
}

func Test_BuildBatchStatus_JobStatus_PodStatusProps(t *testing.T) {
	anyTime := time.Now()
	tests := map[string]struct {
		podStatus radixv1.RadixBatchJobPodStatus
		expected  v1.PodStatus
	}{
		"all non-nil pod props set": {
			podStatus: radixv1.RadixBatchJobPodStatus{
				Name:         "pod1",
				Phase:        "anyphase",
				Message:      "anymessage",
				RestartCount: 4,
				Image:        "anyimage",
				ImageID:      "anyimageid",
				PodIndex:     3,
				ExitCode:     42,
				Reason:       "anyreason",
			},
			expected: v1.PodStatus{
				Name:          "pod1",
				Status:        v1.ReplicaStatus{Status: "anyphase"},
				StatusMessage: "anymessage",
				RestartCount:  4,
				Image:         "anyimage",
				ImageId:       "anyimageid",
				PodIndex:      3,
				ExitCode:      42,
				Reason:        "anyreason",
			},
		},
		"Created set from CreationTime": {
			podStatus: radixv1.RadixBatchJobPodStatus{CreationTime: pointers.Ptr(metav1.NewTime(anyTime))},
			expected:  v1.PodStatus{Created: &anyTime},
		},
		"StartTime and ContainerStarted set from StartTime": {
			podStatus: radixv1.RadixBatchJobPodStatus{StartTime: pointers.Ptr(metav1.NewTime(anyTime))},
			expected:  v1.PodStatus{StartTime: &anyTime, ContainerStarted: &anyTime},
		},
		"EndTime set from EndTime": {
			podStatus: radixv1.RadixBatchJobPodStatus{EndTime: pointers.Ptr(metav1.NewTime(anyTime))},
			expected:  v1.PodStatus{EndTime: &anyTime},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			rb := radixv1.RadixBatch{
				Spec: radixv1.RadixBatchSpec{Jobs: []radixv1.RadixBatchJob{{Name: "job1"}}},
				Status: radixv1.RadixBatchStatus{JobStatuses: []radixv1.RadixBatchJobStatus{{
					Name:                     "job1",
					RadixBatchJobPodStatuses: []radixv1.RadixBatchJobPodStatus{test.podStatus},
				}}},
			}
			actual := v1.BuildBatchStatus(rb, nil, nil)
			require.Len(t, actual.JobStatuses, 1)
			require.Len(t, actual.JobStatuses[0].PodStatuses, 1)
			assert.Equal(t, test.expected, actual.JobStatuses[0].PodStatuses[0])
		})
	}
}
