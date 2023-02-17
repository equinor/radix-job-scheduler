package batchesv1

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	modelsEnv "github.com/equinor/radix-job-scheduler/models"
	models "github.com/equinor/radix-job-scheduler/models/common"
	testUtils "github.com/equinor/radix-job-scheduler/utils/test"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateBatch(t *testing.T) {
	radixClient, kubeClient, _, kubeUtil := testUtils.SetupTest("app", "qa", "compute", "app-deploy-1", 1)
	env := modelsEnv.NewEnv()

	h := New(env, kubeUtil, kubeClient, radixClient)

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
		scheduledBatch, err := radixClient.RadixV1().RadixBatches(rd.Namespace).Get(context.Background(), createdBatch.Name,
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

func TestStopBatchJob(t *testing.T) {
	t.Run("stop job - cleanup resources for job", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixBatch1 := testUtils.AddRadixBatch(radixClient, "test-batch1-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		radixBatch2 := testUtils.AddRadixBatch(radixClient, "test-batch2-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		testUtils.CreateSecretForTest(appName, radixBatch1.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch1-job1", appJobComponent, envNamespace, kubeClient)
		testUtils.CreateSecretForTest(appName, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch2-job1", appJobComponent, envNamespace, kubeClient)
		testUtils.CreateSecretForTest(appName, "secret3", "test-batch3-job1", appJobComponent, envNamespace, kubeClient)
		testUtils.CreateSecretForTest(appName, "secret4", "test-batch4-job1", "other-job-component", envNamespace, kubeClient)
		testUtils.CreateSecretForTest(appName, "secret5", "test-batch5-job1", appJobComponent, "other-ns", kubeClient)

		handler := New(modelsEnv.NewEnv(), kubeUtil, kubeClient, radixClient)
		err := handler.StopBatchJob("test-batch1", "test-batch1-job1")
		assert.Nil(t, err)
		radixBatchList, _ := radixClient.RadixV1().RadixBatches("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, radixBatchList.Items, 2)
		radixBatch1 = testUtils.GetRadixBatchByNameForTest(radixBatchList.Items, "test-batch1-job1")
		assert.NotNil(t, radixBatch1)
		assert.NotNil(t, testUtils.GetRadixBatchByNameForTest(radixBatchList.Items, "test-batch2-job1"))
		for _, jobStatus := range radixBatch1.Status.JobStatuses {
			assert.Equal(t, radixv1.BatchJobPhaseStopped, jobStatus.Phase)
		}
		for _, job := range radixBatch1.Spec.Jobs {
			assert.NotNil(t, job.Stop)
			assert.Equal(t, pointers.Ptr(true), job.Stop)
		}
		secrets, _ := kubeClient.CoreV1().Secrets("").List(context.Background(), metav1.ListOptions{})
		assert.Len(t, secrets.Items, 5)
		assert.NotNil(t, testUtils.GetSecretByNameForTest(secrets.Items, radixBatch1.Spec.Jobs[0].PayloadSecretRef.Name))
		assert.NotNil(t, testUtils.GetSecretByNameForTest(secrets.Items, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name))
		assert.NotNil(t, testUtils.GetSecretByNameForTest(secrets.Items, "secret3"))
		assert.NotNil(t, testUtils.GetSecretByNameForTest(secrets.Items, "secret4"))
		assert.NotNil(t, testUtils.GetSecretByNameForTest(secrets.Items, "secret5"))
	})

	t.Run("stop job - job name does not exist", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		testUtils.AddRadixBatch(radixClient, "test-batch2-job2", appJobComponent, kube.RadixBatchTypeJob, envNamespace)

		handler := New(modelsEnv.NewEnv(), kubeUtil, kubeClient, radixClient)
		err := handler.StopBatchJob("test-batch", "test-batch-job1")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("stop job - another job component name", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		testUtils.AddRadixBatch(radixClient, "test-batch2-job2", "another-job-component", kube.RadixBatchTypeJob, envNamespace)

		handler := New(modelsEnv.NewEnv(), kubeUtil, kubeClient, radixClient)
		err := handler.StopBatchJob("test-batch", "test-batch-job1")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("stop job - another job type", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		testUtils.AddRadixBatch(radixClient, "test-batch2-job2", appJobComponent, kube.RadixBatchTypeBatch, envNamespace)

		handler := New(modelsEnv.NewEnv(), kubeUtil, kubeClient, radixClient)
		err := handler.StopBatchJob("test-batch", "test-batch-job1")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("stop job - another namespace", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "test-batch-job1"
		radixClient, kubeClient, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		testUtils.AddRadixBatch(radixClient, jobName, appJobComponent, kube.RadixBatchTypeJob, "another-ns")

		handler := New(modelsEnv.NewEnv(), kubeUtil, kubeClient, radixClient)
		err := handler.StopBatchJob("test-batch", "test-batch-job1")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})
}
