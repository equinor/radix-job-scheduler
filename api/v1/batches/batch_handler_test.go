package batchesv1

import (
	"context"
	"testing"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/internal/config"
	"github.com/equinor/radix-job-scheduler/internal/test"
	models "github.com/equinor/radix-job-scheduler/models/common"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateBatch(t *testing.T) {
	radixClient, kubeClient, _, kubeUtil := test.SetupTest("app", "qa", "compute", "app-deploy-1", 1)
	cfg := config.NewConfigFromEnv()

	t.Run("Create Batch", func(t *testing.T) {
		t.Parallel()
		params := test.GetTestParams()
		jobComponent := params.RadixDeployJobComponent.BuildJobComponent()
		h := New(kubeUtil, cfg, &jobComponent)
		rd := params.ApplyRd(kubeUtil)
		assert.NotNil(t, rd)

		scheduleDescription := models.BatchScheduleDescription{JobScheduleDescriptions: []models.JobScheduleDescription{
			{
				JobId:                   "job1",
				Payload:                 "{'name1':'value1'}",
				RadixJobComponentConfig: models.RadixJobComponentConfig{},
			},
			{
				JobId:                   "job2",
				Payload:                 "test payload data",
				RadixJobComponentConfig: models.RadixJobComponentConfig{},
			},
		}}
		createdBatch, err := h.CreateBatch(context.TODO(), &scheduleDescription)

		assert.NoError(t, err)
		scheduledBatch, err := radixClient.RadixV1().RadixBatches(rd.Namespace).Get(context.TODO(), createdBatch.Name,
			metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, createdBatch.Name, scheduledBatch.Name)
		assert.Equal(t, len(scheduleDescription.JobScheduleDescriptions),
			len(scheduledBatch.Spec.Jobs))
		assert.Equal(t, params.JobComponentName,
			scheduledBatch.ObjectMeta.Labels[kube.RadixComponentLabel])
		assert.Equal(t, params.AppName,
			scheduledBatch.ObjectMeta.Labels[kube.RadixAppLabel])
		assert.Equal(t, string(kube.RadixBatchTypeBatch),
			scheduledBatch.ObjectMeta.Labels[kube.RadixBatchTypeLabel])
		assert.Len(t, scheduledBatch.Spec.Jobs, 2)
		assert.ElementsMatch(t, []string{"job1", "job2"},
			slice.Map(scheduledBatch.Spec.Jobs, func(job radixv1.RadixBatchJob) string { return job.JobId }))

		secretList, err := kubeClient.CoreV1().Secrets(rd.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: radixLabels.Merge(
			radixLabels.ForApplicationName(params.AppName),
			radixLabels.ForComponentName(params.JobComponentName),
			radixLabels.ForBatchName(createdBatch.Name),
		).
			String(),
		})
		assert.NoError(t, err)
		require.Len(t, secretList.Items, 1)
		secret := secretList.Items[0]
		assert.Len(t, secret.Data, 2)

		expectedSecrets := slice.Reduce(
			scheduleDescription.JobScheduleDescriptions,
			map[string]string{},
			func(acc map[string]string, job models.JobScheduleDescription) map[string]string {
				acc[job.JobId] = job.Payload
				return acc
			},
		)
		jobNameIdMap := slice.Reduce(
			scheduledBatch.Spec.Jobs,
			map[string]string{},
			func(acc map[string]string, job radixv1.RadixBatchJob) map[string]string {
				acc[job.Name] = job.JobId
				return acc
			})

		for _, radixBatchJob := range scheduledBatch.Spec.Jobs {
			assert.NotNil(t, radixBatchJob.PayloadSecretRef)
			assert.Equal(t, secret.GetName(), radixBatchJob.PayloadSecretRef.Name)
			expectedSecret := expectedSecrets[jobNameIdMap[radixBatchJob.Name]]
			assert.Equal(t, expectedSecret, string(secret.Data[radixBatchJob.PayloadSecretRef.Key]))
			assert.True(t, len(secret.Data[radixBatchJob.PayloadSecretRef.Key]) > 0)
			assert.Equal(t, kube.RadixJobTypeJobSchedule, secret.Labels[kube.RadixJobTypeLabel])
		}
	})
}

func TestStopBatch(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()

	t.Run("cleanup resources for job", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, _, kubeUtil := test.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixBatch1 := test.AddRadixBatch(radixClient, "test-batch1-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		radixBatch2 := test.AddRadixBatch(radixClient, "test-batch2-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		test.CreateSecretForTest(appName, radixBatch1.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch1-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch2-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret3", "test-batch3-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret4", "test-batch4-job1", "other-job-component", envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret5", "test-batch5-job1", appJobComponent, "other-ns", kubeClient)

		handler := New(kubeUtil, config.NewConfigFromEnv(), &radixDeployJobComponent)
		err := handler.StopBatch(context.TODO(), "test-batch1")
		assert.Nil(t, err)
		radixBatchList, _ := radixClient.RadixV1().RadixBatches("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, radixBatchList.Items, 2)
		radixBatch1 = test.GetRadixBatchByNameForTest(radixBatchList.Items, "test-batch1-job1")
		assert.NotNil(t, radixBatch1)
		assert.NotNil(t, test.GetRadixBatchByNameForTest(radixBatchList.Items, "test-batch2-job1"))
		for _, jobStatus := range radixBatch1.Status.JobStatuses {
			assert.Equal(t, radixv1.BatchJobPhaseStopped, jobStatus.Phase)
		}
		for _, job := range radixBatch1.Spec.Jobs {
			assert.NotNil(t, job.Stop)
			assert.Equal(t, pointers.Ptr(true), job.Stop)
		}
		secrets, _ := kubeClient.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, secrets.Items, 5)
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, radixBatch1.Spec.Jobs[0].PayloadSecretRef.Name))
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name))
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, "secret3"))
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, "secret4"))
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, "secret5"))
	})

	t.Run("job name does not exist", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := test.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch2-job2", appJobComponent, kube.RadixBatchTypeJob, envNamespace)

		handler := New(kubeUtil, config.NewConfigFromEnv(), &radixDeployJobComponent)
		err := handler.StopBatch(context.TODO(), "test-batch")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("another job component name", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := test.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch2-job2", "another-job-component", kube.RadixBatchTypeJob, envNamespace)

		handler := New(kubeUtil, config.NewConfigFromEnv(), &radixDeployJobComponent)
		err := handler.StopBatch(context.TODO(), "test-batch")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("another job type", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := test.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch2-job2", appJobComponent, kube.RadixBatchTypeBatch, envNamespace)

		handler := New(kubeUtil, config.NewConfigFromEnv(), &radixDeployJobComponent)
		err := handler.StopBatch(context.TODO(), "test-batch")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("another namespace", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "test-batch-job1"
		radixClient, _, _, kubeUtil := test.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, jobName, appJobComponent, kube.RadixBatchTypeJob, "another-ns")

		handler := New(kubeUtil, config.NewConfigFromEnv(), &radixDeployJobComponent)
		err := handler.StopBatch(context.TODO(), "test-batch")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})
}

func TestStopBatchJob(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()
	t.Run("cleanup resources for job", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, _, kubeUtil := test.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixBatch1 := test.AddRadixBatch(radixClient, "test-batch1-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		radixBatch2 := test.AddRadixBatch(radixClient, "test-batch2-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		test.CreateSecretForTest(appName, radixBatch1.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch1-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch2-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret3", "test-batch3-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret4", "test-batch4-job1", "other-job-component", envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret5", "test-batch5-job1", appJobComponent, "other-ns", kubeClient)

		handler := New(kubeUtil, config.NewConfigFromEnv(), &radixDeployJobComponent)
		err := handler.StopBatchJob(context.TODO(), "test-batch1", "test-batch1-job1")
		assert.Nil(t, err)
		radixBatchList, _ := radixClient.RadixV1().RadixBatches("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, radixBatchList.Items, 2)
		radixBatch1 = test.GetRadixBatchByNameForTest(radixBatchList.Items, "test-batch1-job1")
		assert.NotNil(t, radixBatch1)
		assert.NotNil(t, test.GetRadixBatchByNameForTest(radixBatchList.Items, "test-batch2-job1"))
		for _, jobStatus := range radixBatch1.Status.JobStatuses {
			assert.Equal(t, radixv1.BatchJobPhaseStopped, jobStatus.Phase)
		}
		for _, job := range radixBatch1.Spec.Jobs {
			assert.NotNil(t, job.Stop)
			assert.Equal(t, pointers.Ptr(true), job.Stop)
		}
		secrets, _ := kubeClient.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, secrets.Items, 5)
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, radixBatch1.Spec.Jobs[0].PayloadSecretRef.Name))
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name))
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, "secret3"))
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, "secret4"))
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, "secret5"))
	})

	t.Run("job name does not exist", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := test.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch2-job2", appJobComponent, kube.RadixBatchTypeJob, envNamespace)

		handler := New(kubeUtil, config.NewConfigFromEnv(), &radixDeployJobComponent)
		err := handler.StopBatch(context.TODO(), "test-batch")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("another job component name", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := test.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch2-job2", "another-job-component", kube.RadixBatchTypeJob, envNamespace)

		handler := New(kubeUtil, config.NewConfigFromEnv(), &radixDeployJobComponent)
		err := handler.StopBatch(context.TODO(), "test-batch")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("another job type", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := test.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch2-job2", appJobComponent, kube.RadixBatchTypeBatch, envNamespace)

		handler := New(kubeUtil, config.NewConfigFromEnv(), &radixDeployJobComponent)
		err := handler.StopBatch(context.TODO(), "test-batch")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("another namespace", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "test-batch-job1"
		radixClient, _, _, kubeUtil := test.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, jobName, appJobComponent, kube.RadixBatchTypeJob, "another-ns")

		handler := New(kubeUtil, config.NewConfigFromEnv(), &radixDeployJobComponent)
		err := handler.StopBatch(context.TODO(), "test-batch")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})
}

func TestGetBatchJob(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()
	t.Run("get existing job", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		appNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := test.SetupTest(appName, appEnvironment, appComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "testbatch1-job1", appComponent, kube.RadixBatchTypeBatch, appNamespace)
		test.AddRadixBatch(radixClient, "testbatch2-job1", appComponent, kube.RadixBatchTypeBatch, appNamespace)

		handler := New(kubeUtil, config.NewConfigFromEnv(), &radixDeployJobComponent)
		job1, err := handler.GetBatchJob(context.TODO(), "testbatch1", "testbatch1-job1")
		assert.Nil(t, err)
		assert.Equal(t, "testbatch1-job1", job1.Name)
		job2, err := handler.GetBatchJob(context.TODO(), "testbatch2", "testbatch2-job1")
		assert.Nil(t, err)
		assert.Equal(t, "testbatch2-job1", job2.Name)
	})

	t.Run("job in different app namespace", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		radixClient, _, _, kubeUtil := test.SetupTest(appName, appEnvironment, appComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "testbatch1-job1", appComponent, kube.RadixBatchTypeJob, "app-other")

		handler := New(kubeUtil, config.NewConfigFromEnv(), &radixDeployJobComponent)
		job, err := handler.GetBatchJob(context.TODO(), "testbatch1", "testbatch1-job1")
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
		assert.Nil(t, job)
	})
}
