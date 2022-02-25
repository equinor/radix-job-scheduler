package jobs

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-job-scheduler/api"
	jobErrors "github.com/equinor/radix-job-scheduler/api/errors"
	schedulerDefaults "github.com/equinor/radix-job-scheduler/defaults"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const (
	radixJobNameEnvironmentVariable = "RADIX_JOB_NAME"
)

type jobModel struct {
	common *api.Model
}

type Job interface {
	//GetJobs Get status of all jobs
	GetJobs() ([]models.JobStatus, error)
	//GetJob Get status of a job
	GetJob(name string) (*models.JobStatus, error)
	//CreateJob Create a job with parameters
	CreateJob(jobScheduleDescription *models.JobScheduleDescription) (*models.JobStatus, error)
	//MaintainHistoryLimit Delete outdated jobs
	MaintainHistoryLimit() error
	//DeleteJob Delete a job
	DeleteJob(jobName string) error
}

//New Constructor for job model
func New(env *models.Env, kube *kube.Kube, kubeClient kubernetes.Interface, radixClient radixclient.Interface) Job {
	return &jobModel{
		common: &api.Model{
			Kube:                   kube,
			KubeClient:             kubeClient,
			RadixClient:            radixClient,
			Env:                    env,
			SecurityContextBuilder: deployment.NewSecurityContextBuilder(true),
		},
	}
}

//GetJobs Get status of all jobs
func (model *jobModel) GetJobs() ([]models.JobStatus, error) {
	log.Debugf("Get Jobs for namespace: %s", model.common.Env.RadixDeploymentNamespace)

	kubeJobs, err := model.getAllJobs()
	if err != nil {
		return nil, err
	}

	pods, err := model.getJobPods("")
	if err != nil {
		return nil, err
	}
	podsMap := getJobPodsMap(pods)
	jobs := make([]models.JobStatus, len(kubeJobs))
	for idx, k8sJob := range kubeJobs {
		jobs[idx] = *GetJobStatusFromJob(model.common.KubeClient, k8sJob, podsMap[k8sJob.Name])
	}

	log.Debugf("Found %v jobs for namespace %s", len(jobs), model.common.Env.RadixDeploymentNamespace)
	return jobs, nil
}

func getJobPodsMap(pods []corev1.Pod) map[string][]corev1.Pod {
	podsMap := make(map[string][]corev1.Pod)
	for _, pod := range pods {
		jobName := pod.Labels[schedulerDefaults.K8sJobNameLabel]
		if len(jobName) > 0 {
			podsMap[jobName] = append(podsMap[jobName], pod)
		}
	}
	return podsMap
}

//GetJob Get status of a job
func (model *jobModel) GetJob(jobName string) (*models.JobStatus, error) {
	log.Debugf("get jobs for namespace: %s", model.common.Env.RadixDeploymentNamespace)
	job, err := model.getJobByName(jobName)
	if err != nil {
		return nil, err
	}
	log.Debugf("found Job %s for namespace: %s", jobName, model.common.Env.RadixDeploymentNamespace)
	pods, err := model.getJobPods(job.Name)
	if err != nil {
		return nil, err
	}
	jobStatus := GetJobStatusFromJob(model.common.KubeClient, job, pods)
	return jobStatus, nil
}

//CreateJob Create a job with parameters
func (model *jobModel) CreateJob(jobScheduleDescription *models.JobScheduleDescription) (*models.JobStatus, error) {
	log.Debugf("create job for namespace: %s", model.common.Env.RadixDeploymentNamespace)

	radixDeployment, err := model.common.RadixClient.RadixV1().RadixDeployments(model.common.Env.RadixDeploymentNamespace).Get(context.TODO(), model.common.Env.RadixDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, jobErrors.NewNotFound("radix deployment", model.common.Env.RadixDeploymentName)
	}

	jobComponent := radixDeployment.GetJobComponentByName(model.common.Env.RadixComponentName)
	if jobComponent == nil {
		return nil, jobErrors.NewNotFound("job component", model.common.Env.RadixComponentName)
	}

	jobName := generateJobName(jobComponent)

	payloadSecret, err := model.common.CreatePayloadSecret(jobName, jobComponent, radixDeployment,
		jobScheduleDescription)
	if err != nil {
		return nil, jobErrors.NewFromError(err)
	}

	if err = model.common.CreateService(jobName, jobComponent, radixDeployment); err != nil {
		return nil, jobErrors.NewFromError(err)
	}

	job, err := model.createJob(jobName, jobComponent, radixDeployment, payloadSecret, jobScheduleDescription)
	if err != nil {
		return nil, jobErrors.NewFromError(err)
	}

	log.Debug(fmt.Sprintf("created job %s for component %s, environment %s, in namespace: %s", job.Name, model.common.Env.RadixComponentName, radixDeployment.Spec.Environment, model.common.Env.RadixDeploymentNamespace))
	return GetJobStatusFromJob(model.common.KubeClient, job, nil), nil
}

//DeleteJob Delete a job
func (model *jobModel) DeleteJob(jobName string) error {
	log.Debugf("delete job %s for namespace: %s", jobName, model.common.Env.RadixDeploymentNamespace)
	return model.garbageCollectJob(jobName)
}

//MaintainHistoryLimit Delete outdated jobs
func (model *jobModel) MaintainHistoryLimit() error {
	jobList, err := model.getAllJobs()
	if err != nil {
		return err
	}

	log.Debug("maintain history limit for succeeded jobs")
	succeededJobs := jobList.Where(func(j *batchv1.Job) bool { return j.Status.Succeeded > 0 })
	if err = model.maintainHistoryLimitForJobs(succeededJobs, model.common.Env.RadixJobSchedulersPerEnvironmentHistoryLimit); err != nil {
		return err
	}

	log.Debug("maintain history limit for failed jobs")
	failedJobs := jobList.Where(func(j *batchv1.Job) bool { return j.Status.Failed > 0 })
	if err = model.maintainHistoryLimitForJobs(failedJobs, model.common.Env.RadixJobSchedulersPerEnvironmentHistoryLimit); err != nil {
		return err
	}

	return nil
}

func (model *jobModel) maintainHistoryLimitForJobs(jobs []*batchv1.Job, historyLimit int) error {
	numToDelete := len(jobs) - historyLimit
	if numToDelete <= 0 {
		log.Debug("no history jobs to delete")
		return nil
	}
	log.Debugf("history jobs to delete: %v", numToDelete)

	sortedJobs := sortRJSchByCompletionTimeAsc(jobs)
	for i := 0; i < numToDelete; i++ {
		job := sortedJobs[i]
		log.Debugf("deleting job %s", job.Name)
		if err := model.garbageCollectJob(job.Name); err != nil {
			return err
		}
	}
	return nil
}

func (model *jobModel) garbageCollectJob(jobName string) (err error) {
	job, err := model.getJobByName(jobName)
	if err != nil {
		return
	}

	secrets, err := model.common.GetSecretsForJob(jobName)
	if err != nil {
		return
	}

	for _, secret := range secrets.Items {
		if err = model.common.DeleteSecret(&secret); err != nil {
			return
		}
	}

	services, err := model.common.GetServiceForJob(jobName)
	if err != nil {
		return
	}

	for _, service := range services.Items {
		if err = model.common.DeleteService(&service); err != nil {
			return
		}
	}

	err = model.deleteJob(job)
	if err != nil {
		return err
	}

	return
}

func sortRJSchByCompletionTimeAsc(jobs []*batchv1.Job) []*batchv1.Job {
	sort.Slice(jobs, func(i, j int) bool {
		job1 := (jobs)[i]
		job2 := (jobs)[j]
		return isRJS1CompletedBeforeRJS2(job1, job2)
	})
	return jobs
}

func isRJS1CompletedBeforeRJS2(job1 *batchv1.Job, job2 *batchv1.Job) bool {
	rd1ActiveFrom := getCompletionTimeFrom(job1)
	rd2ActiveFrom := getCompletionTimeFrom(job2)

	return rd1ActiveFrom.Before(rd2ActiveFrom)
}

func getCompletionTimeFrom(job *batchv1.Job) *metav1.Time {
	if job.Status.CompletionTime.IsZero() {
		return &job.CreationTimestamp
	}
	return job.Status.CompletionTime
}

func generateJobName(jobComponent *radixv1.RadixDeployJobComponent) string {
	timestamp := time.Now().Format("20060102150405")
	jobTag := strings.ToLower(commonUtils.RandString(8))
	return fmt.Sprintf("%s-%s-%s", jobComponent.Name, timestamp, jobTag)
}

func (model *jobModel) createJob(jobName string, jobComponent *radixv1.RadixDeployJobComponent, rd *radixv1.RadixDeployment, payloadSecret *corev1.Secret, jobScheduleDescription *models.JobScheduleDescription) (*batchv1.Job, error) {
	var jobComponentConfig *models.RadixJobComponentConfig
	if jobScheduleDescription != nil {
		jobComponentConfig = &jobScheduleDescription.RadixJobComponentConfig
	}

	job, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap, err := model.buildJobSpec(jobName, rd, jobComponent, payloadSecret, model.common.Kube, jobComponentConfig)
	if err != nil {
		return nil, err
	}
	namespace := model.common.Env.RadixDeploymentNamespace
	createdJobEnvVarsConfigMap, createdJobEnvVarsMetadataConfigMap, err := model.createEnvVarsConfigMaps(namespace, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap)
	if err != nil {
		return nil, err
	}
	createdJob, err := model.common.KubeClient.BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	err = model.common.UpdateOwnerReferenceOfSecret(createdJob, payloadSecret)
	if err != nil {
		return nil, err
	}
	err = model.common.UpdateOwnerReferenceOfConfigMaps(createdJob, createdJobEnvVarsConfigMap,
		createdJobEnvVarsMetadataConfigMap)
	if err != nil {
		return nil, err
	}
	return createdJob, nil
}

func (model *jobModel) createEnvVarsConfigMaps(namespace string, jobEnvVarsConfigMap *corev1.ConfigMap, jobEnvVarsMetadataConfigMap *corev1.ConfigMap) (*corev1.ConfigMap, *corev1.ConfigMap, error) {
	createdJobEnvVarsConfigMap, err := model.common.Kube.CreateConfigMap(namespace, jobEnvVarsConfigMap)
	if err != nil {
		return nil, nil, err
	}
	createdJobEnvVarsMetadataConfigMap, err := model.common.Kube.CreateConfigMap(namespace, jobEnvVarsMetadataConfigMap)
	if err != nil {
		return nil, nil, err
	}
	return createdJobEnvVarsConfigMap, createdJobEnvVarsMetadataConfigMap, nil
}

func (model *jobModel) deleteJob(job *batchv1.Job) error {
	fg := metav1.DeletePropagationBackground
	return model.common.KubeClient.BatchV1().Jobs(job.Namespace).Delete(context.TODO(), job.Name, metav1.DeleteOptions{PropagationPolicy: &fg})
}

func (model *jobModel) getJobByName(jobName string) (*batchv1.Job, error) {
	allJobs, err := model.getAllJobs()
	if err != nil {
		return nil, err
	}

	allJobs = allJobs.Where(func(j *batchv1.Job) bool { return j.Name == jobName })

	if len(allJobs) == 1 {
		return allJobs[0], nil
	}

	return nil, jobErrors.NewNotFound("job", jobName)
}

func (model *jobModel) getAllJobs() (models.JobList, error) {
	kubeJobs, err := model.common.KubeClient.
		BatchV1().
		Jobs(model.common.Env.RadixDeploymentNamespace).
		List(
			context.TODO(),
			metav1.ListOptions{
				LabelSelector: getLabelSelectorForJobComponent(model.common.Env.RadixComponentName),
			},
		)

	if err != nil {
		return nil, err
	}

	return slice.PointersOf(kubeJobs.Items).([]*batchv1.Job), nil
}

//getJobPods jobName is optional, when empty - returns all job-pods for the namespace
func (model *jobModel) getJobPods(jobName string) ([]corev1.Pod, error) {
	listOptions := metav1.ListOptions{}
	if jobName != "" {
		listOptions.LabelSelector = getLabelSelectorForJobPods(jobName)
	}
	podList, err := model.common.KubeClient.
		CoreV1().
		Pods(model.common.Env.RadixDeploymentNamespace).
		List(
			context.TODO(),
			listOptions,
		)

	if err != nil {
		return nil, err
	}

	return podList.Items, nil
}

func (model *jobModel) buildJobSpec(jobName string, rd *radixv1.RadixDeployment, radixJobComponent *radixv1.RadixDeployJobComponent, payloadSecret *corev1.Secret, kubeutil *kube.Kube, jobComponentConfig *models.RadixJobComponentConfig) (*batchv1.Job, *corev1.ConfigMap, *corev1.ConfigMap, error) {
	podSecurityContext := model.common.SecurityContextBuilder.BuildPodSecurityContext(radixJobComponent)
	volumes, err := model.getVolumes(rd.ObjectMeta.Namespace, rd.Spec.Environment, radixJobComponent, rd.Name, payloadSecret)
	if err != nil {
		return nil, nil, nil, err
	}
	containers, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap, err := getContainersWithEnvVarsConfigMaps(kubeutil, rd, jobName, radixJobComponent, payloadSecret, jobComponentConfig, model.common.SecurityContextBuilder)
	if err != nil {
		return nil, nil, nil, err
	}

	node := &radixJobComponent.Node
	if jobComponentConfig != nil && jobComponentConfig.Node != nil {
		node = jobComponentConfig.Node
	}
	affinity := operatorUtils.GetPodSpecAffinity(node)
	tolerations := operatorUtils.GetPodSpecTolerations(node)

	var timeLimitSeconds *int64
	if jobComponentConfig != nil && jobComponentConfig.TimeLimitSeconds != nil {
		timeLimitSeconds = jobComponentConfig.TimeLimitSeconds
	} else {
		timeLimitSeconds = radixJobComponent.GetTimeLimitSeconds()
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				kube.RadixAppLabel:       rd.Spec.AppName,
				kube.RadixComponentLabel: radixJobComponent.Name,
				kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: numbers.Int32Ptr(0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						kube.RadixAppLabel:     rd.Spec.AppName,
						kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule,
					},
					Namespace: rd.ObjectMeta.Namespace,
				},
				Spec: corev1.PodSpec{
					Containers:            containers,
					Volumes:               volumes,
					SecurityContext:       podSecurityContext,
					RestartPolicy:         corev1.RestartPolicyNever,
					ImagePullSecrets:      rd.Spec.ImagePullSecrets,
					Affinity:              affinity,
					Tolerations:           tolerations,
					ActiveDeadlineSeconds: timeLimitSeconds,
				},
			},
		},
	}, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap, nil
}

func getContainersWithEnvVarsConfigMaps(kubeUtils *kube.Kube, rd *radixv1.RadixDeployment, jobName string, radixJobComponent *radixv1.RadixDeployJobComponent, payloadSecret *corev1.Secret, jobComponentConfig *models.RadixJobComponentConfig, securityContextBuilder deployment.SecurityContextBuilder) ([]corev1.Container, *corev1.ConfigMap, *corev1.ConfigMap, error) {
	environmentVariables, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap, err := buildEnvironmentVariablesWithEnvVarsConfigMaps(kubeUtils, rd, jobName, radixJobComponent)
	if err != nil {
		return nil, nil, nil, err
	}
	ports := getContainerPorts(radixJobComponent)
	containerSecurityContext := securityContextBuilder.BuildContainerSecurityContext(radixJobComponent)
	volumeMounts, err := getVolumeMounts(radixJobComponent, rd.GetName(), payloadSecret)
	if err != nil {
		return nil, nil, nil, err
	}
	resources := getResourceRequirements(radixJobComponent, jobComponentConfig)

	container := corev1.Container{
		Name:            radixJobComponent.Name,
		Image:           radixJobComponent.Image,
		ImagePullPolicy: corev1.PullAlways,
		Env:             environmentVariables,
		Ports:           ports,
		VolumeMounts:    volumeMounts,
		SecurityContext: containerSecurityContext,
		Resources:       resources,
	}

	return []corev1.Container{container}, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap, nil
}

func buildEnvironmentVariablesWithEnvVarsConfigMaps(kubeUtils *kube.Kube, rd *radixv1.RadixDeployment, jobName string, radixJobComponent *radixv1.RadixDeployJobComponent) ([]corev1.EnvVar, *corev1.ConfigMap, *corev1.ConfigMap, error) {
	envVarsConfigMap, _, envVarsMetadataMap, err := kubeUtils.GetEnvVarsConfigMapAndMetadataMap(rd.GetNamespace(), radixJobComponent.GetName()) //env-vars metadata for jobComponent to use it for job's env-vars metadata
	if err != nil {
		return nil, nil, nil, err
	}
	if envVarsMetadataMap == nil {
		envVarsMetadataMap = map[string]kube.EnvVarMetadata{}
	}
	jobEnvVarsConfigMap := kube.BuildRadixConfigEnvVarsConfigMap(rd.GetName(), jobName) //build env-vars config-name with name 'env-vars-JOB_NAME'
	jobEnvVarsConfigMap.Data = envVarsConfigMap.Data
	jobEnvVarsMetadataConfigMap := kube.BuildRadixConfigEnvVarsMetadataConfigMap(rd.GetName(), jobName) //build env-vars metadata config-name with name and 'env-vars-metadata-JOB_NAME'

	environmentVariables, err := deployment.GetEnvironmentVariables(kubeUtils, rd.Spec.AppName, rd, radixJobComponent)
	if err != nil {
		return nil, nil, nil, err
	}
	environmentVariables = append(environmentVariables, corev1.EnvVar{Name: radixJobNameEnvironmentVariable, ValueFrom: &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.labels['job-name']"},
	}})

	err = kube.SetEnvVarsMetadataMapToConfigMap(jobEnvVarsMetadataConfigMap, envVarsMetadataMap) //use env-vars metadata config-map, individual for each job
	if err != nil {
		return nil, nil, nil, err
	}

	return environmentVariables, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap, nil
}

func getVolumeMounts(radixJobComponent *radixv1.RadixDeployJobComponent, radixDeploymentName string, payloadSecret *corev1.Secret) ([]corev1.VolumeMount, error) {
	volumeMounts, err := deployment.GetRadixDeployComponentVolumeMounts(radixJobComponent, radixDeploymentName)
	if err != nil {
		return nil, err
	}
	if payloadSecret != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      schedulerDefaults.JobPayloadPropertyName,
			ReadOnly:  true,
			MountPath: radixJobComponent.Payload.Path,
		})
	}

	return volumeMounts, nil
}

func (model *jobModel) getVolumes(namespace, environment string, radixJobComponent *radixv1.RadixDeployJobComponent, radixDeploymentName string, payloadSecret *corev1.Secret) ([]corev1.Volume, error) {
	volumes, err := deployment.GetVolumes(model.common.KubeClient, model.common.Kube, namespace, environment, radixJobComponent, radixDeploymentName)
	if err != nil {
		return nil, err
	}

	if payloadSecret != nil {
		volumes = append(volumes, *getPayloadVolume(payloadSecret.Name))
	}

	return volumes, nil
}

func getResourceRequirements(radixJobComponent *radixv1.RadixDeployJobComponent, jobComponentConfig *models.RadixJobComponentConfig) corev1.ResourceRequirements {
	if jobComponentConfig != nil && jobComponentConfig.Resources != nil {
		return operatorUtils.BuildResourceRequirement(jobComponentConfig.Resources)
	} else {
		return operatorUtils.GetResourceRequirements(radixJobComponent)
	}
}

func getPayloadVolume(secretName string) *corev1.Volume {
	volume := &corev1.Volume{
		Name: schedulerDefaults.JobPayloadPropertyName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}
	return volume
}

func getContainerPorts(radixJobComponent *radixv1.RadixDeployJobComponent) []corev1.ContainerPort {
	var ports []corev1.ContainerPort
	for _, v := range radixJobComponent.Ports {
		containerPort := corev1.ContainerPort{
			Name:          v.Name,
			ContainerPort: v.Port,
		}
		ports = append(ports, containerPort)
	}
	return ports
}

func getLabelSelectorForJobComponent(componentName string) string {
	return labels.SelectorFromSet(map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
	}).String()
}

func getLabelSelectorForJobPods(jobName string) string {
	return labels.SelectorFromSet(map[string]string{
		schedulerDefaults.K8sJobNameLabel: jobName,
	}).String()
}
