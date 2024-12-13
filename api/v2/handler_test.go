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
	type testSpec map[string]struct {
		defaultRadixJobComponentConfig  *common.RadixJobComponentConfig
		jobScheduleDescription          *common.JobScheduleDescription
		expectedRadixJobComponentConfig *common.RadixJobComponentConfig
	}

	tests := testSpec{
		"Resources merged from job and default spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &common.Resources{
					Limits: common.ResourceList{
						"cpu":    "20m",
						"memory": "20M",
					},
					Requests: common.ResourceList{
						"cpu": "10m",
					},
				},
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Resources: &common.Resources{
						Limits: common.ResourceList{
							"memory": "21M",
						},
						Requests: common.ResourceList{
							"memory": "10M",
							"cpu":    "11m",
						},
					},
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &common.Resources{
					Limits: common.ResourceList{
						"cpu":    "20m",
						"memory": "21M",
					},
					Requests: common.ResourceList{
						"cpu":    "11m",
						"memory": "10M",
					},
				},
			},
		},
		"Resources from job spec only": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Resources: &common.Resources{
						Limits: common.ResourceList{
							"memory": "20M",
						},
						Requests: common.ResourceList{
							"memory": "10M",
						},
					},
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &common.Resources{
					Limits: common.ResourceList{
						"memory": "20M",
					},
					Requests: common.ResourceList{
						"memory": "10M",
					},
				},
			},
		},
		"Resources from default spec only": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &common.Resources{
					Limits: common.ResourceList{
						"memory": "20M",
					},
					Requests: common.ResourceList{
						"memory": "10M",
					},
				},
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Resources: &common.Resources{
					Limits: common.ResourceList{
						"memory": "20M",
					},
					Requests: common.ResourceList{
						"memory": "10M",
					},
				},
			},
		},
		"Node merged from job and default spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Node: &common.Node{
					GpuCount: "2",
					Gpu:      "gpu1,gpu2",
				},
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Node: &common.Node{
						Gpu: "gpu3",
					},
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Node: &common.Node{
					GpuCount: "2",
					Gpu:      "gpu3",
				},
			},
		},
		"Node from default spec only": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Node: &common.Node{
					GpuCount: "2",
					Gpu:      "gpu1,gpu2",
				},
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Node: &common.Node{
					GpuCount: "2",
					Gpu:      "gpu1,gpu2",
				},
			},
		},
		"Node from job spec only": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					Node: &common.Node{
						GpuCount: "2",
						Gpu:      "gpu3",
					},
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				Node: &common.Node{
					GpuCount: "2",
					Gpu:      "gpu3",
				},
			},
		},
		"BackoffLimit from job spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				BackoffLimit: pointers.Ptr[int32](2000),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					BackoffLimit: pointers.Ptr[int32](1000),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				BackoffLimit: pointers.Ptr[int32](1000),
			},
		},
		"BackoffLimit from default spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				BackoffLimit: pointers.Ptr[int32](2000),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				BackoffLimit: pointers.Ptr[int32](2000),
			},
		},
		"TimeLimitSeconds from job spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				TimeLimitSeconds: pointers.Ptr[int64](2000),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					TimeLimitSeconds: pointers.Ptr[int64](1000),
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				TimeLimitSeconds: pointers.Ptr[int64](1000),
			},
		},
		"TimeLimitSeconds from default spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				TimeLimitSeconds: pointers.Ptr[int64](2000),
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				TimeLimitSeconds: pointers.Ptr[int64](2000),
			},
		},
		"FailurePolicy from job spec only": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				FailurePolicy: &common.FailurePolicy{
					Rules: []common.FailurePolicyRule{
						{
							Action: common.FailurePolicyRuleActionCount,
							OnExitCodes: common.FailurePolicyRuleOnExitCodes{
								Operator: common.FailurePolicyRuleOnExitCodesOpIn,
								Values:   []int32{1, 2, 3},
							},
						},
					},
				},
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{
					FailurePolicy: &common.FailurePolicy{
						Rules: []common.FailurePolicyRule{
							{
								Action: common.FailurePolicyRuleActionFailJob,
								OnExitCodes: common.FailurePolicyRuleOnExitCodes{
									Operator: common.FailurePolicyRuleOnExitCodesOpNotIn,
									Values:   []int32{0, 1},
								},
							},
						},
					},
				},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				FailurePolicy: &common.FailurePolicy{
					Rules: []common.FailurePolicyRule{
						{
							Action: common.FailurePolicyRuleActionFailJob,
							OnExitCodes: common.FailurePolicyRuleOnExitCodes{
								Operator: common.FailurePolicyRuleOnExitCodesOpNotIn,
								Values:   []int32{0, 1},
							},
						},
					},
				},
			},
		},
		"FailurePolicy from default spec": {
			defaultRadixJobComponentConfig: &common.RadixJobComponentConfig{
				FailurePolicy: &common.FailurePolicy{
					Rules: []common.FailurePolicyRule{
						{
							Action: common.FailurePolicyRuleActionCount,
							OnExitCodes: common.FailurePolicyRuleOnExitCodes{
								Operator: common.FailurePolicyRuleOnExitCodesOpIn,
								Values:   []int32{1, 2, 3},
							},
						},
					},
				},
			},
			jobScheduleDescription: &common.JobScheduleDescription{
				RadixJobComponentConfig: common.RadixJobComponentConfig{},
			},
			expectedRadixJobComponentConfig: &common.RadixJobComponentConfig{
				FailurePolicy: &common.FailurePolicy{
					Rules: []common.FailurePolicyRule{
						{
							Action: common.FailurePolicyRuleActionCount,
							OnExitCodes: common.FailurePolicyRuleOnExitCodes{
								Operator: common.FailurePolicyRuleOnExitCodesOpIn,
								Values:   []int32{1, 2, 3},
							},
						},
					},
				},
			},
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			err := applyDefaultJobDescriptionProperties(test.jobScheduleDescription, test.defaultRadixJobComponentConfig)
			require.NoError(t, err)
			assert.EqualValues(t, *test.expectedRadixJobComponentConfig, test.jobScheduleDescription.RadixJobComponentConfig)
		})
	}
}
