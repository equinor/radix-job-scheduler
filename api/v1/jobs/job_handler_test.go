package jobs

import (
	"context"
	"fmt"
	"github.com/equinor/radix-common/utils/pointers"
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"strings"
	"testing"

	"github.com/equinor/radix-common/utils/numbers"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	api "github.com/equinor/radix-job-scheduler/api/v1"
	modelsEnv "github.com/equinor/radix-job-scheduler/models"
	models "github.com/equinor/radix-job-scheduler/models/common"
	testUtils "github.com/equinor/radix-job-scheduler/utils/test"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Test_createJob(t *testing.T) {
	radixClient, kubeClient, _, kubeUtil := testUtils.SetupTest("app", "qa", "compute", "app-deploy-1", 1)
	env := modelsEnv.NewEnv()

	h := &jobHandler{
		common: &api.Handler{
			Kube:         kubeUtil,
			KubeClient:   kubeClient,
			RadixClient:  radixClient,
			Env:          env,
			HandlerApiV2: apiv2.New(env, kubeUtil, kubeClient, radixClient),
		},
	}

	t.Run("Create Job", func(t *testing.T) {
		t.Parallel()
		params := testUtils.GetTestParams()
		rd := params.ApplyRd(kubeUtil)

		job, err := h.CreateJob(&models.JobScheduleDescription{Payload: "{}"})

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
	initialEnvVarsConfigMap, _, _ := kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(rd.GetNamespace(),
		rd.Spec.AppName, deployComponent.GetName())
	desiredConfigMap := initialEnvVarsConfigMap.DeepCopy()
	for envVarName, envVarValue := range deployComponent.GetEnvironmentVariables() {
		if strings.HasPrefix(envVarName, "RADIX_") {
			continue
		}
		desiredConfigMap.Data[envVarName] = envVarValue
	}
	err := kubeUtil.ApplyConfigMap(rd.GetNamespace(), initialEnvVarsConfigMap, desiredConfigMap)
	if err != nil {
		panic(err)
	}
	return desiredConfigMap
}

func addRadixBatch(radixClient versioned.Interface, jobName, componentName string, batchJobType kube.RadixBatchType, namespace string) *v1.RadixBatch {
	labels := make(map[string]string)

	if len(strings.TrimSpace(componentName)) > 0 {
		labels[kube.RadixComponentLabel] = componentName
	}
	labels[kube.RadixBatchTypeLabel] = string(batchJobType)

	batchName, batchJobName, ok := api.ParseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		panic(fmt.Sprintf("invalid job name %s", jobName))
	}
	radixBatch, err := radixClient.RadixV1().RadixBatches(namespace).Create(
		context.TODO(),
		&v1.RadixBatch{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchName,
				Labels: labels,
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{
					{
						Name: batchJobName,
						PayloadSecretRef: &v1.PayloadSecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: jobName},
							Key:                  jobName,
						},
					},
				},
			},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		panic(err)
	}
	return radixBatch
}

func TestNewHandler(t *testing.T) {
	radixClient, kubeClient, _, kubeUtil := testUtils.SetupTest("app", "qa", "compute", "app-deploy-1", 1)
	env := modelsEnv.NewEnv()

	h := New(env, kubeUtil)
	assert.IsType(t, &jobHandler{}, h)
	actualHandler := h.(*jobHandler)

	assert.Equal(t, kubeUtil, actualHandler.common.Kube)
	assert.Equal(t, env, actualHandler.common.Env)
	assert.Equal(t, kubeClient, actualHandler.common.KubeClient)
	assert.Equal(t, radixClient, actualHandler.common.RadixClient)
}

func TestGetJobs(t *testing.T) {
	appName, appEnvironment, appComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
	appNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
	radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appComponent, appDeployment, 1)
	addRadixBatch(radixClient, "testbatch1-job1", appComponent, kube.RadixBatchTypeJob, appNamespace)
	addRadixBatch(radixClient, "testbatch2-job2", appComponent, kube.RadixBatchTypeJob, appNamespace)
	addRadixBatch(radixClient, "testbatch3-job3", "other-component", kube.RadixBatchTypeJob, appNamespace)
	addRadixBatch(radixClient, "testbatch4-job4", appComponent, "other-type", appNamespace)
	addRadixBatch(radixClient, "testbatch5-job5", appComponent, kube.RadixBatchTypeJob, "app-other")

	handler := New(modelsEnv.NewEnv(), kubeUtil)
	jobs, err := handler.GetJobs()
	assert.Nil(t, err)
	assert.Len(t, jobs, 2)
	job1 := getJobStatusByNameForTest(jobs, "job1")
	assert.NotNil(t, job1)
	job2 := getJobStatusByNameForTest(jobs, "job2")
	assert.NotNil(t, job2)
}

func TestGetJob(t *testing.T) {

	t.Run("get existing job", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		appNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appComponent, appDeployment, 1)
		addRadixBatch(radixClient, "testbatch1-job1", appComponent, kube.RadixBatchTypeJob, appNamespace)
		addRadixBatch(radixClient, "testbatch2-job1", appComponent, kube.RadixBatchTypeJob, appNamespace)

		handler := New(modelsEnv.NewEnv(), kubeUtil)
		job1, err := handler.GetJob("testbatch1-job1")
		assert.Nil(t, err)
		assert.Equal(t, "testbatch1-job1", job1.Name)
		job2, err := handler.GetJob("testbatch2-job1")
		assert.Nil(t, err)
		assert.Equal(t, "testbatch2-job1", job2.Name)
	})

	t.Run("job in different app namespace", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "a-job"
		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appComponent, appDeployment, 1)
		addRadixBatch(radixClient, jobName, appComponent, kube.RadixBatchTypeJob, "app-other")

		handler := New(modelsEnv.NewEnv(), kubeUtil)
		job, err := handler.GetJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
		assert.Nil(t, job)
	})
}

func TestCreateJob(t *testing.T) {

	// RD job runAsNonRoot (security context)
	t.Run("RD job - static configuration", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
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

		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent,
			appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		handler := New(modelsEnv.NewEnv(), kubeUtil)
		jobStatus, err := handler.CreateJob(&models.JobScheduleDescription{})
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		batchName, batchJobName, ok := api.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		assert.True(t, ok)
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
		appName, appEnvironment, appJobComponent, appDeployment, payloadPath, payloadString := "app", "qa", "compute", "app-deploy-1", "path/payload", "the_payload"
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

		radixClient, kubeClient, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		env := modelsEnv.NewEnv()
		handler := New(env, kubeUtil)
		jobStatus, err := handler.CreateJob(&models.JobScheduleDescription{Payload: payloadString})
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		// Test secret spec
		batchName, jobName, ok := api.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		assert.True(t, ok)
		radixBatch, err := radixClient.RadixV1().RadixBatches(env.RadixDeploymentNamespace).Get(context.Background(), batchName, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Len(t, radixBatch.Spec.Jobs, 1)
		batchJob := radixBatch.Spec.Jobs[0]
		assert.Equal(t, jobName, batchJob.Name)
		secretName := batchJob.PayloadSecretRef.Name
		secret, _ := kubeClient.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
		assert.NotNil(t, secret)
		assert.Len(t, secret.Labels, 3)
		assert.Equal(t, appName, secret.Labels[kube.RadixAppLabel])
		assert.Equal(t, appJobComponent, secret.Labels[kube.RadixComponentLabel])
		assert.Equal(t, batchName, secret.Labels[kube.RadixBatchNameLabel])
		jobPayloadPropertyName := batchJob.PayloadSecretRef.Key
		payloadBytes := secret.Data[jobPayloadPropertyName]
		assert.Equal(t, payloadString, string(payloadBytes))
	})

	t.Run("RD job Without payload path - no secret", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, payloadString := "app", "qa", "compute", "app-deploy-1", "the_payload"
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

		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		env := modelsEnv.NewEnv()
		handler := New(env, kubeUtil)
		_, err = handler.CreateJob(&models.JobScheduleDescription{Payload: payloadString})
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonUnknown, apiErrors.ReasonForError(err))
		assert.Contains(t, err.Error(), "missing an expected payload path, but there is a payload in the job")
	})

	t.Run("RD job with resources - resource specified by request body", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
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

		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		env := modelsEnv.NewEnv()
		handler := New(env, kubeUtil)

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
		jobStatus, err := handler.CreateJob(&jobRequestConfig)
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)

		// Test resources defined
		batchName, jobName, ok := api.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		assert.True(t, ok)
		radixBatch, err := radixClient.RadixV1().RadixBatches(env.RadixDeploymentNamespace).Get(context.Background(), batchName, metav1.GetOptions{})
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
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
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

		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		env := modelsEnv.NewEnv()
		handler := New(env, kubeUtil)
		jobStatus, err := handler.CreateJob(&models.JobScheduleDescription{})
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)

		batchName, jobName, ok := api.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		assert.True(t, ok)
		radixBatch, err := radixClient.RadixV1().RadixBatches(envNamespace).Get(context.Background(), batchName, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Len(t, radixBatch.Spec.Jobs, 1)
		job := radixBatch.Spec.Jobs[0]
		assert.Equal(t, jobName, job.Name)
		assert.Nil(t, job.Resources)
	})

	t.Run("RD job not defined", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(
				utils.NewDeployJobComponentBuilder().
					WithTimeLimitSeconds(numbers.Int64Ptr(10)).
					WithName("another-job"),
			).
			BuildRD()

		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		handler := New(modelsEnv.NewEnv(), kubeUtil)
		jobStatus, err := handler.CreateJob(&models.JobScheduleDescription{})
		assert.Nil(t, jobStatus)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
		assert.Equal(t, apiErrors.NotFoundMessage("job component", appJobComponent), err.Error())
	})

	t.Run("radix deployment does not exist", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
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

		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		handler := New(modelsEnv.NewEnv(), kubeUtil)
		jobStatus, err := handler.CreateJob(&models.JobScheduleDescription{})
		assert.Nil(t, jobStatus)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
		assert.Equal(t, apiErrors.NotFoundMessage("radix deployment", appDeployment), err.Error())
	})

	t.Run("RD job with GPU", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
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

		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		applyRadixDeploymentEnvVarsConfigMaps(kubeUtil, rd)
		_, err := radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		handler := New(modelsEnv.NewEnv(), kubeUtil)
		jobStatus, err := handler.CreateJob(&models.JobScheduleDescription{
			RadixJobComponentConfig: models.RadixJobComponentConfig{
				Node: &v1.RadixNode{
					Gpu:      "gpu1, gpu2",
					GpuCount: "2",
				},
			},
		})
		assert.NotNil(t, jobStatus)
		assert.Nil(t, err)

		batchName, jobName, ok := api.ParseBatchAndJobNameFromScheduledJobName(jobStatus.Name)
		assert.True(t, ok)
		radixBatch, err := radixClient.RadixV1().RadixBatches(envNamespace).Get(context.Background(), batchName, metav1.GetOptions{})
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
	t.Run("delete job - cleanup resources for job", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixBatch1 := addRadixBatch(radixClient, "test-batch1-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		radixBatch2 := addRadixBatch(radixClient, "test-batch2-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		createSecretForTest(appName, radixBatch1.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch1-job1", appJobComponent, envNamespace, kubeClient)
		createSecretForTest(appName, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch2-job1", appJobComponent, envNamespace, kubeClient)
		createSecretForTest(appName, "secret3", "test-batch3-job1", appJobComponent, envNamespace, kubeClient)
		createSecretForTest(appName, "secret4", "test-batch4-job1", "other-job-component", envNamespace, kubeClient)
		createSecretForTest(appName, "secret5", "test-batch5-job1", appJobComponent, "other-ns", kubeClient)

		handler := New(modelsEnv.NewEnv(), kubeUtil)
		err := handler.DeleteJob("test-batch1-job1")
		assert.Nil(t, err)
		radixBatchList, _ := radixClient.RadixV1().RadixBatches("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, radixBatchList.Items, 1)
		assert.NotNil(t, getRadixBatchByNameForTest(radixBatchList.Items, "test-batch2-job1"))
		secrets, _ := kubeClient.CoreV1().Secrets("").List(context.Background(), metav1.ListOptions{})
		assert.Len(t, secrets.Items, 4)
		assert.NotNil(t, getSecretByNameForTest(secrets.Items, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name))
		assert.NotNil(t, getSecretByNameForTest(secrets.Items, "secret3"))
		assert.NotNil(t, getSecretByNameForTest(secrets.Items, "secret4"))
		assert.NotNil(t, getSecretByNameForTest(secrets.Items, "secret5"))
	})

	t.Run("delete job - job name does not exist", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		addRadixBatch(radixClient, "test-batch-another-job", appJobComponent, kube.RadixBatchTypeJob, envNamespace)

		handler := New(modelsEnv.NewEnv(), kubeUtil)
		err := handler.DeleteJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("delete job - another job component name", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		addRadixBatch(radixClient, "test-batch-another-job", "another-job-component", kube.RadixBatchTypeJob, envNamespace)

		handler := New(modelsEnv.NewEnv(), kubeUtil)
		err := handler.DeleteJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("delete job - another job type", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		addRadixBatch(radixClient, "test-batch-another-job", appJobComponent, kube.RadixBatchTypeBatch, envNamespace)

		handler := New(modelsEnv.NewEnv(), kubeUtil)
		err := handler.DeleteJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("delete job - another namespace", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "test-batch-job1"
		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		addRadixBatch(radixClient, jobName, appJobComponent, kube.RadixBatchTypeJob, "another-ns")

		handler := New(modelsEnv.NewEnv(), kubeUtil)
		err := handler.DeleteJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})
}
func TestStopJob(t *testing.T) {
	t.Run("stop job - cleanup resources for job", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixBatch1 := addRadixBatch(radixClient, "test-batch1-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		radixBatch2 := addRadixBatch(radixClient, "test-batch2-job1", appJobComponent, kube.RadixBatchTypeJob, envNamespace)
		createSecretForTest(appName, radixBatch1.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch1-job1", appJobComponent, envNamespace, kubeClient)
		createSecretForTest(appName, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name, "test-batch2-job1", appJobComponent, envNamespace, kubeClient)
		createSecretForTest(appName, "secret3", "test-batch3-job1", appJobComponent, envNamespace, kubeClient)
		createSecretForTest(appName, "secret4", "test-batch4-job1", "other-job-component", envNamespace, kubeClient)
		createSecretForTest(appName, "secret5", "test-batch5-job1", appJobComponent, "other-ns", kubeClient)

		handler := New(modelsEnv.NewEnv(), kubeUtil)
		err := handler.StopJob("test-batch1-job1")
		assert.Nil(t, err)
		radixBatchList, _ := radixClient.RadixV1().RadixBatches("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, radixBatchList.Items, 2)
		radixBatch1 = getRadixBatchByNameForTest(radixBatchList.Items, "test-batch1-job1")
		assert.NotNil(t, radixBatch1)
		assert.NotNil(t, getRadixBatchByNameForTest(radixBatchList.Items, "test-batch2-job1"))
		for _, jobStatus := range radixBatch1.Status.JobStatuses {
			assert.Equal(t, v1.BatchJobPhaseStopped, jobStatus.Phase)
		}
		for _, job := range radixBatch1.Spec.Jobs {
			assert.NotNil(t, job.Stop)
			assert.Equal(t, pointers.Ptr(true), job.Stop)
		}
		secrets, _ := kubeClient.CoreV1().Secrets("").List(context.Background(), metav1.ListOptions{})
		assert.Len(t, secrets.Items, 5)
		assert.NotNil(t, getSecretByNameForTest(secrets.Items, radixBatch1.Spec.Jobs[0].PayloadSecretRef.Name))
		assert.NotNil(t, getSecretByNameForTest(secrets.Items, radixBatch2.Spec.Jobs[0].PayloadSecretRef.Name))
		assert.NotNil(t, getSecretByNameForTest(secrets.Items, "secret3"))
		assert.NotNil(t, getSecretByNameForTest(secrets.Items, "secret4"))
		assert.NotNil(t, getSecretByNameForTest(secrets.Items, "secret5"))
	})

	t.Run("stop job - job name does not exist", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		addRadixBatch(radixClient, "test-batch-another-job", appJobComponent, kube.RadixBatchTypeJob, envNamespace)

		handler := New(modelsEnv.NewEnv(), kubeUtil)
		err := handler.StopJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("stop job - another job component name", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		addRadixBatch(radixClient, "test-batch-another-job", "another-job-component", kube.RadixBatchTypeJob, envNamespace)

		handler := New(modelsEnv.NewEnv(), kubeUtil)
		err := handler.StopJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("stop job - another job type", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "test-batch-job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		addRadixBatch(radixClient, "test-batch-another-job", appJobComponent, kube.RadixBatchTypeBatch, envNamespace)

		handler := New(modelsEnv.NewEnv(), kubeUtil)
		err := handler.StopJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})

	t.Run("stop job - another namespace", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "test-batch-job1"
		radixClient, _, _, kubeUtil := testUtils.SetupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		addRadixBatch(radixClient, jobName, appJobComponent, kube.RadixBatchTypeJob, "another-ns")

		handler := New(modelsEnv.NewEnv(), kubeUtil)
		err := handler.StopJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, apiErrors.ReasonForError(err))
	})
}

func createSecretForTest(appName, secretName, jobName, radixJobComponentName, namespace string, kubeClient kubernetes.Interface) {
	batchName, batchJobName, _ := api.ParseBatchAndJobNameFromScheduledJobName(jobName)
	_, err := kubeClient.CoreV1().Secrets(namespace).Create(
		context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
				Labels: radixLabels.Merge(
					radixLabels.ForApplicationName(appName),
					radixLabels.ForComponentName(radixJobComponentName),
					radixLabels.ForBatchName(batchName),
				),
			},
			Data: map[string][]byte{batchJobName: []byte("secret")},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		panic(err)
	}
}

func getJobStatusByNameForTest(jobs []modelsv1.JobStatus, name string) *modelsv1.JobStatus {
	for _, job := range jobs {
		if strings.HasSuffix(job.Name, "-"+name) {
			return &job
		}
	}
	return nil
}

func getSecretByNameForTest(secrets []corev1.Secret, name string) *corev1.Secret {
	for _, secret := range secrets {
		if secret.Name == name {
			return &secret
		}
	}

	return nil
}

func getRadixBatchByNameForTest(radixBatches []v1.RadixBatch, jobName string) *v1.RadixBatch {
	batchName, _, _ := api.ParseBatchAndJobNameFromScheduledJobName(jobName)
	for _, radixBatch := range radixBatches {
		if radixBatch.Name == batchName {
			return &radixBatch
		}
	}

	return nil
}
