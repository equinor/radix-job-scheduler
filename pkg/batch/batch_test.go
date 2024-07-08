package batch

import (
	"context"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	testUtil "github.com/equinor/radix-job-scheduler/internal/test"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testProps struct {
	appName               string
	envName               string
	radixJobComponentName string
	batchName             string
}

type jobStatusPhase map[string]radixv1.RadixBatchJobPhase
type testArgs struct {
	radixBatch          *radixv1.RadixBatch
	batchRadixDeploy    operatorUtils.DeploymentBuilder
	activeRadixDeploy   *operatorUtils.DeploymentBuilder
	expectedBatchStatus radixv1.RadixBatchJobApiStatus
}

const (
	batchName1           = "batch1"
	jobName1             = "job1"
	jobName2             = "job2"
	jobName3             = "job3"
	jobName4             = "job4"
	radixDeploymentName1 = "any-deployment1"
	radixDeploymentName2 = "any-deployment2"
)

var (
	now       = time.Now()
	yesterday = now.Add(time.Hour * -20)
	props     = testProps{
		appName:               "any-app",
		envName:               "any-env",
		radixJobComponentName: "any-job",
		batchName:             batchName1,
	}
)

func TestCopyRadixBatchOrJob(t *testing.T) {
	tests := []struct {
		name    string
		args    testArgs
		want    *modelsv2.RadixBatch
		wantErr bool
	}{
		{
			name: "only deployment has no rules, no job statuses",
			args: testArgs{
				radixBatch: createRadixBatch(props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, nil),
				batchRadixDeploy:    createRadixDeployJobComponent(radixDeploymentName1, props),
				expectedBatchStatus: radixv1.RadixBatchJobApiStatusWaiting,
			},
		},
		{
			name: "only deployment has no rules, job statuses waiting, active",
			args: testArgs{
				radixBatch: createRadixBatch(props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseWaiting, jobName2: radixv1.BatchJobPhaseActive}),
				batchRadixDeploy:    createRadixDeployJobComponent(radixDeploymentName1, props),
				expectedBatchStatus: radixv1.RadixBatchJobApiStatusWaiting,
			},
		},
		{
			name: "only deployment has no rules, job statuses failed, succeeded",
			args: testArgs{
				radixBatch: createRadixBatch(props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseFailed, jobName2: radixv1.BatchJobPhaseSucceeded}),
				batchRadixDeploy:    createRadixDeployJobComponent(radixDeploymentName1, props),
				expectedBatchStatus: radixv1.RadixBatchJobApiStatusWaiting,
			},
		},
		{
			name: "only deployment, with rules, job statuses failed, succeeded",
			args: testArgs{
				radixBatch: createRadixBatch(props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseFailed, jobName2: radixv1.BatchJobPhaseSucceeded}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseFailed)),
				expectedBatchStatus: radixv1.RadixBatchJobApiStatusWaiting,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			radixClient, _, _, _ := testUtil.SetupTest(props.appName, props.envName, props.radixJobComponentName, radixDeploymentName1, 1)
			tt.args.batchRadixDeploy.WithActiveFrom(yesterday)
			var activeRadixDeployment *radixv1.RadixDeployment
			if tt.args.activeRadixDeploy != nil {
				tt.args.batchRadixDeploy.WithActiveTo(now)
				tt.args.batchRadixDeploy.WithCondition(radixv1.DeploymentInactive)
				activeRadixDeployment = (*tt.args.activeRadixDeploy).WithActiveFrom(now).WithCondition(radixv1.DeploymentActive).BuildRD()
				_, err := radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(props.appName, props.envName)).
					Create(context.Background(), activeRadixDeployment, metav1.CreateOptions{})
				require.NoError(t, err)
			} else {
				tt.args.batchRadixDeploy.WithCondition(radixv1.DeploymentActive)
			}
			batchRadixDeploy, err := radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(props.appName, props.envName)).
				Create(context.Background(), tt.args.batchRadixDeploy.BuildRD(), metav1.CreateOptions{})
			require.NoError(t, err)
			if activeRadixDeployment == nil {
				activeRadixDeployment = batchRadixDeploy
			}
			radixDeployJobComponent, ok := slice.FindFirst(activeRadixDeployment.Spec.Jobs, func(component radixv1.RadixDeployJobComponent) bool {
				return component.Name == props.radixJobComponentName
			})
			require.True(t, ok)

			createdRadixBatchStatus, err := CopyRadixBatchOrJob(context.Background(), radixClient, tt.args.radixBatch, "", &radixDeployJobComponent, radixDeploymentName1)
			if (err != nil) != tt.wantErr {
				t.Errorf("CopyRadixBatchOrJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NotNil(t, createdRadixBatchStatus, "Status is nil")
			assert.Equal(t, tt.args.expectedBatchStatus, createdRadixBatchStatus.Status, "Status is not as expected")
		})
	}
}

func TestGetRadixBatchStatus(t *testing.T) {
	tests := []struct {
		name    string
		args    testArgs
		want    *modelsv2.RadixBatch
		wantErr bool
	}{
		{
			name: "only deployment has no rules, no job statuses",
			args: testArgs{
				radixBatch: createRadixBatch(props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeWaiting, nil),
				batchRadixDeploy:    createRadixDeployJobComponent(radixDeploymentName1, props),
				expectedBatchStatus: radixv1.RadixBatchJobApiStatusWaiting,
			},
		},
		{
			name: "only deployment has no rules, job statuses waiting, active",
			args: testArgs{
				radixBatch: createRadixBatch(props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseWaiting, jobName2: radixv1.BatchJobPhaseActive}),
				batchRadixDeploy:    createRadixDeployJobComponent(radixDeploymentName1, props),
				expectedBatchStatus: radixv1.RadixBatchJobApiStatusActive,
			},
		},
		{
			name: "only deployment has no rules, job statuses failed, succeeded",
			args: testArgs{
				radixBatch: createRadixBatch(props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseFailed, jobName2: radixv1.BatchJobPhaseSucceeded}),
				batchRadixDeploy:    createRadixDeployJobComponent(radixDeploymentName1, props),
				expectedBatchStatus: radixv1.RadixBatchJobApiStatusActive,
			},
		},
		{
			name: "only deployment, with only rule does not match, job statuses failed, succeeded",
			args: testArgs{
				radixBatch: createRadixBatch(props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseWaiting, jobName2: radixv1.BatchJobPhaseSucceeded}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseFailed)),
				expectedBatchStatus: radixv1.RadixBatchJobApiStatusActive,
			},
		},
		{
			name: "only deployment, second rule matches",
			args: testArgs{
				radixBatch: createRadixBatch(props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseStopped, jobName2: radixv1.BatchJobPhaseSucceeded}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseFailed),
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusSucceeded, radixv1.ConditionAll, radixv1.OperatorIn, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseStopped)),
				expectedBatchStatus: radixv1.RadixBatchJobApiStatusSucceeded,
			},
		},
		{
			name: "only deployment, with only rule any in matches, job statuses failed, succeeded",
			args: testArgs{
				radixBatch: createRadixBatch(props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseFailed, jobName2: radixv1.BatchJobPhaseSucceeded}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseFailed)),
				expectedBatchStatus: radixv1.RadixBatchJobApiStatusFailed,
			},
		},
		{
			name: "only deployment, with rule all not-in matches",
			args: testArgs{
				radixBatch: createRadixBatch(props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2, jobName3},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseRunning, jobName2: radixv1.BatchJobPhaseActive}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusRunning, radixv1.ConditionAll, radixv1.OperatorNotIn, radixv1.BatchJobPhaseWaiting, radixv1.BatchJobPhaseStopped, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseFailed),
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusWaiting, radixv1.ConditionAll, radixv1.OperatorNotIn, radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning, radixv1.BatchJobPhaseStopped, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseFailed),
				),
				expectedBatchStatus: radixv1.RadixBatchJobApiStatusRunning,
			},
		},
		{
			name: "only deployment, with second rule all not-in matches",
			args: testArgs{
				radixBatch: createRadixBatch(props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2, jobName3},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseRunning, jobName2: radixv1.BatchJobPhaseActive}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusWaiting, radixv1.ConditionAll, radixv1.OperatorNotIn, radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning, radixv1.BatchJobPhaseStopped, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseFailed),
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusRunning, radixv1.ConditionAll, radixv1.OperatorNotIn, radixv1.BatchJobPhaseWaiting, radixv1.BatchJobPhaseStopped, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseFailed),
				),
				expectedBatchStatus: radixv1.RadixBatchJobApiStatusRunning,
			},
		},
		{
			name: "only deployment, with none of rules all not-in matches",
			args: testArgs{
				radixBatch: createRadixBatch(props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2, jobName3},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseRunning, jobName2: radixv1.BatchJobPhaseActive, jobName3: radixv1.BatchJobPhaseFailed}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusWaiting, radixv1.ConditionAll, radixv1.OperatorNotIn, radixv1.BatchJobPhaseActive, radixv1.BatchJobPhaseRunning, radixv1.BatchJobPhaseStopped, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseFailed),
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusRunning, radixv1.ConditionAll, radixv1.OperatorNotIn, radixv1.BatchJobPhaseWaiting, radixv1.BatchJobPhaseStopped, radixv1.BatchJobPhaseSucceeded, radixv1.BatchJobPhaseFailed),
				),
				expectedBatchStatus: radixv1.RadixBatchJobApiStatusActive,
			},
		},
		{
			name: "two deployments, with rule from active applied",
			args: testArgs{
				radixBatch: createRadixBatch(props, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2, jobName3},
					radixv1.BatchConditionTypeActive, jobStatusPhase{jobName1: radixv1.BatchJobPhaseRunning, jobName2: radixv1.BatchJobPhaseActive, jobName3: radixv1.BatchJobPhaseFailed, jobName4: radixv1.BatchJobPhaseSucceeded}),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseFailed)),
				activeRadixDeploy: pointers.Ptr(createRadixDeployJobComponent(radixDeploymentName2, props,
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusFailed, radixv1.ConditionAll, radixv1.OperatorIn, radixv1.BatchJobPhaseFailed),
					createBatchStatusRule(radixv1.RadixBatchJobApiStatusRunning, radixv1.ConditionAny, radixv1.OperatorIn, radixv1.BatchJobPhaseRunning))),
				expectedBatchStatus: radixv1.RadixBatchJobApiStatusRunning,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			radixClient, _, _, _ := testUtil.SetupTest(props.appName, props.envName, props.radixJobComponentName, radixDeploymentName1, 1)
			tt.args.batchRadixDeploy.WithActiveFrom(yesterday)
			var activeRadixDeployment *radixv1.RadixDeployment
			if tt.args.activeRadixDeploy != nil {
				tt.args.batchRadixDeploy.WithActiveTo(now)
				tt.args.batchRadixDeploy.WithCondition(radixv1.DeploymentInactive)
				activeRadixDeployment = (*tt.args.activeRadixDeploy).WithActiveFrom(now).WithCondition(radixv1.DeploymentActive).BuildRD()
				_, err := radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(props.appName, props.envName)).
					Create(context.Background(), activeRadixDeployment, metav1.CreateOptions{})
				require.NoError(t, err)
			} else {
				tt.args.batchRadixDeploy.WithCondition(radixv1.DeploymentActive)
			}
			batchRadixDeploy, err := radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(props.appName, props.envName)).
				Create(context.Background(), tt.args.batchRadixDeploy.BuildRD(), metav1.CreateOptions{})
			require.NoError(t, err)
			if activeRadixDeployment == nil {
				activeRadixDeployment = batchRadixDeploy
			}
			radixDeployJobComponent, ok := slice.FindFirst(activeRadixDeployment.Spec.Jobs, func(component radixv1.RadixDeployJobComponent) bool {
				return component.Name == props.radixJobComponentName
			})
			require.True(t, ok)

			actualBatchStatus := GetRadixBatchStatus(tt.args.radixBatch, &radixDeployJobComponent)
			assert.Equal(t, tt.args.expectedBatchStatus, actualBatchStatus.Status, "Status is not as expected")
		})
	}
}

func TestGetRadixBatchStatuses(t *testing.T) {
	// GetRadixBatchStatuses(tt.args.radixBatches, tt.args.radixDeployJobComponent)
}

func aRadixDeploymentWithComponentModifier(props testProps, radixDeploymentName string, m func(builder operatorUtils.DeployJobComponentBuilder) operatorUtils.DeployJobComponentBuilder) operatorUtils.DeploymentBuilder {
	builder := operatorUtils.NewDeploymentBuilder().
		WithAppName(props.appName).
		WithDeploymentName(radixDeploymentName).
		WithImageTag("imagetag").
		WithEnvironment(props.envName).
		WithJobComponent(m(operatorUtils.NewDeployJobComponentBuilder().
			WithName(props.radixJobComponentName).
			WithImage("radixdev.azurecr.io/job:imagetag").
			WithSchedulerPort(numbers.Int32Ptr(8080))))
	return builder
}

func createBatchStatusRule(batchStatus radixv1.RadixBatchJobApiStatus, condition radixv1.Condition, operator radixv1.Operator, jobPhases ...radixv1.RadixBatchJobPhase) radixv1.BatchStatusRule {
	return radixv1.BatchStatusRule{Condition: condition, BatchStatus: batchStatus, Operator: operator, JobStatuses: jobPhases}
}

func createRadixDeployJobComponent(radixDeploymentName string, props testProps, rules ...radixv1.BatchStatusRule) operatorUtils.DeploymentBuilder {
	return aRadixDeploymentWithComponentModifier(props, radixDeploymentName, func(builder operatorUtils.DeployJobComponentBuilder) operatorUtils.DeployJobComponentBuilder {
		return builder.WithBatchStatusRules(rules...)
	})
}

func createRadixBatch(props testProps, radixBatchType kube.RadixBatchType, radixDeploymentName string, jobNames []string, batchStatus radixv1.RadixBatchConditionType, jobStatuses jobStatusPhase) *radixv1.RadixBatch {
	radixBatch := radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name: props.batchName,
			Labels: radixLabels.Merge(
				radixLabels.ForApplicationName(props.appName),
				radixLabels.ForComponentName(props.radixJobComponentName),
				radixLabels.ForBatchType(radixBatchType),
			),
		},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: radixDeploymentName},
				Job:                  props.radixJobComponentName,
			},
		},
		Status: radixv1.RadixBatchStatus{
			Condition: radixv1.RadixBatchCondition{
				Type: batchStatus,
			},
		},
	}
	for _, jobName := range jobNames {
		radixBatch.Spec.Jobs = append(radixBatch.Spec.Jobs, radixv1.RadixBatchJob{
			Name: jobName,
		})
	}
	if jobStatuses != nil {
		for _, jobName := range jobNames {
			if jobPhase, ok := jobStatuses[jobName]; ok {
				radixBatch.Status.JobStatuses = append(radixBatch.Status.JobStatuses, radixv1.RadixBatchJobStatus{
					Name:  jobName,
					Phase: jobPhase,
				})
			}
		}
	}
	return &radixBatch
}
