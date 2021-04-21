package job

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func setupTest(appName, appEnvironment, appComponent, appDeployment string, historyLimit int) (radixclient.Interface, kubernetes.Interface, *kube.Kube) {
	os.Setenv("RADIX_APP", appName)
	os.Setenv("RADIX_ENVIRONMENT", appEnvironment)
	os.Setenv("RADIX_COMPONENT", appComponent)
	os.Setenv("RADIX_DEPLOYMENT", appDeployment)
	os.Setenv("RADIX_JOB_SCHEDULERS_PER_ENVIRONMENT_HISTORY_LIMIT", fmt.Sprint(historyLimit))
	os.Setenv(defaults.OperatorEnvLimitDefaultCPUEnvironmentVariable, "200m")
	os.Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "500M")
	kubeclient := kubefake.NewSimpleClientset()
	radixclient := radixfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeclient, radixclient)
	return radixclient, kubeclient, kubeUtil
}

func addKubeJob(kubeClient kubernetes.Interface, jobName, componentName, jobType, namespace string) {
	labels := make(map[string]string)

	if len(strings.TrimSpace(componentName)) > 0 {
		labels[kube.RadixComponentLabel] = componentName
	}

	if len(strings.TrimSpace(jobType)) > 0 {
		labels[kube.RadixJobTypeLabel] = jobType
	}

	kubeClient.BatchV1().Jobs(namespace).Create(
		context.TODO(),
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:   jobName,
				Labels: labels,
			},
		},
		metav1.CreateOptions{},
	)
}

func TestGetJobs(t *testing.T) {
	appName, appEnvironment, appComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
	appNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
	radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appComponent, appDeployment, 1)
	addKubeJob(kubeClient, "job1", appComponent, kube.RadixJobTypeJobSchedule, appNamespace)
	addKubeJob(kubeClient, "job2", appComponent, kube.RadixJobTypeJobSchedule, appNamespace)
	addKubeJob(kubeClient, "job3", "other-component", kube.RadixJobTypeJobSchedule, appNamespace)
	addKubeJob(kubeClient, "job4", appComponent, "other-type", appNamespace)
	addKubeJob(kubeClient, "job5", appComponent, kube.RadixJobTypeJobSchedule, "app-other")

	handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
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
		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appComponent, appDeployment, 1)
		addKubeJob(kubeClient, "job1", appComponent, kube.RadixJobTypeJobSchedule, appNamespace)
		addKubeJob(kubeClient, "job2", appComponent, kube.RadixJobTypeJobSchedule, appNamespace)

		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		job1, err := handler.GetJob("job1")
		assert.Nil(t, err)
		assert.Equal(t, "job1", job1.Name)
		job2, err := handler.GetJob("job2")
		assert.Nil(t, err)
		assert.Equal(t, "job2", job2.Name)
	})

	t.Run("job with different component name label", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "a-job"
		appNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appComponent, appDeployment, 1)
		addKubeJob(kubeClient, jobName, "other-component", kube.RadixJobTypeJobSchedule, appNamespace)

		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		job, err := handler.GetJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, errors.ReasonForError(err))
		assert.Nil(t, job)
	})

	t.Run("job with different job type label", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "a-job"
		appNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appComponent, appDeployment, 1)
		addKubeJob(kubeClient, jobName, appComponent, "other-type", appNamespace)

		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		job, err := handler.GetJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, errors.ReasonForError(err))
		assert.Nil(t, job)
	})

	t.Run("job in different app namespace", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "a-job"
		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appComponent, appDeployment, 1)
		addKubeJob(kubeClient, jobName, appComponent, kube.RadixJobTypeJobSchedule, "app-other")

		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		job, err := handler.GetJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, errors.ReasonForError(err))
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
					WithName(appJobComponent),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.Len(t, job.Labels, 3)
		assert.Equal(t, appName, job.Labels[kube.RadixAppLabel])
		assert.Equal(t, appJobComponent, job.Labels[kube.RadixComponentLabel])
		assert.Equal(t, kube.RadixJobTypeJobSchedule, job.Labels[kube.RadixJobTypeLabel])
		assert.Len(t, job.Spec.Template.Labels, 2)
		assert.Equal(t, appJobComponent, job.Spec.Template.Labels[kube.RadixComponentLabel])
		assert.Equal(t, kube.RadixJobTypeJobSchedule, job.Spec.Template.Labels[kube.RadixJobTypeLabel])
		assert.Equal(t, numbers.Int32Ptr(0), job.Spec.BackoffLimit)
		assert.Equal(t, corev1.RestartPolicyNever, job.Spec.Template.Spec.RestartPolicy)
		assert.Equal(t, corev1.PullAlways, job.Spec.Template.Spec.Containers[0].ImagePullPolicy)
	})

	t.Run("RD job image", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, image := "app", "qa", "compute", "app-deploy-1", "image:xyz"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		rd := utils.ARadixDeployment().
			WithDeploymentName(appDeployment).
			WithAppName(appName).
			WithEnvironment(appEnvironment).
			WithComponents().
			WithJobComponents(
				utils.NewDeployJobComponentBuilder().
					WithName(appJobComponent).
					WithImage(image),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.Equal(t, image, job.Spec.Template.Spec.Containers[0].Image)
	})

	t.Run("RD job with env vars", func(t *testing.T) {
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
					WithName(appJobComponent).
					WithEnvironmentVariables(map[string]string{"ENV1": "value1", "ENV2": "value2"}),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		// Test environment variables
		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.Len(t, job.Spec.Template.Spec.Containers[0].Env, 2)
		env1 := getEnvByNameForTest(job.Spec.Template.Spec.Containers[0].Env, "ENV1")
		assert.Equal(t, "value1", env1.Value)
		env2 := getEnvByNameForTest(job.Spec.Template.Spec.Containers[0].Env, "ENV2")
		assert.Equal(t, "value2", env2.Value)
	})

	t.Run("RD job with payload path - secret exists and mounted", func(t *testing.T) {
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
					WithName(appJobComponent).
					WithPayloadPath(&payloadPath),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(&models.JobScheduleDescription{Payload: payloadString})
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		// Test secret spec
		secretName := getPayloadSecretName(jobStatus.Name)
		secret, _ := kubeClient.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
		assert.NotNil(t, secret)
		assert.Len(t, secret.Labels, 4)
		assert.Equal(t, appName, secret.Labels[kube.RadixAppLabel])
		assert.Equal(t, appJobComponent, secret.Labels[kube.RadixComponentLabel])
		assert.Equal(t, kube.RadixJobTypeJobSchedule, secret.Labels[kube.RadixJobTypeLabel])
		assert.Equal(t, jobStatus.Name, secret.Labels[kube.RadixJobNameLabel])
		payloadBytes := secret.Data[JOB_PAYLOAD_PROPERTY_NAME]
		assert.Equal(t, payloadString, string(payloadBytes))
		// Test secret mounted
		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.NotNil(t, job)
		assert.Len(t, job.Spec.Template.Spec.Volumes, 1)
		assert.Equal(t, JOB_PAYLOAD_PROPERTY_NAME, job.Spec.Template.Spec.Volumes[0].Name)
		assert.Equal(t, secretName, job.Spec.Template.Spec.Volumes[0].Secret.SecretName)
		assert.Len(t, job.Spec.Template.Spec.Containers[0].VolumeMounts, 1)
		assert.Equal(t, JOB_PAYLOAD_PROPERTY_NAME, job.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name)
		assert.Equal(t, payloadPath, job.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
	})

	t.Run("RD job without payload path - no secret, no volume mounts", func(t *testing.T) {
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
					WithName(appJobComponent),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(&models.JobScheduleDescription{Payload: payloadString})
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		// Test secret does not exist
		secretName := getPayloadSecretName(jobStatus.Name)
		secret, _ := kubeClient.CoreV1().Secrets(envNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
		assert.Nil(t, secret)
		// Test no volume mounts
		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.NotNil(t, job)
		assert.Len(t, job.Spec.Template.Spec.Volumes, 0)
		assert.Len(t, job.Spec.Template.Spec.Containers[0].VolumeMounts, 0)
	})

	t.Run("RD job with ports - service created", func(t *testing.T) {
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
					WithName(appJobComponent).
					WithPort("http", 8000),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		// Test service spec
		service, _ := kubeClient.CoreV1().Services(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.NotNil(t, service)
		assert.Len(t, service.Labels, 4)
		assert.Equal(t, appName, service.Labels[kube.RadixAppLabel])
		assert.Equal(t, appJobComponent, service.Labels[kube.RadixComponentLabel])
		assert.Equal(t, kube.RadixJobTypeJobSchedule, service.Labels[kube.RadixJobTypeLabel])
		assert.Equal(t, jobStatus.Name, service.Labels[kube.RadixJobNameLabel])
		assert.Len(t, service.Spec.Selector, 1)
		assert.Equal(t, jobStatus.Name, service.Spec.Selector[kube.RadixJobNameLabel])
		assert.Len(t, service.Spec.Ports, 1)
		assert.Equal(t, "http", service.Spec.Ports[0].Name)
		assert.Equal(t, int32(8000), service.Spec.Ports[0].Port)
	})

	t.Run("RD job without ports - no service created", func(t *testing.T) {
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
					WithName(appJobComponent),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		// Test service spec
		service, _ := kubeClient.CoreV1().Services(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.Nil(t, service)
	})

	t.Run("RD job with resources", func(t *testing.T) {
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
					WithName(appJobComponent).
					WithResource(
						map[string]string{"cpu": "10m", "memory": "20M"},
						map[string]string{"cpu": "30m", "memory": "40M"},
					),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		// Test resources defined
		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.NotNil(t, job)
		assert.Len(t, job.Spec.Template.Spec.Containers, 1)
		assert.Len(t, job.Spec.Template.Spec.Containers[0].Resources.Requests, 2)
		assert.Equal(t, int64(10), job.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().MilliValue())
		assert.Equal(t, int64(20), job.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().ScaledValue(resource.Mega))
		assert.Len(t, job.Spec.Template.Spec.Containers[0].Resources.Limits, 2)
		assert.Equal(t, int64(30), job.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().MilliValue())
		assert.Equal(t, int64(40), job.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().ScaledValue(resource.Mega))
	})

	t.Run("RD job without resources", func(t *testing.T) {
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
					WithName(appJobComponent),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)

		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.NotNil(t, job)
		assert.Len(t, job.Spec.Template.Spec.Containers, 1)
		assert.Len(t, job.Spec.Template.Spec.Containers[0].Resources.Requests, 0)
		assert.Len(t, job.Spec.Template.Spec.Containers[0].Resources.Limits, 0)
	})

	t.Run("RD job with only request resources, not exceeding default limit", func(t *testing.T) {
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
					WithName(appJobComponent).
					WithResource(
						map[string]string{"cpu": "50m", "memory": "60M"},
						map[string]string{},
					),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)

		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.NotNil(t, job)
		assert.Len(t, job.Spec.Template.Spec.Containers, 1)
		assert.Len(t, job.Spec.Template.Spec.Containers[0].Resources.Requests, 2)
		assert.Equal(t, int64(50), job.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().MilliValue())
		assert.Equal(t, int64(60), job.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().ScaledValue(resource.Mega))
		assert.Len(t, job.Spec.Template.Spec.Containers[0].Resources.Limits, 0)
	})

	t.Run("RD job with only request resources, exceeding default limit", func(t *testing.T) {
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
					WithName(appJobComponent).
					WithResource(
						map[string]string{"cpu": "400m", "memory": "600M"},
						map[string]string{},
					),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)

		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.NotNil(t, job)
		assert.Len(t, job.Spec.Template.Spec.Containers, 1)
		assert.Len(t, job.Spec.Template.Spec.Containers[0].Resources.Requests, 2)
		assert.Equal(t, int64(400), job.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().MilliValue())
		assert.Equal(t, int64(600), job.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().ScaledValue(resource.Mega))
		assert.Len(t, job.Spec.Template.Spec.Containers[0].Resources.Limits, 2)
		assert.Equal(t, int64(400), job.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().MilliValue())
		assert.Equal(t, int64(600), job.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().ScaledValue(resource.Mega))
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
					WithName("another-job"),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.Nil(t, jobStatus)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, errors.ReasonForError(err))
		assert.Equal(t, errors.NotFoundMessage("job component", appJobComponent), err.Error())
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
					WithName(appJobComponent),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.Nil(t, jobStatus)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, errors.ReasonForError(err))
		assert.Equal(t, errors.NotFoundMessage("radix deployment", appDeployment), err.Error())
	})

	t.Run("RD job with secrets - env correctly set", func(t *testing.T) {
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
					WithName(appJobComponent).
					WithSecrets([]string{"SECRET1", "SECRET2"}),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.NotNil(t, jobStatus)
		assert.Nil(t, err)
		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.Len(t, job.Spec.Template.Spec.Containers[0].Env, 2)
		env1 := getEnvByNameForTest(job.Spec.Template.Spec.Containers[0].Env, "SECRET1")
		assert.Equal(t, "SECRET1", env1.ValueFrom.SecretKeyRef.Key)
		assert.Equal(t, utils.GetComponentSecretName(appJobComponent), env1.ValueFrom.SecretKeyRef.LocalObjectReference.Name)
		env2 := getEnvByNameForTest(job.Spec.Template.Spec.Containers[0].Env, "SECRET2")
		assert.NotNil(t, env2)
		assert.Equal(t, "SECRET2", env2.ValueFrom.SecretKeyRef.Key)
		assert.Equal(t, utils.GetComponentSecretName(appJobComponent), env2.ValueFrom.SecretKeyRef.LocalObjectReference.Name)
	})

	t.Run("RD job with volume mount", func(t *testing.T) {
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
					WithName(appJobComponent).
					WithVolumeMounts([]v1.RadixVolumeMount{
						{
							Type:      "blob",
							Name:      "blobname",
							Container: "blobcontainer",
							Path:      "/blobpath",
						},
					}),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.NotNil(t, jobStatus)
		assert.Nil(t, err)
		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.Len(t, job.Spec.Template.Spec.Volumes, 1)
		assert.Len(t, job.Spec.Template.Spec.Containers[0].VolumeMounts, 1)
		assert.Equal(t, job.Spec.Template.Spec.Volumes[0].Name, job.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name)
		assert.Equal(t, "/blobpath", job.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath)
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
					WithName(appJobComponent).
					WithNodeGpu("gpu1, gpu2").
					WithNodeGpuCount("2"),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.NotNil(t, jobStatus)
		assert.Nil(t, err)
		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.Len(t, job.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, 1)
		assert.Len(t, job.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions, 2)
		gpu := getNodeSelectorRequirementByKeyForTest(job.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions, kube.RadixGpuLabel)
		assert.Equal(t, corev1.NodeSelectorOpIn, gpu.Operator)
		assert.ElementsMatch(t, gpu.Values, []string{"gpu1", "gpu2"})
		gpuCount := getNodeSelectorRequirementByKeyForTest(job.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions, kube.RadixGpuCountLabel)
		assert.Equal(t, corev1.NodeSelectorOpGt, gpuCount.Operator)
		assert.Equal(t, gpuCount.Values, []string{"1"})
	})

	t.Run("RD job with runAsNonRoot true", func(t *testing.T) {
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
					WithName(appJobComponent).
					WithRunAsNonRoot(true),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.NotNil(t, jobStatus)
		assert.Nil(t, err)
		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.Equal(t, utils.BoolPtr(true), job.Spec.Template.Spec.SecurityContext.RunAsNonRoot)
		assert.Equal(t, utils.BoolPtr(true), job.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot)
		assert.Equal(t, utils.BoolPtr(false), job.Spec.Template.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation)
		assert.Equal(t, utils.BoolPtr(false), job.Spec.Template.Spec.Containers[0].SecurityContext.Privileged)
	})

	t.Run("RD job with runAsNonRoot false", func(t *testing.T) {
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
					WithName(appJobComponent).
					WithRunAsNonRoot(false),
			).
			BuildRD()

		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(context.TODO(), rd, metav1.CreateOptions{})
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.NotNil(t, jobStatus)
		assert.Nil(t, err)
		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(context.TODO(), jobStatus.Name, metav1.GetOptions{})
		assert.Equal(t, utils.BoolPtr(false), job.Spec.Template.Spec.SecurityContext.RunAsNonRoot)
		assert.Equal(t, utils.BoolPtr(false), job.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot)
		assert.Equal(t, utils.BoolPtr(false), job.Spec.Template.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation)
		assert.Equal(t, utils.BoolPtr(false), job.Spec.Template.Spec.Containers[0].SecurityContext.Privileged)
	})
}

func TestDeleteJob(t *testing.T) {
	t.Run("delete job - cleanup resources for job", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		createJobForTest(jobName, appJobComponent, kube.RadixJobTypeJobSchedule, envNamespace, kubeClient)
		createJobForTest("another-job", appJobComponent, kube.RadixJobTypeJobSchedule, envNamespace, kubeClient)
		createSecretForTest("secret1", jobName, appJobComponent, envNamespace, kubeClient)
		createSecretForTest("secret2", jobName, appJobComponent, envNamespace, kubeClient)
		createSecretForTest("secret3", "other-job", appJobComponent, envNamespace, kubeClient)
		createSecretForTest("secret4", jobName, "other-job-component", envNamespace, kubeClient)
		createSecretForTest("secret5", jobName, appJobComponent, "other-ns", kubeClient)
		createServiceForTest("service1", jobName, appJobComponent, envNamespace, kubeClient)
		createServiceForTest("service2", jobName, appJobComponent, envNamespace, kubeClient)
		createServiceForTest("service3", "other-job", appJobComponent, envNamespace, kubeClient)
		createServiceForTest("service4", jobName, "other-job-component", envNamespace, kubeClient)
		createServiceForTest("service5", jobName, appJobComponent, "other-ns", kubeClient)

		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		err := handler.DeleteJob(jobName)
		assert.Nil(t, err)
		jobs, _ := kubeClient.BatchV1().Jobs("").List(context.TODO(), metav1.ListOptions{})
		assert.Len(t, jobs.Items, 1)
		assert.NotNil(t, getJobByNameForTest(jobs.Items, "another-job"))
		secrets, _ := kubeClient.CoreV1().Secrets("").List(context.Background(), metav1.ListOptions{})
		assert.Len(t, secrets.Items, 3)
		assert.NotNil(t, getSecretByNameForTest(secrets.Items, "secret3"))
		assert.NotNil(t, getSecretByNameForTest(secrets.Items, "secret4"))
		assert.NotNil(t, getSecretByNameForTest(secrets.Items, "secret5"))
		services, _ := kubeClient.CoreV1().Services("").List(context.Background(), metav1.ListOptions{})
		assert.Len(t, services.Items, 3)
		assert.NotNil(t, getServiceByNameForTest(services.Items, "service3"))
		assert.NotNil(t, getServiceByNameForTest(services.Items, "service4"))
		assert.NotNil(t, getServiceByNameForTest(services.Items, "service5"))
	})

	t.Run("delete job - job name does not exist", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		createJobForTest("another-job", appJobComponent, kube.RadixJobTypeJobSchedule, envNamespace, kubeClient)

		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		err := handler.DeleteJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, errors.ReasonForError(err))
	})

	t.Run("delete job - another job component name", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		createJobForTest(jobName, "another-job-component", kube.RadixJobTypeJobSchedule, envNamespace, kubeClient)

		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		err := handler.DeleteJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, errors.ReasonForError(err))
	})

	t.Run("delete job - another job type", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "job1"
		envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		createJobForTest(jobName, appJobComponent, "another-type", envNamespace, kubeClient)

		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		err := handler.DeleteJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, errors.ReasonForError(err))
	})

	t.Run("delete job - another namespace", func(t *testing.T) {
		t.Parallel()
		appName, appEnvironment, appJobComponent, appDeployment, jobName := "app", "qa", "compute", "app-deploy-1", "job1"
		radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 1)
		createJobForTest(jobName, appJobComponent, kube.RadixJobTypeJobSchedule, "another-ns", kubeClient)

		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		err := handler.DeleteJob(jobName)
		assert.NotNil(t, err)
		assert.Equal(t, models.StatusReasonNotFound, errors.ReasonForError(err))
	})
}

func TestMaintainHistoryLimit(t *testing.T) {
	appName, appEnvironment, appJobComponent, appDeployment := "app", "qa", "compute", "app-deploy-1"
	envNamespace := utils.GetEnvironmentNamespace(appName, appEnvironment)
	radixClient, kubeClient, kubeUtil := setupTest(appName, appEnvironment, appJobComponent, appDeployment, 2)

	createJobForTest("running1", appJobComponent, kube.RadixJobTypeJobSchedule, envNamespace, kubeClient)
	createJobForTest("running2", appJobComponent, kube.RadixJobTypeJobSchedule, envNamespace, kubeClient)
	createJobForTest("running3", appJobComponent, kube.RadixJobTypeJobSchedule, envNamespace, kubeClient)

	createJobForTest("failed1", appJobComponent, kube.RadixJobTypeJobSchedule, envNamespace, kubeClient, SetJobFailedTestFunc,
		SetJobCreatedTimeTestFunc(metav1.NewTime(time.Date(2020, 1, 4, 0, 0, 0, 0, time.UTC))))
	createJobForTest("failed2", appJobComponent, kube.RadixJobTypeJobSchedule, envNamespace, kubeClient, SetJobFailedTestFunc,
		SetJobCreatedTimeTestFunc(metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))))
	createJobForTest("failed3", appJobComponent, kube.RadixJobTypeJobSchedule, envNamespace, kubeClient, SetJobFailedTestFunc,
		SetJobCreatedTimeTestFunc(metav1.NewTime(time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC))))
	createJobForTest("failed4", appJobComponent, kube.RadixJobTypeJobSchedule, envNamespace, kubeClient, SetJobFailedTestFunc,
		SetJobCreatedTimeTestFunc(metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC))))

	createJobForTest("succeeded1", appJobComponent, kube.RadixJobTypeJobSchedule, envNamespace, kubeClient, SetJobSucceededTestFunc,
		SetJobCompletionTimeTestFunc(metav1.NewTime(time.Date(2020, 1, 4, 0, 0, 0, 0, time.UTC))))
	createJobForTest("succeeded2", appJobComponent, kube.RadixJobTypeJobSchedule, envNamespace, kubeClient, SetJobSucceededTestFunc,
		SetJobCompletionTimeTestFunc(metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))))
	createJobForTest("succeeded3", appJobComponent, kube.RadixJobTypeJobSchedule, envNamespace, kubeClient, SetJobSucceededTestFunc,
		SetJobCompletionTimeTestFunc(metav1.NewTime(time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC))))
	createJobForTest("succeeded4", appJobComponent, kube.RadixJobTypeJobSchedule, envNamespace, kubeClient, SetJobSucceededTestFunc,
		SetJobCompletionTimeTestFunc(metav1.NewTime(time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC))))

	handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
	handler.MaintainHistoryLimit()
	jobs, _ := kubeClient.BatchV1().Jobs("").List(context.TODO(), metav1.ListOptions{})
	assert.Len(t, jobs.Items, 7)
	assert.NotNil(t, getJobByNameForTest(jobs.Items, "running1"))
	assert.NotNil(t, getJobByNameForTest(jobs.Items, "running2"))
	assert.NotNil(t, getJobByNameForTest(jobs.Items, "running3"))
	assert.NotNil(t, getJobByNameForTest(jobs.Items, "failed1"))
	assert.NotNil(t, getJobByNameForTest(jobs.Items, "failed3"))
	assert.NotNil(t, getJobByNameForTest(jobs.Items, "succeeded1"))
	assert.NotNil(t, getJobByNameForTest(jobs.Items, "succeeded3"))
}

func SetJobSucceededTestFunc(job *batchv1.Job) {
	job.Status.Succeeded = 1
}

func SetJobFailedTestFunc(job *batchv1.Job) {
	job.Status.Failed = 1
}

func SetJobCreatedTimeTestFunc(createdTime metav1.Time) func(*batchv1.Job) {
	return func(job *batchv1.Job) {
		job.CreationTimestamp = createdTime
	}
}

func SetJobCompletionTimeTestFunc(completionTime metav1.Time) func(*batchv1.Job) {
	return func(job *batchv1.Job) {
		job.Status.CompletionTime = &completionTime
	}
}

func createJobForTest(name, jobComponentLabel, jobTypeLabel, namespace string, kubeClient kubernetes.Interface, jobFormatter ...func(*batchv1.Job)) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				kube.RadixComponentLabel: jobComponentLabel,
				kube.RadixJobTypeLabel:   jobTypeLabel,
			},
		},
	}

	for _, formatter := range jobFormatter {
		formatter(job)
	}

	kubeClient.BatchV1().Jobs(namespace).Create(
		context.Background(),
		job,
		metav1.CreateOptions{},
	)
}

func createSecretForTest(name, jobNameLabel, jobComponentLabel, namespace string, kubeClient kubernetes.Interface) {
	kubeClient.CoreV1().Secrets(namespace).Create(
		context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					kube.RadixComponentLabel: jobComponentLabel,
					kube.RadixJobNameLabel:   jobNameLabel,
				},
			},
		},
		metav1.CreateOptions{},
	)
}

func createServiceForTest(name, jobNameLabel, jobComponentLabel, namespace string, kubeClient kubernetes.Interface) {
	kubeClient.CoreV1().Services(namespace).Create(
		context.Background(),
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					kube.RadixComponentLabel: jobComponentLabel,
					kube.RadixJobNameLabel:   jobNameLabel,
				},
			},
		},
		metav1.CreateOptions{},
	)
}

func getJobStatusByNameForTest(jobs []models.JobStatus, name string) *models.JobStatus {
	for _, job := range jobs {
		if job.Name == name {
			return &job
		}
	}
	return nil
}

func getNodeSelectorRequirementByKeyForTest(requirements []corev1.NodeSelectorRequirement, key string) *corev1.NodeSelectorRequirement {
	for _, requirement := range requirements {
		if requirement.Key == key {
			return &requirement
		}
	}
	return nil
}

func getEnvByNameForTest(envs []corev1.EnvVar, name string) *corev1.EnvVar {
	for _, env := range envs {
		if env.Name == name {
			return &env
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

func getServiceByNameForTest(services []corev1.Service, name string) *corev1.Service {
	for _, service := range services {
		if service.Name == name {
			return &service
		}
	}

	return nil
}

func getJobByNameForTest(jobs []batchv1.Job, name string) *batchv1.Job {
	for _, job := range jobs {
		if job.Name == name {
			return &job
		}
	}

	return nil
}
