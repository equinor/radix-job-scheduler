package batch

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-common/utils/slice"
	testUtil "github.com/equinor/radix-job-scheduler/internal/test"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testProps struct {
	appName               string
	envName               string
	radixJobComponentName string
}

func TestCopyRadixBatchOrJob(t *testing.T) {
	props := testProps{
		appName:               "any-app",
		envName:               "any-env",
		radixJobComponentName: "any-job",
	}
	const (
		batchName1           = "batch1"
		jobName1             = "job1"
		jobName2             = "job2"
		radixDeploymentName1 = "any-deployment1"
		radixDeploymentName2 = "any-deployment2"
	)
	var (
		now       = time.Now()
		yesterday = now.Add(time.Hour * -20)
	)
	type args struct {
		sourceRadixBatch  *radixv1.RadixBatch
		sourceJobName     string
		batchRadixDeploy  operatorUtils.DeploymentBuilder
		activeRadixDeploy *operatorUtils.DeploymentBuilder
	}
	tests := []struct {
		name    string
		args    args
		want    *modelsv2.RadixBatch
		wantErr bool
	}{
		{
			name: "Copy batch with no rules",
			args: args{
				sourceRadixBatch: createRadixBatch(props, batchName1, kube.RadixBatchTypeBatch, radixDeploymentName1, []string{jobName1, jobName2}, nil),
				batchRadixDeploy: createRadixDeployJobComponent(radixDeploymentName1, props),
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

			createdRadixBatchStatus, err := CopyRadixBatchOrJob(context.Background(), radixClient, tt.args.sourceRadixBatch, tt.args.sourceJobName, &radixDeployJobComponent, radixDeploymentName1)
			if (err != nil) != tt.wantErr {
				t.Errorf("CopyRadixBatchOrJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.NotNil(t, createdRadixBatchStatus, "Status is nil")
		})
	}
}

func createRadixDeployJobComponent(radixDeploymentName string, props testProps, rules ...radixv1.BatchStatusRule) operatorUtils.DeploymentBuilder {
	return aRadixDeploymentWithComponentModifier(props, radixDeploymentName, func(builder operatorUtils.DeployJobComponentBuilder) operatorUtils.DeployJobComponentBuilder {
		return builder.WithBatchStatusRules(rules...)
	})
}

func createRadixBatch(props testProps, batchName string, radixBatchType kube.RadixBatchType, radixDeploymentName string, jobNames []string, jobStatuses map[string]radixv1.RadixBatchJobPhase) *radixv1.RadixBatch {
	radixBatch := radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name: batchName,
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

func TestDeleteRadixBatch(t *testing.T) {
	type args struct {
		ctx         context.Context
		radixClient versioned.Interface
		radixBatch  *radixv1.RadixBatch
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DeleteRadixBatch(tt.args.ctx, tt.args.radixClient, tt.args.radixBatch); (err != nil) != tt.wantErr {
				t.Errorf("DeleteRadixBatch() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetRadixBatchStatus(t *testing.T) {
	type args struct {
		radixBatch              *radixv1.RadixBatch
		radixDeployJobComponent *radixv1.RadixDeployJobComponent
	}
	tests := []struct {
		name string
		args args
		want modelsv2.RadixBatch
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetRadixBatchStatus(tt.args.radixBatch, tt.args.radixDeployJobComponent); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetRadixBatchStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetRadixBatchStatuses(t *testing.T) {
	type args struct {
		radixBatches            []*radixv1.RadixBatch
		radixDeployJobComponent *radixv1.RadixDeployJobComponent
	}
	tests := []struct {
		name string
		args args
		want []modelsv2.RadixBatch
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetRadixBatchStatuses(tt.args.radixBatches, tt.args.radixDeployJobComponent); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetRadixBatchStatuses() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRestartRadixBatch(t *testing.T) {
	type args struct {
		ctx         context.Context
		radixClient versioned.Interface
		radixBatch  *radixv1.RadixBatch
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RestartRadixBatch(tt.args.ctx, tt.args.radixClient, tt.args.radixBatch); (err != nil) != tt.wantErr {
				t.Errorf("RestartRadixBatch() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRestartRadixBatchJob(t *testing.T) {
	type args struct {
		ctx         context.Context
		radixClient versioned.Interface
		radixBatch  *radixv1.RadixBatch
		jobName     string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RestartRadixBatchJob(tt.args.ctx, tt.args.radixClient, tt.args.radixBatch, tt.args.jobName); (err != nil) != tt.wantErr {
				t.Errorf("RestartRadixBatchJob() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStopRadixBatch(t *testing.T) {
	type args struct {
		ctx         context.Context
		radixClient versioned.Interface
		radixBatch  *radixv1.RadixBatch
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := StopRadixBatch(tt.args.ctx, tt.args.radixClient, tt.args.radixBatch); (err != nil) != tt.wantErr {
				t.Errorf("StopRadixBatch() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStopRadixBatchJob(t *testing.T) {
	type args struct {
		ctx         context.Context
		radixClient versioned.Interface
		radixBatch  *radixv1.RadixBatch
		jobName     string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := StopRadixBatchJob(tt.args.ctx, tt.args.radixClient, tt.args.radixBatch, tt.args.jobName); (err != nil) != tt.wantErr {
				t.Errorf("StopRadixBatchJob() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_copyBatchJobs(t *testing.T) {
	type args struct {
		ctx              context.Context
		sourceRadixBatch *radixv1.RadixBatch
		sourceJobName    string
	}
	tests := []struct {
		name string
		args args
		want []radixv1.RadixBatchJob
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := copyBatchJobs(tt.args.ctx, tt.args.sourceRadixBatch, tt.args.sourceJobName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("copyBatchJobs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPodStatusByRadixBatchJobPodStatus(t *testing.T) {
	type args struct {
		podStatuses []radixv1.RadixBatchJobPodStatus
	}
	tests := []struct {
		name string
		args args
		want []modelsv2.RadixBatchJobPodStatus
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPodStatusByRadixBatchJobPodStatus(tt.args.podStatuses); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPodStatusByRadixBatchJobPodStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getRadixBatchJobStatusesFromRadixBatch(t *testing.T) {
	type args struct {
		radixBatch            *radixv1.RadixBatch
		radixBatchJobStatuses []radixv1.RadixBatchJobStatus
	}
	tests := []struct {
		name string
		args args
		want []modelsv2.RadixBatchJobStatus
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getRadixBatchJobStatusesFromRadixBatch(tt.args.radixBatch, tt.args.radixBatchJobStatuses); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getRadixBatchJobStatusesFromRadixBatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isBatchStoppable(t *testing.T) {
	type args struct {
		condition radixv1.RadixBatchCondition
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBatchStoppable(tt.args.condition); got != tt.want {
				t.Errorf("isBatchStoppable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_setRestartJobTimeout(t *testing.T) {
	type args struct {
		batch            *radixv1.RadixBatch
		jobIdx           int
		restartTimestamp string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRestartJobTimeout(tt.args.batch, tt.args.jobIdx, tt.args.restartTimestamp)
		})
	}
}

func Test_updateRadixBatch(t *testing.T) {
	type args struct {
		ctx         context.Context
		radixClient versioned.Interface
		namespace   string
		radixBatch  *radixv1.RadixBatch
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := updateRadixBatch(tt.args.ctx, tt.args.radixClient, tt.args.namespace, tt.args.radixBatch); (err != nil) != tt.wantErr {
				t.Errorf("updateRadixBatch() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
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

func aRadixDeployment(props testProps, radixDeploymentName string) operatorUtils.DeploymentBuilder {
	return aRadixDeploymentWithComponentModifier(props, radixDeploymentName, func(builder operatorUtils.DeployJobComponentBuilder) operatorUtils.DeployJobComponentBuilder { return builder })
}
