package job

import (
	"errors"
	"fmt"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strings"
	"time"
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
	newJob, _ := createJob(jh.kube, radixDeployment, jobScheduleDescription)
	job, err := kubeClient.BatchV1().Jobs(namespace).Create(newJob)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, errors.New(fmt.Sprintf("not created job for component %s, environment %s, in namespace: %s", jobScheduleDescription.ComponentName, jobScheduleDescription.Environment, namespace))
	}
	log.Debugf("found Job %s for namespace: %s", newJob.Name, namespace)
	return nil, nil
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

func createJob(kubeutil *kube.Kube, rd *radixv1.RadixDeployment, jobDesc *models.JobScheduleDescription) (*batchv1.Job, error) {
	appName := rd.Spec.AppName
	timestamp := time.Now().Format("20060102150405")
	jobTag := strings.ToLower(utils.RandString(8))
	componentName := jobDesc.ComponentName
	//environment := jobDesc.Environment
	radixComponent := getRadixJobComponentBy(rd, componentName)
	if radixComponent == nil {
		return nil, errors.New(fmt.Sprintf("radix job component %s not found in Radix Deployment %s", componentName, rd.Name))
	}
	//imageTag := radixComponent.Image
	jobName := fmt.Sprintf("%s-%s-%s", componentName, timestamp, jobTag)

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				kube.RadixJobNameLabel:   jobName,
				"radix-app-name":         appName, // For backwards compatibility. Remove when cluster is migrated
				kube.RadixAppLabel:       appName,
				kube.RadixComponentLabel: componentName,
				kube.RadixJobTypeLabel:   "job-schedule", //TODO: kube.RadixJobTypeJobSchedule
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						kube.RadixJobNameLabel: jobName,
					},
					Namespace: rd.ObjectMeta.Namespace,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: "Never",
					//InitContainers: initContainers,
					Containers: getContainers(kubeutil, rd, radixComponent),
					Volumes: []corev1.Volume{
						{
							Name: git.BuildContextVolumeName,
						},
						{
							Name: git.GitSSHKeyVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: git.GitSSHKeyVolumeName,
									//DefaultMode: &defaultMode,
								},
							},
						},
						{
							//Name: azureServicePrincipleSecretName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									//SecretName: azureServicePrincipleSecretName,
								},
							},
						},
					},
				},
			},
		},
	}
	return &job, nil
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
	environmentVariables := deployment.GetEnvironmentVariablesFromRadixDeployJobComponent("", kube, rd, radixJobComponent)
	container := corev1.Container{
		Name:            radixJobComponent.Name,
		Image:           radixJobComponent.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env:             environmentVariables,
		VolumeMounts:    deployment.GetRadixDeployJobComponentVolumeMounts(radixJobComponent),
	}
	return []corev1.Container{container}
}
