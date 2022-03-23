package batches

import (
	"context"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"strconv"
	"testing"

	"github.com/equinor/radix-job-scheduler/api"
	"github.com/equinor/radix-job-scheduler/defaults"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/utils/test"
	testUtils "github.com/equinor/radix-job-scheduler/utils/test"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_createBatch(t *testing.T) {
	radixClient, kubeClient, kubeUtil := test.SetupTest("app", "qa", "compute", "app-deploy-1", 1)
	env := models.NewEnv()

	h := &batchHandler{
		common: &api.Handler{
			Kube:                   kubeUtil,
			KubeClient:             kubeClient,
			RadixClient:            radixClient,
			Env:                    env,
			SecurityContextBuilder: deployment.NewSecurityContextBuilder(true),
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
		scheduledBatch, err := h.common.KubeClient.BatchV1().Jobs(rd.Namespace).Get(context.Background(), createdBatch.Name,
			metav1.GetOptions{})
		assert.Nil(t, err)
		assert.NotNil(t, scheduledBatch)
		assert.Equal(t, createdBatch.Name, scheduledBatch.Name)
		assert.Equal(t, strconv.Itoa(len(scheduleDescription.JobScheduleDescriptions)),
			scheduledBatch.ObjectMeta.Annotations[defaults.RadixBatchJobCountAnnotation])
		assert.Equal(t, params.JobComponentName,
			scheduledBatch.ObjectMeta.Labels[kube.RadixComponentLabel])
		assert.Equal(t, params.AppName,
			scheduledBatch.ObjectMeta.Labels[kube.RadixAppLabel])
		assert.Equal(t, kube.RadixJobTypeBatchSchedule,
			scheduledBatch.ObjectMeta.Labels[kube.RadixJobTypeLabel])
	})
}
