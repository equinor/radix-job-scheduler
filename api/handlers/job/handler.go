package job

import (
	"errors"
	"fmt"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strings"
	"time"
)

const (
	JOB_PAYLOAD_PROPERTY_NAME          = "payload"
	JOB_SERVICE_ACCOUNT_NAME           = "radix-job-scheduler"
	OBSOLETE_RADIX_APP_NAME_LABEL_NAME = "radix-app-name"
)

type Handler interface {
	GetJobs() (*[]models.JobStatus, error)
	GetJob(name string) (*models.JobStatus, error)
	CreateJob(jobScheduleDescription *models.JobScheduleDescription) (*models.JobStatus, error)
	DeleteJob(jobName string) error
}

type jobHandler struct {
	kube        *kube.Kube
	env         *models.Env
	kubeClient  *kubernetes.Interface
	radixClient *radixclient.Interface
}

func New(env *models.Env, kube *kube.Kube, kubeClient *kubernetes.Interface, radixClient *radixclient.Interface) Handler {
	return &jobHandler{
		kube:        kube,
		kubeClient:  kubeClient,
		radixClient: radixClient,
		env:         env,
	}
}

func (jh *jobHandler) GetJobs() (*[]models.JobStatus, error) {
	kubeClient := *jh.kubeClient
	namespace := jh.env.RadixDeploymentNamespace
	log.Debugf("Get Jobs for namespace: %s", namespace)
	kubeJobs, err := kubeClient.BatchV1().Jobs(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	jobs := make([]models.JobStatus, len(kubeJobs.Items))

	for idx, k8sJob := range kubeJobs.Items {
		jobs[idx] = *models.GetJobStatusFromJob(&k8sJob)
	}
	log.Debugf("Found %v jobs for namespace %s", len(jobs), namespace)
	return &jobs, nil
}

func (jh *jobHandler) GetJob(jobName string) (*models.JobStatus, error) {
	kubeClient := *jh.kubeClient
	namespace := jh.env.RadixDeploymentNamespace
	log.Debugf("get jobs for namespace: %s", namespace)
	job, err := kubeClient.BatchV1().Jobs(namespace).Get(jobName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	log.Debugf("found Job %s for namespace: %s", jobName, namespace)
	jobStatus := models.GetJobStatusFromJob(job)
	return jobStatus, nil
}

func (jh *jobHandler) CreateJob(jobScheduleDescription *models.JobScheduleDescription) (*models.JobStatus, error) {
	kubeClient := *jh.kubeClient
	radixV1 := *jh.radixClient
	namespace := jh.env.RadixDeploymentNamespace
	log.Debugf("create job for namespace: %s", namespace)

	radixDeployment, err := radixV1.RadixV1().RadixDeployments(namespace).Get(jobScheduleDescription.RadixDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("fail to get Radix deployment %s for namespace: %s", jobScheduleDescription.RadixDeploymentName, namespace))
	}

	job, _ := buildJob(jh.kube, radixDeployment, jobScheduleDescription)

	secret := createPayloadSecret(job, jobScheduleDescription)
	_, err = kubeClient.CoreV1().Secrets(namespace).Create(secret)
	if err != nil {
		return nil, err
	}

	createdJob, err := kubeClient.BatchV1().Jobs(namespace).Create(job)
	if err != nil {
		return nil, err
	}

	log.Debug(fmt.Sprintf("created job for component %s, environment %s, in namespace: %s", jobScheduleDescription.ComponentName, radixDeployment.Spec.Environment, namespace))
	return models.GetJobStatusFromJob(createdJob), nil
}

func createPayloadSecret(job *batchv1.Job, jobScheduleDescription *models.JobScheduleDescription) *corev1.Secret {
	secret := corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      getPayloadSecretName(job.Name),
			Namespace: job.Namespace,
		},
		Data: map[string][]byte{
			JOB_PAYLOAD_PROPERTY_NAME: []byte(jobScheduleDescription.Payload),
		},
	}
	return &secret
}

func (jh *jobHandler) DeleteJob(jobName string) error {
	kubeClient := *jh.kubeClient
	namespace := jh.env.RadixDeploymentNamespace
	log.Debugf("delete job %s for namespace: %s", jobName, namespace)
	err := kubeClient.BatchV1().Jobs(namespace).Delete(jobName, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	log.Debugf("deleted job %s for namespace: %s", jobName, namespace)
	return nil
}

func buildJob(kubeutil *kube.Kube, rd *radixv1.RadixDeployment, jobDesc *models.JobScheduleDescription) (*batchv1.Job, error) {
	jobComponentName := jobDesc.ComponentName
	radixJobComponent := getRadixJobComponentBy(rd, jobComponentName)
	if radixJobComponent == nil {
		return nil, errors.New(fmt.Sprintf("radix Job Component %s not found in Radix Deployment %s", jobComponentName, rd.Name))
	}

	appName := rd.Spec.AppName
	namespace := rd.ObjectMeta.Namespace

	job := createJob(appName, namespace, jobComponentName)

	containers := getContainers(kubeutil, rd, radixJobComponent)
	containers[0].VolumeMounts = append(containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      JOB_PAYLOAD_PROPERTY_NAME,
		ReadOnly:  true,
		MountPath: radixJobComponent.Payload.Path,
	})
	volumes := getVolumes(namespace, rd.Spec.Environment, radixJobComponent)
	volumes = append(volumes, *getPayloadSecretVolumeMount(job.Name)...)
	job.Spec.Template.Spec.Volumes = volumes
	job.Spec.Template.Spec.Containers = containers

	return &job, nil
}

func getVolumes(namespace string, environment string, component *radixv1.RadixDeployJobComponent) []corev1.Volume {
	return deployment.GetVolumes(namespace, environment, component.Name, &component.VolumeMounts)
}

func createJob(appName string, namespace string, jobComponentName string) batchv1.Job {
	jobName := getJobName(jobComponentName)
	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				kube.RadixAppLabel:                 appName,
				kube.RadixComponentLabel:           jobComponentName,
				kube.RadixJobTypeLabel:             kube.RadixJobTypeJobSchedule,
				kube.RadixJobNameLabel:             jobName,
				OBSOLETE_RADIX_APP_NAME_LABEL_NAME: appName, // For backwards compatibility. Remove when cluster is migrated
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						kube.RadixComponentLabel: jobName,
					},
					Namespace: namespace,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:                corev1.RestartPolicyOnFailure, //TODO: decide what to do with failed job
					AutomountServiceAccountToken: boolPtr(true),
					ServiceAccountName:           JOB_SERVICE_ACCOUNT_NAME,
				},
			},
		},
	}
}

func getJobName(jobComponentName string) string {
	timestamp := time.Now().Format("20060102150405")
	jobTag := strings.ToLower(utils.RandString(8))
	return fmt.Sprintf("%s-%s-%s", jobComponentName, timestamp, jobTag)
}

func getPayloadSecretVolumeMount(jobName string) *[]corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: JOB_PAYLOAD_PROPERTY_NAME,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: getPayloadSecretName(jobName),
				},
			},
		},
	}
	return &volumes
}

func getPayloadSecretName(jobName string) string {
	return fmt.Sprintf("%s-%s", JOB_PAYLOAD_PROPERTY_NAME, jobName)
}

func boolPtr(value bool) *bool {
	return &value
}

func getRadixJobComponentBy(rd *radixv1.RadixDeployment, componentName string) *radixv1.RadixDeployJobComponent {
	for _, jobComponent := range rd.Spec.Jobs {
		if strings.EqualFold(jobComponent.Name, componentName) {
			return &jobComponent
		}
	}
	return nil
}

func getContainers(kube *kube.Kube, rd *radixv1.RadixDeployment, radixJobComponent *radixv1.RadixDeployJobComponent) []corev1.Container {
	environmentVariables := deployment.GetEnvironmentVariablesFromRadixDeployJobComponent(rd.Spec.AppName, kube, rd, radixJobComponent)
	container := corev1.Container{
		Name:  radixJobComponent.Name,
		Image: radixJobComponent.Image,
		//TODO: add setting to be PullAlways, PullIfNotPresent, PullNever. Image should be pulled on job's InitContainer
		ImagePullPolicy: corev1.PullAlways,
		Env:             environmentVariables,
		VolumeMounts:    deployment.GetRadixDeployJobComponentVolumeMounts(radixJobComponent),
	}
	return []corev1.Container{container}
}
