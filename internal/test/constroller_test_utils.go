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
	"strings"
	"time"

	radixutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/numbers"
	"github.com/equinor/radix-job-scheduler/api/controllers"
	"github.com/equinor/radix-job-scheduler/internal/config"
	"github.com/equinor/radix-job-scheduler/internal/router"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ControllerTestUtils struct {
	controllers []controllers.Controller
}

func NewControllerTestUtils(controllers ...controllers.Controller) ControllerTestUtils {
	return ControllerTestUtils{
		controllers: controllers,
	}
}

// ExecuteRequest Helper method to issue a http request
func (ctrl *ControllerTestUtils) ExecuteRequest(ctx context.Context, method, path string) <-chan *http.Response {
	return ctrl.ExecuteRequestWithBody(ctx, method, path, nil)
}

// ExecuteRequestWithBody Helper method to issue a http request with body
func (ctrl *ControllerTestUtils) ExecuteRequestWithBody(ctx context.Context, method, path string, body interface{}) <-chan *http.Response {
	responseChan := make(chan *http.Response)

	go func() {
		var reader io.Reader

		if body != nil {
			payload, _ := json.Marshal(body)
			reader = bytes.NewReader(payload)
		}

		serverRouter := router.NewServer(config.NewConfigFromEnv(), ctrl.controllers...)
		server := httptest.NewServer(serverRouter)
		defer server.Close()
		serverUrl := buildURLFromServer(server, path)
		request, err := http.NewRequestWithContext(ctx, method, serverUrl, reader)
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

func AddRadixBatch(radixClient radixclient.Interface, jobName, componentName string, batchJobType kube.RadixBatchType, namespace string) *radixv1.RadixBatch {
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
		&radixv1.RadixBatch{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchName,
				Labels: labels,
			},
			Spec: radixv1.RadixBatchSpec{
				Jobs: []radixv1.RadixBatchJob{
					{
						Name: batchJobName,
						PayloadSecretRef: &radixv1.PayloadSecretKeySelector{
							LocalObjectReference: radixv1.LocalObjectReference{Name: jobName},
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
	twoDaysAgo := time.Now().Add(-50 * time.Hour).Local()
	_, err := kubeClient.CoreV1().Secrets(namespace).Create(
		context.TODO(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
				Labels: radixLabels.Merge(
					radixLabels.ForApplicationName(appName),
					radixLabels.ForComponentName(radixJobComponentName),
					radixLabels.ForBatchName(batchName),
					radixLabels.ForJobScheduleJobType(),
					radixLabels.ForRadixSecretType(kube.RadixSecretJobPayload),
				),
				CreationTimestamp: metav1.NewTime(twoDaysAgo),
			},
			Data: map[string][]byte{batchJobName: []byte("secret")},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		panic(err)
	}
}

func GetSecretByNameForTest(secrets []corev1.Secret, name string) *corev1.Secret {
	for _, secret := range secrets {
		if secret.Name == name {
			return &secret
		}
	}
	return nil
}

func GetRadixBatchByNameForTest(radixBatches []radixv1.RadixBatch, jobName string) *radixv1.RadixBatch {
	batchName, _, _ := ParseBatchAndJobNameFromScheduledJobName(jobName)
	for _, radixBatch := range radixBatches {
		if radixBatch.Name == batchName {
			return &radixBatch
		}
	}
	return nil
}

func (params *TestParams) ApplyRd(kubeUtil *kube.Kube) *radixv1.RadixDeployment {
	envVarsConfigMap, envVarsMetadataConfigMap, _ := kubeUtil.GetOrCreateEnvVarsConfigMapAndMetadataMap(context.Background(), params.Namespace, params.AppName, params.JobComponentName)
	envVarsConfigMap.Data = params.EnvVarsConfigMapData
	metadataMap := make(map[string]kube.EnvVarMetadata)
	for name, value := range params.EnvVarsMetadataConfigMapData {
		metadataMap[name] = kube.EnvVarMetadata{RadixConfigValue: value}
	}
	_ = kube.SetEnvVarsMetadataMapToConfigMap(envVarsMetadataConfigMap, metadataMap)

	for _, cm := range []*corev1.ConfigMap{envVarsConfigMap, envVarsMetadataConfigMap} {
		_, _ = kubeUtil.KubeClient().CoreV1().ConfigMaps(params.Namespace).Update(context.Background(), cm, metav1.UpdateOptions{})
	}

	rd := utils.ARadixDeployment().
		WithDeploymentName(params.DeploymentName).
		WithAppName(params.AppName).
		WithEnvironment(params.Environment).
		WithComponents().
		WithJobComponents(
			utils.NewDeployJobComponentBuilder().
				WithName(params.JobComponentName).
				WithTimeLimitSeconds(numbers.Int64Ptr(10)).
				WithPayloadPath(radixutils.StringPtr("payload-path")).
				WithEnvironmentVariables(params.RadixConfigEnvVarsMap),
		).
		BuildRD()
	_, _ = kubeUtil.RadixClient().RadixV1().RadixDeployments(rd.Namespace).Create(context.TODO(), rd, metav1.CreateOptions{})
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
	RadixDeployComponent         utils.DeployComponentBuilder
	RadixDeployJobComponent      utils.DeployJobComponentBuilder
}

func GetTestParams() *TestParams {
	appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
	params := TestParams{
		AppName:                      appName,
		Environment:                  appEnvironment,
		JobComponentName:             "compute",
		DeploymentName:               appDeployment,
		JobName:                      "some-job",
		RadixConfigEnvVarsMap:        make(map[string]string),
		EnvVarsConfigMapData:         make(map[string]string),
		EnvVarsMetadataConfigMapData: make(map[string]string),
		RadixDeployJobComponent:      utils.NewDeployJobComponentBuilder().WithName(appJobComponent),
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

type RequestContextMatcher struct {
}

func (c RequestContextMatcher) Matches(x interface{}) bool {
	_, ok := x.(context.Context)
	return ok
}

func (c RequestContextMatcher) String() string {
	return "is context"
}
