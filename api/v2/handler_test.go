package apiv2

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-job-scheduler/api/test"
	testUtil "github.com/equinor/radix-job-scheduler/internal/test"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_createBatch(t *testing.T) {
	type scenario struct {
		name              string
		batchDescription  common.BatchScheduleDescription
		expectedBatchType kube.RadixBatchType
		expectedError     bool
	}
	scenarios := []scenario{
		{
			name: "batch with multiple jobs",
			batchDescription: common.BatchScheduleDescription{JobScheduleDescriptions: []common.JobScheduleDescription{
				{
					JobId:                   "job1",
					Payload:                 "{}",
					RadixJobComponentConfig: common.RadixJobComponentConfig{},
				},
				{
					JobId:                   "job2",
					Payload:                 "{}",
					RadixJobComponentConfig: common.RadixJobComponentConfig{},
				},
			}},
			expectedBatchType: kube.RadixBatchTypeBatch,
			expectedError:     false,
		},
		{
			name: "batch with one job",
			batchDescription: common.BatchScheduleDescription{JobScheduleDescriptions: []common.JobScheduleDescription{
				{
					JobId:                   "job1",
					Payload:                 "{}",
					RadixJobComponentConfig: common.RadixJobComponentConfig{},
				},
			}},
			expectedBatchType: kube.RadixBatchTypeBatch,
			expectedError:     false,
		},
		{
			name:              "batch with no job failed",
			batchDescription:  common.BatchScheduleDescription{JobScheduleDescriptions: []common.JobScheduleDescription{}},
			expectedBatchType: kube.RadixBatchTypeBatch,
			expectedError:     true,
		},
		{
			name: "single job",
			batchDescription: common.BatchScheduleDescription{JobScheduleDescriptions: []common.JobScheduleDescription{
				{
					JobId:                   "job1",
					Payload:                 "{}",
					RadixJobComponentConfig: common.RadixJobComponentConfig{},
				},
			}},
			expectedBatchType: kube.RadixBatchTypeJob,
			expectedError:     false,
		},
		{
			name:              "single job with no job failed",
			batchDescription:  common.BatchScheduleDescription{JobScheduleDescriptions: []common.JobScheduleDescription{}},
			expectedBatchType: kube.RadixBatchTypeJob,
			expectedError:     true,
		},
	}

	for _, ts := range scenarios {
		appJobComponent := "compute"
		radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()
		_, _, _, kubeUtil := testUtil.SetupTest("app", "qa", appJobComponent, "app-deploy-1", 1)
		env := models.NewEnv()

		h := &handler{
			kubeUtil:                kubeUtil,
			env:                     env,
			radixDeployJobComponent: &radixDeployJobComponent,
		}
		t.Run(ts.name, func(t *testing.T) {
			// t.Parallel()
			params := test.GetTestParams()
			rd := params.ApplyRd(kubeUtil)
			assert.NotNil(t, rd)

			var err error
			var createdRadixBatch *modelsv2.RadixBatch
			if ts.expectedBatchType == kube.RadixBatchTypeBatch {
				createdRadixBatch, err = h.CreateRadixBatch(context.TODO(), &ts.batchDescription)
			} else {
				var jobScheduleDescription *common.JobScheduleDescription
				if len(ts.batchDescription.JobScheduleDescriptions) > 0 {
					jobScheduleDescription = &ts.batchDescription.JobScheduleDescriptions[0]
				}
				createdRadixBatch, err = h.CreateRadixBatchSingleJob(context.TODO(), jobScheduleDescription)
			}
			if ts.expectedError {
				assert.NotNil(t, err)
				return
			}
			assert.Nil(t, err)
			assert.NotNil(t, createdRadixBatch)

			scheduledBatchList, err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(rd.Namespace).List(context.TODO(),
				metav1.ListOptions{})
			assert.Nil(t, err)

			assert.Len(t, scheduledBatchList.Items, 1)
			scheduledBatch := scheduledBatchList.Items[0]
			assert.Equal(t, params.JobComponentName,
				scheduledBatch.ObjectMeta.Labels[kube.RadixComponentLabel])
			assert.Equal(t, params.AppName,
				scheduledBatch.ObjectMeta.Labels[kube.RadixAppLabel])
			assert.Equal(t, len(ts.batchDescription.JobScheduleDescriptions), len(scheduledBatch.Spec.Jobs))
			assert.Equal(t, string(ts.expectedBatchType),
				scheduledBatch.ObjectMeta.Labels[kube.RadixBatchTypeLabel])
		})
	}
}

func TestMergeJobDescriptionWithDefaultJobDescription(t *testing.T) {
	type scenario struct {
		name                            string
		defaultRadixJobComponentConfig  *common.RadixJobComponentConfig
		jobScheduleDescription          *common.JobScheduleDescription
		expectedRadixJobComponentConfig *common.RadixJobComponentConfig
	}

	scenarios := []scenario{
		{
			name:                           "Only job description",
			defaultRadixJobComponentConfig: nil,
			jobScheduleDescription: &common.JobScheduleDescription{
				JobId:   "job1",
				Payload: "{'n':'v'}",
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Resources: &radixv1.ResourceRequirements{
						Limits:   radixv1.ResourceList{"cpu": "100m", "memory": "1000Mi"},
						Requests: radixv1.ResourceList{"cpu": "200m", "memory": "2000Mi"},
					},
					Node:             &radixv1.RadixNode{Gpu: "v100", GpuCount: "1"},
					TimeLimitSeconds: pointers.Ptr(int64(1000)),
					BackoffLimit:     pointers.Ptr(int32(1)),
					ImageTagName:     "job1-tag",
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &radixv1.ResourceRequirements{
					Limits:   radixv1.ResourceList{"cpu": "100m", "memory": "1000Mi"},
					Requests: radixv1.ResourceList{"cpu": "200m", "memory": "2000Mi"},
				},
				Node:             &radixv1.RadixNode{Gpu: "v100", GpuCount: "1"},
				TimeLimitSeconds: pointers.Ptr(int64(1000)),
				BackoffLimit:     pointers.Ptr(int32(1)),
				ImageTagName:     "job1-tag",
			},
		},
		{
			name: "Only default job description",
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &radixv1.ResourceRequirements{
					Limits:   radixv1.ResourceList{"cpu": "100m", "memory": "1000Mi"},
					Requests: radixv1.ResourceList{"cpu": "200m", "memory": "2000Mi"},
				},
				Node:             &radixv1.RadixNode{Gpu: "v100", GpuCount: "1"},
				TimeLimitSeconds: pointers.Ptr(int64(1000)),
				BackoffLimit:     pointers.Ptr(int32(1)),
				ImageTagName:     "default-tag",
			},
			jobScheduleDescription: &common.JobScheduleDescription{},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &radixv1.ResourceRequirements{
					Limits:   radixv1.ResourceList{"cpu": "100m", "memory": "1000Mi"},
					Requests: radixv1.ResourceList{"cpu": "200m", "memory": "2000Mi"},
				},
				Node:             &radixv1.RadixNode{Gpu: "v100", GpuCount: "1"},
				TimeLimitSeconds: pointers.Ptr(int64(1000)),
				BackoffLimit:     pointers.Ptr(int32(1)),
				ImageTagName:     "default-tag",
			},
		},
		{
			name: "Added values to job description",
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &radixv1.ResourceRequirements{
					Limits:   radixv1.ResourceList{"cpu": "100m"},
					Requests: radixv1.ResourceList{"memory": "2000Mi"},
				},
				Node:             &radixv1.RadixNode{GpuCount: "1"},
				TimeLimitSeconds: pointers.Ptr(int64(1000)),
				BackoffLimit:     nil,
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				JobId:   "job1",
				Payload: "{'n':'v'}",
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Resources: &radixv1.ResourceRequirements{
						Limits:   radixv1.ResourceList{"memory": "1000Mi"},
						Requests: radixv1.ResourceList{"cpu": "200m"},
					},
					Node:             &radixv1.RadixNode{Gpu: "v100"},
					TimeLimitSeconds: nil,
					BackoffLimit:     pointers.Ptr(int32(1)),
					ImageTagName:     "job1-tag",
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &radixv1.ResourceRequirements{
					Limits:   radixv1.ResourceList{"cpu": "100m", "memory": "1000Mi"},
					Requests: radixv1.ResourceList{"cpu": "200m", "memory": "2000Mi"},
				},
				Node:             &radixv1.RadixNode{Gpu: "v100", GpuCount: "1"},
				TimeLimitSeconds: pointers.Ptr(int64(1000)),
				BackoffLimit:     pointers.Ptr(int32(1)),
				ImageTagName:     "job1-tag",
			},
		},
		{
			name: "Not overwritten values in job description",
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &radixv1.ResourceRequirements{
					Limits:   radixv1.ResourceList{"cpu": "400m", "memory": "4000Mi"},
					Requests: radixv1.ResourceList{"cpu": "400m", "memory": "4000Mi"},
				},
				Node:             &radixv1.RadixNode{Gpu: "P100", GpuCount: "5"},
				TimeLimitSeconds: pointers.Ptr(int64(6000)),
				BackoffLimit:     pointers.Ptr(int32(3)),
				ImageTagName:     "default-tag",
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				JobId:   "job1",
				Payload: "{'n':'v'}",
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Resources: &radixv1.ResourceRequirements{
						Limits:   radixv1.ResourceList{"cpu": "100m", "memory": "1000Mi"},
						Requests: radixv1.ResourceList{"cpu": "200m", "memory": "2000Mi"},
					},
					Node:             &radixv1.RadixNode{Gpu: "v100", GpuCount: "1"},
					TimeLimitSeconds: pointers.Ptr(int64(1000)),
					BackoffLimit:     pointers.Ptr(int32(1)),
					ImageTagName:     "job1-tag",
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &radixv1.ResourceRequirements{
					Limits:   radixv1.ResourceList{"cpu": "100m", "memory": "1000Mi"},
					Requests: radixv1.ResourceList{"cpu": "200m", "memory": "2000Mi"},
				},
				Node:             &radixv1.RadixNode{Gpu: "v100", GpuCount: "1"},
				TimeLimitSeconds: pointers.Ptr(int64(1000)),
				BackoffLimit:     pointers.Ptr(int32(1)),
				ImageTagName:     "job1-tag",
			},
		},
		{
			name: "Added but not overwritten values in job description",
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &radixv1.ResourceRequirements{
					Limits:   radixv1.ResourceList{"cpu": "100m", "memory": "4000Mi"},
					Requests: radixv1.ResourceList{"cpu": "200m", "memory": "2000Mi"},
				},
				Node:             &radixv1.RadixNode{Gpu: "P100", GpuCount: "1"},
				TimeLimitSeconds: pointers.Ptr(int64(1000)),
				BackoffLimit:     pointers.Ptr(int32(3)),
				ImageTagName:     "default-tag",
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				JobId:   "job1",
				Payload: "{'n':'v'}",
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Resources: &radixv1.ResourceRequirements{
						Limits: radixv1.ResourceList{"memory": "1000Mi"},
					},
					Node:         &radixv1.RadixNode{Gpu: "v100"},
					BackoffLimit: pointers.Ptr(int32(1)),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &radixv1.ResourceRequirements{
					Limits:   radixv1.ResourceList{"cpu": "100m", "memory": "1000Mi"},
					Requests: radixv1.ResourceList{"cpu": "200m", "memory": "2000Mi"},
				},
				Node:             &radixv1.RadixNode{Gpu: "v100", GpuCount: "1"},
				TimeLimitSeconds: pointers.Ptr(int64(1000)),
				BackoffLimit:     pointers.Ptr(int32(1)),
				ImageTagName:     "default-tag",
			},
		},
	}
	for _, ts := range scenarios {
		t.Run(ts.name, func(t *testing.T) {
			err := applyDefaultJobDescriptionProperties(ts.jobScheduleDescription, ts.defaultRadixJobComponentConfig)
			require.NoError(t, err)
			assert.EqualValues(t, *ts.expectedRadixJobComponentConfig, ts.jobScheduleDescription.RadixJobComponentConfig)
		})
	}
}
