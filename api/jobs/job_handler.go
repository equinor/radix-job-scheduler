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
	"github.com/equinor/radix-operator/pkg/apis/securitycontext"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	radixJobNameEnvironmentVariable = "RADIX_JOB_NAME"
)

type jobHandler struct {
	common *api.Handler
}

type JobHandler interface {
	//GetJobs Get status of all jobs
	GetJobs() ([]models.JobStatus, error)
	//GetJob Get status of a job
	GetJob(name string) (*models.JobStatus, error)
	//CreateJob Create a job with parameters. `batchName` is optional.
	CreateJob(jobScheduleDescription *models.JobScheduleDescription, batchName string) (*models.JobStatus, error)
	//MaintainHistoryLimit Delete outdated jobs
	MaintainHistoryLimit() error
	//DeleteJob Delete a job
	DeleteJob(jobName string) error
}

// New Constructor for job handler
func New(env *models.Env, kube *kube.Kube) JobHandler {
	return &jobHandler{
		common: &api.Handler{
			Kube:        kube,
			KubeClient:  kube.KubeClient(),
			RadixClient: kube.RadixClient(),
			Env:         env,
		},
	}
}

// GetJobs Get status of all jobs
func (handler *jobHandler) GetJobs() ([]models.JobStatus, error) {
	log.Debugf("Get Jobs for namespace: %s", handler.common.Env.RadixDeploymentNamespace)

	kubeJobs, err := handler.getAllJobs()
	if err != nil {
		return nil, err
	}

	pods, err := handler.getJobPods("")
	if err != nil {
		return nil, err
	}
	podsMap := api.GetPodsToJobNameMap(pods)
	jobs := make([]models.JobStatus, len(kubeJobs))
	for idx, k8sJob := range kubeJobs {
		jobs[idx] = *GetJobStatusFromJob(handler.common.KubeClient, k8sJob, podsMap[k8sJob.Name])
	}

	log.Debugf("Found %v jobs for namespace %s", len(jobs), handler.common.Env.RadixDeploymentNamespace)
	return jobs, nil
}

// GetJob Get status of a job
func (handler *jobHandler) GetJob(jobName string) (*models.JobStatus, error) {
	log.Debugf("get jobs for namespace: %s", handler.common.Env.RadixDeploymentNamespace)
	job, err := handler.getJobByName(jobName)
	if err != nil {
		return nil, err
	}
	log.Debugf("found Job %s for namespace: %s", jobName, handler.common.Env.RadixDeploymentNamespace)
	pods, err := handler.getJobPods(job.Name)
	if err != nil {
		return nil, err
	}
	jobStatus := GetJobStatusFromJob(handler.common.KubeClient, job, pods)
	return jobStatus, nil
}

// CreateJob Create a job with parameters
func (handler *jobHandler) CreateJob(jobScheduleDescription *models.JobScheduleDescription,
	batchName string) (*models.JobStatus, error) {
	log.Debugf("create job for namespace: %s", handler.common.Env.RadixDeploymentNamespace)

	radixDeployment, err := handler.common.RadixClient.RadixV1().RadixDeployments(handler.common.Env.RadixDeploymentNamespace).Get(context.Background(), handler.common.Env.RadixDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, jobErrors.NewNotFound("radix deployment", handler.common.Env.RadixDeploymentName)
	}

	jobComponent := radixDeployment.GetJobComponentByName(handler.common.Env.RadixComponentName)
	if jobComponent == nil {
		return nil, jobErrors.NewNotFound("job component", handler.common.Env.RadixComponentName)
	}

	jobName := generateJobName(jobComponent)

	job, err := handler.createJob(jobName, jobComponent, radixDeployment, jobScheduleDescription, batchName)
	if err != nil {
		return nil, jobErrors.NewFromError(err)
	}

	log.Debug(fmt.Sprintf("created job %s for component %s, environment %s, in namespace: %s", job.Name, handler.common.Env.RadixComponentName, radixDeployment.Spec.Environment, handler.common.Env.RadixDeploymentNamespace))
	return GetJobStatusFromJob(handler.common.KubeClient, job, nil), nil
}

// DeleteJob Delete a job
func (handler *jobHandler) DeleteJob(jobName string) error {
	log.Debugf("delete job %s for namespace: %s", jobName, handler.common.Env.RadixDeploymentNamespace)
	return handler.garbageCollectJob(jobName)
}

// MaintainHistoryLimit Delete outdated jobs
func (handler *jobHandler) MaintainHistoryLimit() error {
	jobList, err := handler.getAllJobs()
	if err != nil {
		return err
	}

	log.Debug("maintain history limit for succeeded jobs")
	succeededJobs := jobList.Where(func(j *batchv1.Job) bool {
		_, batchNameLabelExists := j.ObjectMeta.Labels[kube.RadixBatchNameLabel]
		return j.Status.Succeeded > 0 && !batchNameLabelExists
	})
	if err = handler.maintainHistoryLimitForJobs(succeededJobs, handler.common.Env.RadixJobSchedulersPerEnvironmentHistoryLimit); err != nil {
		return err
	}

	log.Debug("maintain history limit for failed jobs")
	failedJobs := jobList.Where(func(j *batchv1.Job) bool { return j.Status.Failed > 0 })
	if err = handler.maintainHistoryLimitForJobs(failedJobs, handler.common.Env.RadixJobSchedulersPerEnvironmentHistoryLimit); err != nil {
		return err
	}

	return nil
}

func (handler *jobHandler) maintainHistoryLimitForJobs(jobs []*batchv1.Job, historyLimit int) error {
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
		if err := handler.garbageCollectJob(job.Name); err != nil {
			return err
		}
	}
	return nil
}

func (handler *jobHandler) garbageCollectJob(jobName string) (err error) {
	job, err := handler.getJobByName(jobName)
	if err != nil {
		return
	}

	secrets, err := handler.common.GetSecretsForJob(jobName)
	if err != nil {
		return
	}

	for _, secret := range secrets.Items {
		if err = handler.common.DeleteSecret(&secret); err != nil {
			return
		}
	}

	services, err := handler.common.GetServiceForJob(jobName)
	if err != nil {
		return
	}

	for _, service := range services.Items {
		if err = handler.common.DeleteService(&service); err != nil {
			return
		}
	}

	err = handler.deleteJob(job)
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

func (handler *jobHandler) createJob(jobName string, jobComponent *radixv1.RadixDeployJobComponent, rd *radixv1.RadixDeployment, jobScheduleDescription *models.JobScheduleDescription, batchName string) (*batchv1.Job, error) {
	appName := rd.Spec.AppName
	namespace := handler.common.Env.RadixDeploymentNamespace
	var jobComponentConfig *models.RadixJobComponentConfig
	if jobScheduleDescription != nil {
		jobComponentConfig = &jobScheduleDescription.RadixJobComponentConfig
	}

	payloadSecret, err := handler.common.CreatePayloadSecret(appName, jobName, jobComponent, jobScheduleDescription)
	if err != nil {
		return nil, jobErrors.NewFromError(err)
	}
	job, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap, err := handler.buildJobSpec(jobName, rd, jobComponent,
		payloadSecret, jobComponentConfig, batchName, jobScheduleDescription)
	if err != nil {
		return nil, err
	}

	createdJobEnvVarsConfigMap, createdJobEnvVarsMetadataConfigMap, err := handler.createEnvVarsConfigMaps(namespace, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap)
	if err != nil {
		return nil, err
	}

	service, err := handler.common.CreateService(appName, jobName, jobComponent)
	if err != nil {
		return nil, jobErrors.NewFromError(err)
	}
	if len(batchName) > 0 {
		batch, err := handler.common.GetBatch(batchName)
		if err == nil {
			job.OwnerReferences = []metav1.OwnerReference{api.GetJobOwnerReference(batch)}
		} else if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("the batch %s does not exist", batchName)
		}
	}
	createdJob, err := handler.common.KubeClient.BatchV1().Jobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	if err != nil {
		_ = handler.common.Kube.DeleteSecret(payloadSecret.GetNamespace(), payloadSecret.GetName())
		_ = handler.common.DeleteConfigMap(createdJobEnvVarsConfigMap)
		_ = handler.common.DeleteConfigMap(createdJobEnvVarsMetadataConfigMap)
		_ = handler.common.DeleteService(service)
		return nil, err
	}
	jobOwnerReference := api.GetJobOwnerReference(createdJob)
	err = handler.common.UpdateOwnerReferenceOfSecret(namespace, jobOwnerReference, payloadSecret)
	if err != nil {
		return nil, err
	}
	err = handler.common.UpdateOwnerReferenceOfConfigMaps(namespace, jobOwnerReference, createdJobEnvVarsConfigMap,
		createdJobEnvVarsMetadataConfigMap)
	if err != nil {
		return nil, err
	}
	err = handler.common.UpdateOwnerReferenceOfService(namespace, jobOwnerReference, service)
	if err != nil {
		return nil, err
	}

	return createdJob, nil
}

func (handler *jobHandler) createEnvVarsConfigMaps(namespace string, jobEnvVarsConfigMap *corev1.ConfigMap, jobEnvVarsMetadataConfigMap *corev1.ConfigMap) (*corev1.ConfigMap, *corev1.ConfigMap, error) {
	createdJobEnvVarsConfigMap, err := handler.common.Kube.CreateConfigMap(namespace, jobEnvVarsConfigMap)
	if err != nil {
		return nil, nil, err
	}
	createdJobEnvVarsMetadataConfigMap, err := handler.common.Kube.CreateConfigMap(namespace, jobEnvVarsMetadataConfigMap)
	if err != nil {
		return nil, nil, err
	}
	return createdJobEnvVarsConfigMap, createdJobEnvVarsMetadataConfigMap, nil
}

func (handler *jobHandler) deleteJob(job *batchv1.Job) error {
	fg := metav1.DeletePropagationBackground
	return handler.common.KubeClient.BatchV1().Jobs(job.Namespace).Delete(context.Background(), job.Name, metav1.DeleteOptions{PropagationPolicy: &fg})
}

func (handler *jobHandler) getJobByName(jobName string) (*batchv1.Job, error) {
	allJobs, err := handler.getAllJobs()
	if err != nil {
		return nil, err
	}

	allJobs = allJobs.Where(func(j *batchv1.Job) bool { return j.Name == jobName })

	if len(allJobs) == 1 {
		return allJobs[0], nil
	}

	return nil, jobErrors.NewNotFound("job", jobName)
}

func (handler *jobHandler) getAllJobs() (models.JobList, error) {
	kubeJobs, err := handler.common.KubeClient.
		BatchV1().
		Jobs(handler.common.Env.RadixDeploymentNamespace).
		List(
			context.Background(),
			metav1.ListOptions{
				LabelSelector: getLabelSelectorForJobComponent(handler.common.Env.RadixComponentName),
			},
		)

	if err != nil {
		return nil, err
	}

	return slice.PointersOf(kubeJobs.Items).([]*batchv1.Job), nil
}

// getJobPods jobName is optional, when empty - returns all job-pods for the namespace
func (handler *jobHandler) getJobPods(jobName string) ([]corev1.Pod, error) {
	listOptions := metav1.ListOptions{}
	if jobName != "" {
		listOptions.LabelSelector = getLabelSelectorForJobPods(jobName)
	}
	podList, err := handler.common.KubeClient.
		CoreV1().
		Pods(handler.common.Env.RadixDeploymentNamespace).
		List(
			context.Background(),
			listOptions,
		)

	if err != nil {
		return nil, err
	}

	return podList.Items, nil
}

func (handler *jobHandler) buildJobSpec(jobName string, rd *radixv1.RadixDeployment, radixJobComponent *radixv1.RadixDeployJobComponent, payloadSecret *corev1.Secret, jobComponentConfig *models.RadixJobComponentConfig, batchName string, jobScheduleDescription *models.JobScheduleDescription) (*batchv1.Job, *corev1.ConfigMap, *corev1.ConfigMap, error) {
	volumes, err := handler.getVolumes(rd.ObjectMeta.Namespace, rd.Spec.Environment, radixJobComponent, rd.Name, payloadSecret)
	if err != nil {
		return nil, nil, nil, err
	}
	containers, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap, err := handler.getContainersWithEnvVarsConfigMaps(rd, jobName, radixJobComponent, payloadSecret, jobComponentConfig)
	if err != nil {
		return nil, nil, nil, err
	}

	node := &radixJobComponent.Node
	if jobComponentConfig != nil && jobComponentConfig.Node != nil {
		node = jobComponentConfig.Node
	}
	affinity := operatorUtils.GetPodSpecAffinity(node, rd.Spec.AppName, radixJobComponent.Name)
	tolerations := operatorUtils.GetPodSpecTolerations(node)

	var timeLimitSeconds *int64
	if jobComponentConfig != nil && jobComponentConfig.TimeLimitSeconds != nil {
		timeLimitSeconds = jobComponentConfig.TimeLimitSeconds
	} else {
		timeLimitSeconds = radixJobComponent.GetTimeLimitSeconds()
	}

	if *timeLimitSeconds < *numbers.Int64Ptr(1) {
		return nil, nil, nil, fmt.Errorf("timeLimitSeconds must be greater than 0")
	}

	jobLabels := radixlabels.Merge(
		radixlabels.ForApplicationName(rd.Spec.AppName),
		radixlabels.ForComponentName(radixJobComponent.Name),
		map[string]string{kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule},
	)

	podLabels := radixlabels.Merge(
		radixlabels.ForApplicationName(rd.Spec.AppName),
		radixlabels.ForComponentName(radixJobComponent.Name),
		radixlabels.ForPodWithRadixIdentity(radixJobComponent.Identity),
		map[string]string{kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule},
	)

	serviceAccountSpec := deployment.NewServiceAccountSpec(rd, radixJobComponent)

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: jobLabels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: numbers.Int32Ptr(0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:    podLabels,
					Namespace: rd.ObjectMeta.Namespace,
				},
				Spec: corev1.PodSpec{
					Containers:                   containers,
					Volumes:                      volumes,
					SecurityContext:              securitycontext.Pod(),
					RestartPolicy:                corev1.RestartPolicyNever,
					ImagePullSecrets:             rd.Spec.ImagePullSecrets,
					Affinity:                     affinity,
					Tolerations:                  tolerations,
					ActiveDeadlineSeconds:        timeLimitSeconds,
					ServiceAccountName:           serviceAccountSpec.ServiceAccountName(),
					AutomountServiceAccountToken: serviceAccountSpec.AutomountServiceAccountToken(),
				},
			},
		},
	}
	if len(jobScheduleDescription.JobId) > 0 {
		job.ObjectMeta.Labels[kube.RadixJobIdLabel] = jobScheduleDescription.JobId
		job.Spec.Template.ObjectMeta.Labels[kube.RadixJobIdLabel] = jobScheduleDescription.JobId
	}
	if len(batchName) > 0 {
		job.ObjectMeta.Labels[kube.RadixBatchNameLabel] = batchName
		job.Spec.Template.ObjectMeta.Labels[kube.RadixBatchNameLabel] = batchName
	}
	return &job, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap, nil
}

func (handler *jobHandler) getContainersWithEnvVarsConfigMaps(rd *radixv1.RadixDeployment, jobName string, radixJobComponent *radixv1.RadixDeployJobComponent, payloadSecret *corev1.Secret, jobComponentConfig *models.RadixJobComponentConfig) ([]corev1.Container, *corev1.ConfigMap, *corev1.ConfigMap, error) {
	environmentVariables, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap,
		err := handler.buildEnvironmentVariablesWithEnvVarsConfigMaps(rd, jobName, radixJobComponent)
	if err != nil {
		return nil, nil, nil, err
	}
	ports := getContainerPorts(radixJobComponent)
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
		SecurityContext: securitycontext.Container(),
		Resources:       resources,
	}

	return []corev1.Container{container}, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap, nil
}

func (handler *jobHandler) buildEnvironmentVariablesWithEnvVarsConfigMaps(rd *radixv1.RadixDeployment, jobName string, radixJobComponent *radixv1.RadixDeployJobComponent) ([]corev1.EnvVar, *corev1.ConfigMap, *corev1.ConfigMap, error) {
	envVarsConfigMap, _, envVarsMetadataMap, err := handler.common.Kube.GetEnvVarsConfigMapAndMetadataMap(rd.
		GetNamespace(), radixJobComponent.GetName()) //env-vars metadata for jobComponent to use it for job's env-vars metadata
	if err != nil {
		return nil, nil, nil, err
	}
	if envVarsMetadataMap == nil {
		envVarsMetadataMap = map[string]kube.EnvVarMetadata{}
	}
	jobEnvVarsConfigMap := kube.BuildRadixConfigEnvVarsConfigMap(rd.GetName(), jobName) //build env-vars config-name with name 'env-vars-JOB_NAME'
	jobEnvVarsConfigMap.Data = envVarsConfigMap.Data
	jobEnvVarsMetadataConfigMap := kube.BuildRadixConfigEnvVarsMetadataConfigMap(rd.GetName(), jobName) //build env-vars metadata config-name with name and 'env-vars-metadata-JOB_NAME'

	environmentVariables, err := deployment.GetEnvironmentVariables(handler.common.Kube, rd.Spec.AppName, rd, radixJobComponent)
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

func (handler *jobHandler) getVolumes(namespace, environment string, radixJobComponent *radixv1.RadixDeployJobComponent, radixDeploymentName string, payloadSecret *corev1.Secret) ([]corev1.Volume, error) {
	volumes, err := deployment.GetVolumes(handler.common.KubeClient, handler.common.Kube, namespace, environment, radixJobComponent, radixDeploymentName)
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
