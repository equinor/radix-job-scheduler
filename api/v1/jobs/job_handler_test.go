package jobs

import (
	"context"
	"strings"
	"testing"

	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-common/utils/pointers"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/api/test"
	apiv1 "github.com/equinor/radix-job-scheduler/api/v1"
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"github.com/equinor/radix-job-scheduler/internal"
	testUtil "github.com/equinor/radix-job-scheduler/internal/test"
	modelsEnv "github.com/equinor/radix-job-scheduler/models"
	models "github.com/equinor/radix-job-scheduler/models/common"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_createJob(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()
	_, _, _, kubeUtil := testUtil.SetupTest("app", "qa", appJobComponent, "app-deploy-1", 1)
	env := modelsEnv.NewEnv()

	h := &jobHandler{
		common: &apiv1.Handler{
			Kube:         kubeUtil,
			Env:          env,
			HandlerApiV2: apiv2.New(kubeUtil, env, &radixDeployJobComponent),
		},
	}

	t.Run("Create Job", func(t *testing.T) {
		t.Parallel()
		params := test.GetTestParams()
		rd := params.ApplyRd(kubeUtil)

		job, err := h.CreateJob(context.TODO(), &models.JobScheduleDescription{Payload: "{}"})

		assert.NoError(t, err)
		assert.NotNil(t, job)
		assert.True(t, strings.HasPrefix(job.Name, "batch-"+rd.Labels[kube.RadixComponentLabel]))
	})
}

func applyRadixDeploymentEnvVarsConfigMaps(kubeUtil *kube.Kube, rd *v1.RadixDeployment) map[string]*corev1.ConfigMap {

	envVarConfigMapsMap := map[string]*corev1.ConfigMap{}
	for _, deployComponent := range rd.Spec.Components {
		envVarConfigMapsMap[deployComponent.GetName()] = ensurePopulatedEnvVarsConfigMaps(kubeUtil, rd, &deployComponent)
	}
	for _, deployJoyComponent := range rd.Spec.Jobs {
		envVarConfigMapsMap[deployJoyComponent.GetName()] = ensurePopulatedEnvVarsConfigMaps(kubeUtil, rd, &deployJoyComponent)
	}
	return envVarConfigMapsMap
}

func ensurePopulatedEnvVarsConfigMaps(kubeUtil *kube.Kube, rd *v1.RadixDeployment, deployComponent v1.RadixCommonDeployComponent) *corev1.ConfigMap {
	initialEnvVarsConfigMap, _, _ := kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(context.Background(), rd.GetNamespace(), rd.Spec.AppName, deployComponent.GetName())
	desiredConfigMap := initialEnvVarsConfigMap.DeepCopy()
	for envVarName, envVarValue := range deployComponent.GetEnvironmentVariables() {
		if strings.HasPrefix(envVarName, "RADIX_") {
			continue
		}
		desiredConfigMap.Data[envVarName] = envVarValue
	}
	err := kubeUtil.ApplyConfigMap(context.Background(), rd.GetNamespace(), initialEnvVarsConfigMap, desiredConfigMap)
	if err != nil {
		panic(err)
	}
	return desiredConfigMap
}

func TestNewHandler(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()
	radixClient, kubeClient, _, kubeUtil := testUtil.SetupTest("app", "qa", appJobComponent, "app-deploy-1", 1)
	env := modelsEnv.NewEnv()

	h := New(kubeUtil, env, &radixDeployJobComponent)
	assert.IsType(t, &jobHandler{}, h)
	actualHandler := h.(*jobHandler)

	assert.Equal(t, kubeUtil, actualHandler.common.Kube)
	assert.Equal(t, env, actualHandler.common.Env)
	assert.Equal(t, kubeClient, actualHandler.common.Kube.KubeClient())
	assert.Equal(t, radixClient, actualHandler.common.Kube.RadixClient())
}

func TestGetJobs(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()
	appName, appEnvironment, appComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
	appNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
	radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appComponent, appDeployment, 1)
	test.AddRadixBatch(radixClient, "testbatch1-job1", appComponent, kube.RadixBatchTypeJob, appNamespace)
	test.AddRadixBatch(radixClient, "testbatch2-job2", appComponent, kube.RadixBatchTypeJob, appNamespace)
	test.AddRadixBatch(radixClient, "testbatch3-job3", "other-component", kube.RadixBatchTypeJob, appNamespace)
	test.AddRadixBatch(radixClient, "testbatch4-job4", appComponent, "other-type", appNamespace)
	test.AddRadixBatch(radixClient, "testbatch5-job5", appComponent, kube.RadixBatchTypeJob, "app-other")

	handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
	jobs, err := handler.GetJobs(context.TODO())
	assert.Nil(t, err)
	assert.Len(t, jobs, 2)
	job1 := test.GetJobStatusByNameForTest(jobs, "job1")
	assert.NotNil(t, job1)
	assert.Equal(t, "testbatch1-job1", job1.Name)
	assert.Equal(t, "", job1.BatchName)
	job2 := test.GetJobStatusByNameForTest(jobs, "job2")
	assert.NotNil(t, job2)
	assert.Equal(t, "testbatch2-job2", job2.Name)
	assert.Equal(t, "", job2.BatchName)
}

func TestGetJob(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()

	t.Run("get existing job", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		appNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "testbatch1-job1", appComponent, kube.RadixBatchTypeJob, appNamespace)
		test.AddRadixBatch(radixClient, "testbatch2-job1", appComponent, kube.RadixBatchTypeJob, appNamespace)

		radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appComponent).BuildJobComponent()
		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		job1, err := handler.GetJob(context.TODO(), "testbatch1-job1")
		assert.Nil(t, err)
		assert.Equal(t, "testbatch1-job1", job1.Name)
		assert.Equal(t, "", job1.BatchName)
		job2, err := handler.GetJob(context.TODO(), "testbatch2-job1")
		assert.Nil(t, err)
		assert.Equal(t, "testbatch2-job1", job2.Name)
		assert.Equal(t, "", job2.BatchName)
	})

	t.Run("job in different app namespace", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appComponent, appDeployment, jobName := "app", "qa", appJobComponent, "app-deploy-1", "a-job"
		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, jobName, appComponent, kube.RadixBatchTypeJob, "app-other")

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		job, err := handler.GetJob(context.TODO(), jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
		assert.Nil(t, job)
	})
}

func TestCreateJob(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()

	// RD job runAsNonRoot (security context)
	t.Run("RD job - static configuration", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(
				utils.NewDeployJobComponentBuilder().
					WithTimeLimitSeconds(numbers.Int64Ptr(10)).
					WithName(appJobComponent),
			).
			BuildRD()

		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent,
			appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		jobStatus, err := handler.CreateJob(context.TODO(), &models.JobScheduleDescription{})
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		batchName, batchJobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		assert.True(t, ok)
		assert.Equal(t, "", jobStatus.BatchName)
		radixBatch, _ := radixClient.RadixV1().RadixBatches(envNamespace).Get(context.TODO(), batchName, metav1.GetOptions{})
		assert.Len(t, radixBatch.Labels, 3)
		assert.Equal(t, appName, radixBatch.Labels[kube.RadixAppLabel])
		assert.Equal(t, appJobComponent, radixBatch.Labels[kube.RadixComponentLabel])
		assert.Equal(t, string(kube.RadixBatchTypeJob), radixBatch.Labels[kube.RadixBatchTypeLabel])
		assert.Len(t, radixBatch.Spec.Jobs, 1)
		job := radixBatch.Spec.Jobs[0]
		assert.Equal(t, batchJobName, job.Name)
	})

	t.Run("RD job with payload path - secret exists", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, payloadPath, payloadString := "app", "qa", appJobComponent, "app-deploy-1", "path/payload", "the_payload"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(
				utils.NewDeployJobComponentBuilder().
					WithTimeLimitSeconds(numbers.Int64Ptr(10)).
					WithName(appJobComponent).
					WithPayloadPath(&payloadPath),
			).
			BuildRD()

		radixClient, kubeClient, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		env := modelsEnv.NewEnv()
		handler := New(kubeUtil, env, &radixDeployJobComponent)
		jobStatus, err := handler.CreateJob(context.TODO(), &models.JobScheduleDescription{Payload: payloadString})
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		// Test secret spec
		batchName, jobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		assert.True(t, ok)
		radixBatch, err := radixClient.RadixV1().RadixBatches(env.RadixDeploymentNamespace).Get(context.TODO(), batchName, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Len(t, radixBatch.Spec.Jobs, 1)
		batchJob := radixBatch.Spec.Jobs[0]
		assert.Equal(t, jobName, batchJob.Name)
		secretName := batchJob.PayloadSecretRef.Name
		secret, _ := kubeClient.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
		assert.NotNil(t, secret)
		assert.Len(t, secret.Labels, 5)
		assert.Equal(t, appName, secret.Labels[kube.RadixAppLabel])
		assert.Equal(t, appJobComponent, secret.Labels[kube.RadixComponentLabel])
		assert.Equal(t, batchName, secret.Labels[kube.RadixBatchNameLabel])
		assert.Equal(t, kube.RadixJobTypeJobSchedule, secret.Labels[kube.RadixJobTypeLabel])
		assert.Equal(t, string(kube.RadixSecretJobPayload), secret.Labels[kube.RadixSecretTypeLabel])
		jobPayloadPropertyName := batchJob.PayloadSecretRef.Key
		payloadBytes := secret.Data[jobPayloadPropertyName]
		assert.Equal(t, payloadString, string(payloadBytes))
	})

	t.Run("RD job Without payload path - no secret", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, payloadString := "app", "qa", appJobComponent, "app-deploy-1", "the_payload"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(
				utils.NewDeployJobComponentBuilder().
					WithTimeLimitSeconds(numbers.Int64Ptr(10)).
					WithName(appJobComponent),
			).
			BuildRD()

		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		env := modelsEnv.NewEnv()
		handler := New(kubeUtil, env, &radixDeployJobComponent)
		_, err = handler.CreateJob(context.TODO(), &models.JobScheduleDescription{Payload: payloadString})
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonUnknown, apiErrors.ReasonForError(err))
		assert.Contains(t, err.Error(), "missing an expected payload path, but there is a payload in the job")
	})

	t.Run("RD job with resources - resource specified by request body", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(
				utils.NewDeployJobComponentBuilder().
					WithTimeLimitSeconds(numbers.Int64Ptr(10)).
					WithName(appJobComponent),
			).
			BuildRD()

		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		env := modelsEnv.NewEnv()
		handler := New(kubeUtil, env, &radixDeployJobComponent)

		jobRequestConfig := models.JobScheduleDescription{
			RadixJobComponentConfig: models.RadixJobComponentConfig{
				Resources: &v1.ResourceRequirements{
					Requests: v1.ResourceList{
						"cpu":    "50m",
						"memory": "60M",
					},
					Limits: v1.ResourceList{
						"cpu":    "100m",
						"memory": "120M",
					},
				},
			},
		}
		jobStatus, err := handler.CreateJob(context.TODO(), &jobRequestConfig)
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)

		// Test resources defined
		batchName, jobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		assert.True(t, ok)
		radixBatch, err := radixClient.RadixV1().RadixBatches(env.RadixDeploymentNamespace).Get(context.TODO(), batchName, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Len(t, radixBatch.Spec.Jobs, 1)
		job := radixBatch.Spec.Jobs[0]
		assert.Equal(t, jobName, job.Name)
		// Test CPU resource set by request
		assert.Len(t, job.Resources.Requests, 2)
		assert.Len(t, job.Resources.Limits, 2)
		assert.Equal(t, "50m", job.Resources.Requests["cpu"])
		assert.Equal(t, "60M", job.Resources.Requests["memory"])
		assert.Equal(t, "100m", job.Resources.Limits["cpu"])
		assert.Equal(t, "120M", job.Resources.Limits["memory"])
	})

	t.Run("RD job Without resources", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(
				utils.NewDeployJobComponentBuilder().
					WithTimeLimitSeconds(numbers.Int64Ptr(10)).
					WithName(appJobComponent),
			).
			BuildRD()

		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		env := modelsEnv.NewEnv()
		handler := New(kubeUtil, env, &radixDeployJobComponent)
		jobStatus, err := handler.CreateJob(context.TODO(), &models.JobScheduleDescription{})
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)

		batchName, jobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		assert.True(t, ok)
		radixBatch, err := radixClient.RadixV1().RadixBatches(envNamespace).Get(context.TODO(), batchName, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Len(t, radixBatch.Spec.Jobs, 1)
		job := radixBatch.Spec.Jobs[0]
		assert.Equal(t, jobName, job.Name)
		assert.Nil(t, job.Resources)
	})

	t.Run("RD job not defined", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(
				utils.NewDeployJobComponentBuilder().
					WithTimeLimitSeconds(numbers.Int64Ptr(10)).
					WithName("anotherjob"),
			).
			BuildRD()

		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		jobStatus, err := handler.CreateJob(context.TODO(), &models.JobScheduleDescription{})
		assert.Nil(t, jobStatus)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
		assert.Equal(t, apiErrors.NotFoundMessage("job component", appJobComponent), err.Error())
	})

	t.Run("radix deployment does not exist", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName("another-deployment").
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(
				utils.NewDeployJobComponentBuilder().
					WithTimeLimitSeconds(numbers.Int64Ptr(10)).
					WithName(appJobComponent),
			).
			BuildRD()

		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		jobStatus, err := handler.CreateJob(context.TODO(), &models.JobScheduleDescription{})
		assert.Nil(t, jobStatus)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
		assert.Equal(t, apiErrors.NotFoundMessage("radix deployment", appDeployment), err.Error())
	})

	t.Run("RD job with GPU", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(
				utils.NewDeployJobComponentBuilder().
					WithTimeLimitSeconds(numbers.Int64Ptr(10)).
					WithName(appJobComponent).
					WithNodeGpu("gpu1, gpu2").
					WithNodeGpuCount("2"),
			).
			BuildRD()

		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		jobStatus, err := handler.CreateJob(context.TODO(), &models.JobScheduleDescription{
			RadixJobComponentConfig: models.RadixJobComponentConfig{
				Node: &v1.RadixNode{
					Gpu:      "gpu1, gpu2",
					GpuCount: "2",
				},
			},
		})
		assert.NotNil(t, jobStatus)
		assert.Nil(t, err)

		batchName, jobName, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		assert.True(t, ok)
		radixBatch, err := radixClient.RadixV1().RadixBatches(envNamespace).Get(context.TODO(), batchName, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Len(t, radixBatch.Spec.Jobs, 1)
		job := radixBatch.Spec.Jobs[0]
		assert.Equal(t, jobName, job.Name)

		assert.NotNil(t, job.Node)
		assert.Equal(t, job.Node.Gpu, "gpu1, gpu2")
		assert.Equal(t, job.Node.GpuCount, "2")
	})
}

func TestDeleteJob(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()
	t.Run("delete job - cleanup resources for job", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixBatch1 := test.AddRadixBatch(radixClient, "test-batch1-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		radixBatch2 := test.AddRadixBatch(radixClient, "test-batch2-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		test.CreateSecretForTest(appName, radixBatch1.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch1-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch2-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret3", "test-batch3-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret4", "test-batch4-job1", "other-job-component", envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret5", "test-batch5-job1", appJobComponent, "other-ns", kubeClient)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.DeleteJob(context.TODO(), "test-batch1-job1")
		assert.Nil(t, err)
		radixBatchList, _ := radixClient.RadixV1().RadixBatches("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, radixBatchList.Items, 1)
		assert.NotNil(t, test.GetRadixBatchByNameForTest(radixBatchList.Items, "test-batch2-job1"))
		secrets, _ := kubeClient.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, secrets.Items, 3)
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name), "remaining test-batch2-job1")
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, "secret4"), "other-job-component")
		assert.NotNil(t, test.GetSecretByNameForTest(secrets.Items, "secret5"), "other-ns")
	})

	t.Run("delete job - job name does not exist", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", appJobComponent, "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch-anotherjob", appJobComponent, kube.RadixBatchTypeJob, envNamespace)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.DeleteJob(context.TODO(), jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("delete job - another job component name", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", appJobComponent, "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch-anotherjob", "anotherjob-component", kube.RadixBatchTypeJob, envNamespace)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.DeleteJob(context.TODO(), jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("delete job - another job type", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", appJobComponent, "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch-anotherjob", appJobComponent, kube.RadixBatchTypeBatch, envNamespace)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.DeleteJob(context.TODO(), jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonInvalid, apiErrors.ReasonForError(err))
	})

	t.Run("delete job - another namespace", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", appJobComponent, "app-deploy-1", "test-batch-job1"
		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, jobName, appJobComponent, kube.RadixBatchTypeJob, "another-ns")

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.DeleteJob(context.TODO(), jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})
}
func TestStopJob(t *testing.T) {
	appJobComponent := "compute"
	radixDeployJobComponent := utils.NewDeployJobComponentBuilder().WithName(appJobComponent).BuildJobComponent()
	t.Run("stop job - cleanup resources for job", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", appJobComponent, "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixBatch1 := test.AddRadixBatch(radixClient, "test-batch1-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		radixBatch2 := test.AddRadixBatch(radixClient, "test-batch2-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		test.CreateSecretForTest(appName, radixBatch1.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch1-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch2-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret3", "test-batch3-job1", appJobComponent, envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret4", "test-batch4-job1", "other-job-component", envNamespace, kubeClient)
		test.CreateSecretForTest(appName, "secret5", "test-batch5-job1", appJobComponent, "other-ns", kubeClient)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.StopJob(context.TODO(), "test-batch1-job1")
		assert.Nil(t, err)
		radixBatchList, _ := radixClient.RadixV1().RadixBatches("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, radixBatchList.Items, 2)
		radixBatch1 = test.GetRadixBatchByNameForTest(radixBatchList.Items, "test-batch1-job1")
		assert.NotNil(t, radixBatch1)
		assert.NotNil(t, test.GetRadixBatchByNameForTest(radixBatchList.Items, "test-batch2-job1"))
		for _, jobStatus := range radixBatch1.Status.JobStatuses {
			assert.Equal(t, v1.BatchJobPhaseStopped, jobStatus.Phase)
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

	t.Run("stop job - job name does not exist", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", appJobComponent, "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch-anotherjob", appJobComponent, kube.RadixBatchTypeJob, envNamespace)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.StopJob(context.TODO(), jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("stop job - another job component name", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", appJobComponent, "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch-anotherjob", "anotherjob-component", kube.RadixBatchTypeJob, envNamespace)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.StopJob(context.TODO(), jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("stop job - another job type", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", appJobComponent, "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, "test-batch-anotherjob", appJobComponent, kube.RadixBatchTypeBatch, envNamespace)

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.StopJob(context.TODO(), jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("stop job - another namespace", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", appJobComponent, "app-deploy-1", "test-batch-job1"
		radixClient, _, _, kubeUtil := testUtil.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		test.AddRadixBatch(radixClient, jobName, appJobComponent, kube.RadixBatchTypeJob, "another-ns")

		handler := New(kubeUtil, modelsEnv.NewEnv(), &radixDeployJobComponent)
		err := handler.StopJob(context.TODO(), jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})
}
