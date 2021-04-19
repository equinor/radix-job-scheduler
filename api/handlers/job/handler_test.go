package job

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/batch/v1"
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

	j, err := kubeClient.BatchV1().Jobs(namespace).Create(
		&v1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:   jobName,
				Labels: labels,
			},
		},
	)

	fmt.Println(j)
	fmt.Println(err)
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
	job1 := getJobStatusByName(jobs, "job1")
	assert.NotNil(t, job1)
	job2 := getJobStatusByName(jobs, "job2")
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

	// RD job without ports - service not created
	// RD job with resources
	// RD job validate job and container labels, image, pull policy, environment variables, restart policy, security context
	// RD does not exist - notfound error
	// RD does not container job name - notfound error

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
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(rd)
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(&models.JobScheduleDescription{Payload: payloadString})
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		// Test secret spec
		secretName := getPayloadSecretName(jobStatus.Name)
		secret, _ := kubeClient.CoreV1().Secrets(envNamespace).Get(secretName, metav1.GetOptions{})
		assert.NotNil(t, secret)
		assert.Len(t, secret.Labels, 4)
		assert.Equal(t, appName, secret.Labels[kube.RadixAppLabel])
		assert.Equal(t, appJobComponent, secret.Labels[kube.RadixComponentLabel])
		assert.Equal(t, kube.RadixJobTypeJobSchedule, secret.Labels[kube.RadixJobTypeLabel])
		assert.Equal(t, jobStatus.Name, secret.Labels[kube.RadixJobNameLabel])
		payloadBytes := secret.Data[JOB_PAYLOAD_PROPERTY_NAME]
		assert.Equal(t, payloadString, string(payloadBytes))
		// Test secret mounted
		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(jobStatus.Name, metav1.GetOptions{})
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
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(rd)
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(&models.JobScheduleDescription{Payload: payloadString})
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		// Test secret does not exist
		secretName := getPayloadSecretName(jobStatus.Name)
		secret, _ := kubeClient.CoreV1().Secrets(envNamespace).Get(secretName, metav1.GetOptions{})
		assert.Nil(t, secret)
		// Test no volume mounts
		job, _ := kubeClient.BatchV1().Jobs(envNamespace).Get(jobStatus.Name, metav1.GetOptions{})
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
		radixClient.RadixV1().RadixDeployments(envNamespace).Create(rd)
		handler := New(models.NewEnv(), kubeUtil, kubeClient, radixClient)
		jobStatus, err := handler.CreateJob(nil)
		assert.Nil(t, err)
		assert.NotNil(t, jobStatus)
		// Test service spec
		service, _ := kubeClient.CoreV1().Services(envNamespace).Get(jobStatus.Name, metav1.GetOptions{})
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

}

func getJobStatusByName(jobs []models.JobStatus, name string) *models.JobStatus {
	for _, job := range jobs {
		if job.Name == name {
			return &job
		}
	}
	return nil
}
