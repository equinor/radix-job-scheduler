package batchesv1

import (
	"context"
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"testing"

	"github.com/equinor/radix-operator/pkg/apis/kube"

	api "github.com/equinor/radix-job-scheduler/api/v1"
	modelsEnv "github.com/equinor/radix-job-scheduler/models"
	models "github.com/equinor/radix-job-scheduler/models/common"
	testUtils "github.com/equinor/radix-job-scheduler/utils/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_createBatch(t *testing.T) {
	radixClient, kubeClient, _, kubeUtil := testUtils.SetupTest("app", "qa", "compute", "app-deploy-1", 1)
	env := modelsEnv.NewEnv()

	h := &batchHandler{
		common: &api.Handler{
			Kube:         kubeUtil,
			KubeClient:   kubeClient,
			RadixClient:  radixClient,
			Env:          env,
			HandlerApiV2: apiv2.New(env, kubeUtil, kubeClient, radixClient),
		},
	}

	t.Run("Create Batch", func(t *testing.T) {
		t.Parallel()
		params := testUtils.GetTestParams()
		rd := params.ApplyRd(kubeUtil)
		assert.NotNil(t, rd)

		scheduleDescription := models.BatchScheduleDescription{JobScheduleDescriptions: []models.JobScheduleDescription{
			{
				JobId:                   "job1",
				Payload:                 "{}",
				RadixJobComponentConfig: models.RadixJobComponentConfig{},
			},
			{
				JobId:                   "job2",
				Payload:                 "{}",
				RadixJobComponentConfig: models.RadixJobComponentConfig{},
			},
		}}
		createdBatch, err := h.CreateBatch(&scheduleDescription)

		assert.NoError(t, err)
		scheduledBatch, err := h.common.RadixClient.RadixV1().RadixBatches(rd.Namespace).Get(context.Background(), createdBatch.Name,
			metav1.GetOptions{})
		assert.Nil(t, err)
		assert.NotNil(t, scheduledBatch)
		assert.Equal(t, createdBatch.Name, scheduledBatch.Name)
		assert.Equal(t, len(scheduleDescription.JobScheduleDescriptions),
			len(scheduledBatch.Spec.Jobs))
		assert.Equal(t, params.JobComponentName,
			scheduledBatch.ObjectMeta.Labels[kube.RadixComponentLabel])
		assert.Equal(t, params.AppName,
			scheduledBatch.ObjectMeta.Labels[kube.RadixAppLabel])
		assert.Equal(t, string(kube.RadixBatchTypeBatch),
			scheduledBatch.ObjectMeta.Labels[kube.RadixBatchTypeLabel])
	})
}
