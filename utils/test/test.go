package test

import (
	"context"
	"fmt"
	"os"
	"strings"

	radixUtils "github.com/equinor/radix-common/utils"
	numbers "github.com/equinor/radix-common/utils/numbers"
	apiv1 "github.com/equinor/radix-job-scheduler/api/v1"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fake2 "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	secretstoragefake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
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
	kubeUtil, _ := kube.New(kubeclient, radixclient, secretstoragefake.NewSimpleClientset())
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

func AddRadixBatch(radixClient versioned.Interface, jobName, componentName string, batchJobType kube.RadixBatchType, namespace string) *v1.RadixBatch {
	labels := make(map[string]string)

	if len(strings.TrimSpace(componentName)) > 0 {
		labels[kube.RadixComponentLabel] = componentName
	}
	labels[kube.RadixBatchTypeLabel] = string(batchJobType)

	batchName, batchJobName, ok := apiv1.ParseBatchAndJobNameFromScheduledJobName(jobName)
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

func CreateSecretForTest(appName, secretName, jobName, radixJobComponentName, namespace string, kubeClient kubernetes.Interface) {
	batchName, batchJobName, _ := apiv1.ParseBatchAndJobNameFromScheduledJobName(jobName)
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

func GetJobStatusByNameForTest(jobs []modelsv1.JobStatus, name string) *modelsv1.JobStatus {
	for _, job := range jobs {
		if strings.HasSuffix(job.Name, "-"+name) {
			return &job
		}
	}
	return nil
}

func GetSecretByNameForTest(secrets []corev1.Secret, name string) *corev1.Secret {
	for _, secret := range secrets {
		if secret.Name == name {
			return &secret
		}
	}
	return nil
}

func GetRadixBatchByNameForTest(radixBatches []v1.RadixBatch, jobName string) *v1.RadixBatch {
	batchName, _, _ := apiv1.ParseBatchAndJobNameFromScheduledJobName(jobName)
	for _, radixBatch := range radixBatches {
		if radixBatch.Name == batchName {
			return &radixBatch
		}
	}
	return nil
}
