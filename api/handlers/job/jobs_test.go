package job

import (
	radixUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func Test_createJob(t *testing.T) {
	radixClient, kubeClient, kubeUtil := setupTest("app", "qa", "compute", "app-deploy-1", 1)
	env := models.NewEnv()

	h := &jobHandler{
		kube:                   kubeUtil,
		kubeClient:             kubeClient,
		radixClient:            radixClient,
		env:                    env,
		securityContextBuilder: deployment.NewSecurityContextBuilder(true),
	}

	t.Run("Create Job", func(t *testing.T) {
		t.Parallel()
		params := getTestParams()
		rd := params.applyRd(kubeUtil)

		job, err := h.createJob(params.jobName, &rd.Spec.Jobs[0], rd, &corev1.Secret{}, &models.JobScheduleDescription{Payload: "{}"})

		assert.NoError(t, err)
		assert.Equal(t, params.jobName, job.Name)
		assert.Equal(t, params.namespace, job.Namespace)
	})
}

func Test_createJobWithEnvVars(t *testing.T) {
	t.Run("Create Job with new env-vars", func(t *testing.T) {
		t.Parallel()
		radixClient, kubeClient, kubeUtil := setupTest("app", "qa", "compute", "app-deploy-1", 1)
		env := models.NewEnv()
		h := &jobHandler{
			kube:                   kubeUtil,
			kubeClient:             kubeClient,
			radixClient:            radixClient,
			env:                    env,
			securityContextBuilder: deployment.NewSecurityContextBuilder(true),
		}
		params := getTestParams().withRadixConfigEnvVarsMap(map[string]string{"VAR1": "val1", "VAR2": "val2"})
		rd := params.applyRd(kubeUtil)

		job, err := h.createJob(params.jobName, &rd.Spec.Jobs[0], rd, &corev1.Secret{}, &models.JobScheduleDescription{Payload: "{}"})

		assert.NoError(t, err)
		envVars := job.Spec.Template.Spec.Containers[0].Env
		assert.Len(t, envVars, 2)
		envVarsMap := getEnvVarsMap(envVars)
		assert.Equal(t, "val1", envVarsMap["VAR1"].Value)
		assert.Equal(t, "val2", envVarsMap["VAR2"].Value)
	})

	t.Run("Create Job with updated and deleted env-vars", func(t *testing.T) {
		t.Parallel()
		radixClient, kubeClient, kubeUtil := setupTest("app", "qa", "compute", "app-deploy-1", 1)
		env := models.NewEnv()
		h := &jobHandler{
			kube:                   kubeUtil,
			kubeClient:             kubeClient,
			radixClient:            radixClient,
			env:                    env,
			securityContextBuilder: deployment.NewSecurityContextBuilder(true),
		}
		params := getTestParams().
			withRadixConfigEnvVarsMap(map[string]string{"VAR1": "val1", "VAR2": "orig-val2"}).
			withEnvVarsConfigMapData(map[string]string{"VAR1": "val1", "VAR2": "edited-val2"}).
			withEnvVarsMetadataConfigMapData(map[string]string{"VAR2": "orig-val2"})
		rd := params.applyRd(kubeUtil)

		job, err := h.createJob(params.jobName, &rd.Spec.Jobs[0], rd, &corev1.Secret{}, &models.JobScheduleDescription{Payload: "{}"})

		envVarsConfigMap, _, envVarsMetadataMap, err := kubeUtil.GetEnvVarsConfigMapAndMetadataMap(params.namespace, params.jobName)
		assert.NoError(t, err)
		envVars := job.Spec.Template.Spec.Containers[0].Env
		assert.Len(t, envVars, 2)
		envVarsMap := getEnvVarsMap(envVars)
		assert.NotNil(t, envVarsMap["VAR1"].ValueFrom)
		assert.Equal(t, "val1", envVarsConfigMap.Data["VAR1"])
		assert.NotNil(t, envVarsMap["VAR2"].ValueFrom)
		assert.Equal(t, "edited-val2", envVarsConfigMap.Data["VAR2"])
		assert.NotEmpty(t, envVarsMetadataMap)
		assert.NotEmpty(t, envVarsMetadataMap["VAR2"])
		assert.Equal(t, "orig-val2", envVarsMetadataMap["VAR2"].RadixConfigValue)
	})
}

func getEnvVarsMap(envVars []corev1.EnvVar) map[string]corev1.EnvVar {
	envVarsMap := make(map[string]corev1.EnvVar)
	for _, envVar := range envVars {
		envVar := envVar
		envVarsMap[envVar.Name] = envVar
	}
	return envVarsMap
}

func (params *testParams) applyRd(kubeUtil *kube.Kube) *v1.RadixDeployment {
	envVarsConfigMap, envVarsMetadataConfigMap, _ := kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(params.namespace, params.appName, params.jobComponentName)
	envVarsConfigMap.Data = params.envVarsConfigMapData
	metadataMap := make(map[string]kube.EnvVarMetadata)
	for name, value := range params.envVarsMetadataConfigMapData {
		metadataMap[name] = kube.EnvVarMetadata{RadixConfigValue: value}
	}
	kube.SetEnvVarsMetadataMapToConfigMap(envVarsMetadataConfigMap, metadataMap)
	kubeUtil.UpdateConfigMap(params.namespace, envVarsConfigMap, envVarsMetadataConfigMap)

	rd := utils.ARadixDeployment().
		WithDeploymentName(params.deploymentName).
		WithAppName(params.appName).
		WithEnvironment(params.environment).
		WithComponents().
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().
				WithName(params.jobComponentName).
				WithPayloadPath(radixUtils.StringPtr("payload-path")).
				WithEnvironmentVariables(params.radixConfigEnvVarsMap),
		).
		BuildRD()
	return rd
}

type testParams struct {
	appName                      string
	environment                  string
	namespace                    string
	jobComponentName             string
	deploymentName               string
	jobName                      string
	radixConfigEnvVarsMap        map[string]string
	envVarsConfigMapData         map[string]string
	envVarsMetadataConfigMapData map[string]string
}

func getTestParams() *testParams {
	params := testParams{
		appName:                      "app",
		environment:                  "qa",
		jobComponentName:             "compute",
		deploymentName:               "app-deploy-1",
		jobName:                      "some-job",
		radixConfigEnvVarsMap:        make(map[string]string),
		envVarsConfigMapData:         make(map[string]string),
		envVarsMetadataConfigMapData: make(map[string]string),
	}
	params.namespace = utils.GetEnvironmentNamespace(params.appName, params.environment)
	return &params
}

func (params *testParams) withRadixConfigEnvVarsMap(envVars map[string]string) *testParams {
	params.radixConfigEnvVarsMap = envVars
	return params
}

func (params *testParams) withEnvVarsConfigMapData(envVars map[string]string) *testParams {
	params.envVarsConfigMapData = envVars
	return params
}

func (params *testParams) withEnvVarsMetadataConfigMapData(envVars map[string]string) *testParams {
	params.envVarsMetadataConfigMapData = envVars
	return params
}
