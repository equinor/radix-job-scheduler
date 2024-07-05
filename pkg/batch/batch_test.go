package batch

import (
	"context"
	"reflect"
	"testing"

	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
)

func TestCopyRadixBatchOrJob(t *testing.T) {
	type args struct {
		ctx                     context.Context
		radixClient             versioned.Interface
		sourceRadixBatch        *radixv1.RadixBatch
		sourceJobName           string
		radixDeployJobComponent *radixv1.RadixDeployJobComponent
		radixDeploymentName     string
	}
	tests := []struct {
		name    string
		args    args
		want    *modelsv2.RadixBatch
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CopyRadixBatchOrJob(tt.args.ctx, tt.args.radixClient, tt.args.sourceRadixBatch, tt.args.sourceJobName, tt.args.radixDeployJobComponent, tt.args.radixDeploymentName)
			if (err != nil) != tt.wantErr {
				t.Errorf("CopyRadixBatchOrJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CopyRadixBatchOrJob() got = %v, want %v", got, tt.want)
			}
		})
	}
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
