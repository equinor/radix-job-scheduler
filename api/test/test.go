package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"

	radixUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-job-scheduler/models"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-job-scheduler/router"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixclientfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	secretstoragefake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

type ControllerTestUtils struct {
	controllers []models.Controller
}

func New(controllers ...models.Controller) ControllerTestUtils {
	return ControllerTestUtils{
		controllers: controllers,
	}
}

// ExecuteRequest Helper method to issue a http request
func (ctrl *ControllerTestUtils) ExecuteRequest(method, path string) <-chan *http.Response {
	return ctrl.ExecuteRequestWithBody(method, path, nil)
}

// ExecuteRequestWithBody Helper method to issue a http request with body
func (ctrl *ControllerTestUtils) ExecuteRequestWithBody(method, path string, body interface{}) <-chan *http.Response {
	responseChan := make(chan *http.Response)

	go func() {
		var reader io.Reader

		if body != nil {
			payload, _ := json.Marshal(body)
			reader = bytes.NewReader(payload)
		}

		serverRouter := router.NewServer(models.NewEnv(), ctrl.controllers...)
		server := httptest.NewServer(serverRouter)
		defer server.Close()
		serverUrl := buildURLFromServer(server, path)
		request, err := http.NewRequest(method, serverUrl, reader)
		if err != nil {
			panic(err)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			panic(err)
		}
		responseChan <- response
		close(responseChan)
	}()

	return responseChan
}

// GetResponseBody Gets response payload as type
func GetResponseBody(response *http.Response, target interface{}) error {
	body, _ := io.ReadAll(response.Body)

	return json.Unmarshal(body, target)
}

func buildURLFromServer(server *httptest.Server, path string) string {
	serverUrl, _ := url.Parse(server.URL)
	serverUrl.Path = path
	return serverUrl.String()
}

func AddRadixBatch(radixClient versioned.Interface, jobName, componentName string, batchJobType kube.RadixBatchType, namespace string) *v1.RadixBatch {
	labels := make(map[string]string)

	if len(strings.TrimSpace(componentName)) > 0 {
		labels[kube.RadixComponentLabel] = componentName
	}
	labels[kube.RadixBatchTypeLabel] = string(batchJobType)

	batchName, batchJobName, ok := ParseBatchAndJobNameFromScheduledJobName(jobName)
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
	batchName, batchJobName, _ := ParseBatchAndJobNameFromScheduledJobName(jobName)
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
	batchName, _, _ := ParseBatchAndJobNameFromScheduledJobName(jobName)
	for _, radixBatch := range radixBatches {
		if radixBatch.Name == batchName {
			return &radixBatch
		}
	}
	return nil
}

func SetupTest(appName, appEnvironment, appComponent, appDeployment string, historyLimit int) (versioned.Interface,
	kubernetes.Interface, prometheusclient.Interface, *kube.Kube) {
	_ = os.Setenv("RADIX_APP", appName)
	_ = os.Setenv("RADIX_ENVIRONMENT", appEnvironment)
	_ = os.Setenv("RADIX_COMPONENT", appComponent)
	_ = os.Setenv("RADIX_DEPLOYMENT", appDeployment)
	_ = os.Setenv("RADIX_JOB_SCHEDULERS_PER_ENVIRONMENT_HISTORY_LIMIT", fmt.Sprint(historyLimit))
	_ = os.Setenv(defaults.OperatorRollingUpdateMaxUnavailable, "25%")
	_ = os.Setenv(defaults.OperatorRollingUpdateMaxSurge, "25%")
	_ = os.Setenv(defaults.OperatorEnvLimitDefaultCPUEnvironmentVariable, "200m")
	_ = os.Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "500M")
	kubeclient := fake.NewSimpleClientset()
	radixclient := radixclientfake.NewSimpleClientset()
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
	_ = kube.SetEnvVarsMetadataMapToConfigMap(envVarsMetadataConfigMap, metadataMap)
	_ = kubeUtil.UpdateConfigMap(params.Namespace, envVarsConfigMap, envVarsMetadataConfigMap)

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
	_, _ = kubeUtil.RadixClient().RadixV1().RadixDeployments(rd.Namespace).Create(context.Background(), rd, metav1.CreateOptions{})
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

func ParseBatchAndJobNameFromScheduledJobName(scheduleJobName string) (batchName, batchJobName string, ok bool) {
	scheduleJobNameParts := strings.Split(scheduleJobName, "-")
	if len(scheduleJobNameParts) < 2 {
		return
	}
	batchName = strings.Join(scheduleJobNameParts[:len(scheduleJobNameParts)-1], "-")
	batchJobName = scheduleJobNameParts[len(scheduleJobNameParts)-1]
	ok = true
	return
}