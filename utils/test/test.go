package test

import (
	"context"
	"fmt"
	"os"

	radixUtils "github.com/equinor/radix-common/utils"
	numbers "github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fake2 "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	fake3 "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func SetupTest(appName, appEnvironment, appComponent, appDeployment string, historyLimit int) (versioned.Interface,
	kubernetes.Interface, prometheusclient.Interface, *kube.Kube) {
	os.Setenv("RADIX_APP", appName)
	os.Setenv("RADIX_ENVIRONMENT", appEnvironment)
	os.Setenv("RADIX_COMPONENT", appComponent)
	os.Setenv("RADIX_DEPLOYMENT", appDeployment)
	os.Setenv("RADIX_JOB_SCHEDULERS_PER_ENVIRONMENT_HISTORY_LIMIT", fmt.Sprint(historyLimit))
	os.Setenv(defaults.OperatorRollingUpdateMaxUnavailable, "25%")
	os.Setenv(defaults.OperatorRollingUpdateMaxSurge, "25%")
	os.Setenv(defaults.OperatorEnvLimitDefaultCPUEnvironmentVariable, "200m")
	os.Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "500M")
	kubeclient := fake.NewSimpleClientset()
	radixclient := fake2.NewSimpleClientset()
	prometheusClient := prometheusfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeclient, radixclient, fake3.NewSimpleClientset())
	return radixclient, kubeclient, prometheusClient, kubeUtil
}

func (params *TestParams) ApplyRd(kubeUtil *kube.Kube) *v1.RadixDeployment {
	envVarsConfigMap, envVarsMetadataConfigMap, _ := kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(params.Namespace, params.AppName, params.JobComponentName)
	envVarsConfigMap.Data = params.EnvVarsConfigMapData
	metadataMap := make(map[string]kube.EnvVarMetadata)
	for name, value := range params.EnvVarsMetadataConfigMapData {
		metadataMap[name] = kube.EnvVarMetadata{RadixConfigValue: value}
	}
	kube.SetEnvVarsMetadataMapToConfigMap(envVarsMetadataConfigMap, metadataMap)
	kubeUtil.UpdateConfigMap(params.Namespace, envVarsConfigMap, envVarsMetadataConfigMap)

	rd := utils.ARadixDeployment().
		WithDeploymentName(params.DeploymentName).
		WithAppName(params.AppName).
		WithEnvironment(params.Environment).
		WithComponents().
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().
				WithName(params.JobComponentName).
				WithTimeLimitSeconds(numbers.Int64Ptr(10)).
				WithPayloadPath(radixUtils.StringPtr("payload-path")).
				WithEnvironmentVariables(params.RadixConfigEnvVarsMap),
		).
		BuildRD()
	kubeUtil.RadixClient().RadixV1().RadixDeployments(rd.Namespace).Create(context.Background(), rd, metav1.CreateOptions{})
	return rd
}

type TestParams struct {
	AppName                      string
	Environment                  string
	Namespace                    string
	JobComponentName             string
	DeploymentName               string
	JobName                      string
	RadixConfigEnvVarsMap        map[string]string
	EnvVarsConfigMapData         map[string]string
	EnvVarsMetadataConfigMapData map[string]string
}

func GetTestParams() *TestParams {
	params := TestParams{
		AppName:                      "app",
		Environment:                  "qa",
		JobComponentName:             "compute",
		DeploymentName:               "app-deploy-1",
		JobName:                      "some-job",
		RadixConfigEnvVarsMap:        make(map[string]string),
		EnvVarsConfigMapData:         make(map[string]string),
		EnvVarsMetadataConfigMapData: make(map[string]string),
	}
	params.Namespace = utils.GetEnvironmentNamespace(params.AppName, params.Environment)
	return &params
}

func (params *TestParams) WithRadixConfigEnvVarsMap(envVars map[string]string) *TestParams {
	params.RadixConfigEnvVarsMap = envVars
	return params
}

func (params *TestParams) WithEnvVarsConfigMapData(envVars map[string]string) *TestParams {
	params.EnvVarsConfigMapData = envVars
	return params
}

func (params *TestParams) WithEnvVarsMetadataConfigMapData(envVars map[string]string) *TestParams {
	params.EnvVarsMetadataConfigMapData = envVars
	return params
}
