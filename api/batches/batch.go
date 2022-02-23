package batches

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-job-scheduler/api"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
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
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

const (
	radixJobNameEnvironmentVariable = "RADIX_JOB_NAME"
)

type batchModel struct {
	common *api.Model
}

type Batch interface {
	//GetBatches Get status of all batches
	GetBatches() ([]models.BatchStatus, error)
	//GetBatch Get status of a batch
	GetBatch(batchName string) (*models.BatchStatus, error)
	//CreateBatch Create a batch with parameters
	CreateBatch(batchScheduleDescription *models.BatchScheduleDescription) (*models.BatchStatus, error)
	//MaintainHistoryLimit Delete outdated batches
	MaintainHistoryLimit() error
	//DeleteBatch Delete a batch
	DeleteBatch(batchName string) error
}

//New Constructor of the batch model
func New(env *models.Env, kube *kube.Kube, kubeClient kubernetes.Interface, radixClient radixclient.Interface) Batch {
	return &batchModel{
		common: &api.Model{
			Kube:                   kube,
			KubeClient:             kubeClient,
			RadixClient:            radixClient,
			Env:                    env,
			SecurityContextBuilder: deployment.NewSecurityContextBuilder(true),
		},
	}
}

//GetBatches Get status of all batches
func (model *batchModel) GetBatches() ([]models.BatchStatus, error) {
	log.Debugf("Get Batches for namespace: %s", model.common.Env.RadixDeploymentNamespace)

	//TODO
	//kubeBatches, err := model.getAllBatches()
	//if err != nil {
	//    return nil, err
	//}

	//pods, err := model.GetBatchPods("")
	//if err != nil {
	//    return nil, err
	//}
	//podsMap := getBatchPodsMap(pods)
	//batches := make([]models.BatchStatus, len(kubeBatches))
	//for idx, k8sBatch := range kubeBatches {
	//    batches[idx] = *models.GetBatchStatusFromJob(model.common.KubeClient, k8sBatch, podsMap[k8sBatch.Name])
	//}

	//log.Debugf("Found %v batches for namespace %s", len(batches), model.common.Env.RadixDeploymentNamespace)
	//return batches, nil
	return nil, nil
}

func getBatchPodsMap(pods []corev1.Pod) map[string][]corev1.Pod {
	podsMap := make(map[string][]corev1.Pod)
	for _, pod := range pods {
		batchName := pod.Labels[schedulerDefaults.K8sJobNameLabel]
		if len(batchName) > 0 {
			podsMap[batchName] = append(podsMap[batchName], pod)
		}
	}
	return podsMap
}

//GetBatch Get status of a batch
func (model *batchModel) GetBatch(batchName string) (*models.BatchStatus, error) {
	log.Debugf("get batches for namespace: %s", model.common.Env.RadixDeploymentNamespace)
	batch, err := model.getBatchByName(batchName)
	if err != nil {
		return nil, err
	}
	log.Debugf("found Batch %s for namespace: %s", batchName, model.common.Env.RadixDeploymentNamespace)
	pods, err := model.getJobPods(batch.Name)
	if err != nil {
		return nil, err
	}
	return GetBatchStatusFromJob(model.common.KubeClient, batch, pods)
}

//CreateBatch Create a batch with parameters
func (model *batchModel) CreateBatch(batchScheduleDescription *models.BatchScheduleDescription) (*models.BatchStatus, error) {
	log.Debugf("create batch for namespace: %s", model.common.Env.RadixDeploymentNamespace)

	radixDeployment, err := model.common.RadixClient.RadixV1().RadixDeployments(model.common.Env.RadixDeploymentNamespace).Get(context.TODO(), model.common.Env.RadixDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, apiErrors.NewNotFound("radix deployment", model.common.Env.RadixDeploymentName)
	}

	jobComponent := radixDeployment.GetJobComponentByName(model.common.Env.RadixComponentName)
	if jobComponent == nil {
		return nil, apiErrors.NewNotFound("job component", model.common.Env.RadixComponentName)
	}

	batchName := generateBatchName(jobComponent)

	descriptionSecret, err := model.common.CreateBatchScheduleDescriptionSecret(batchName, jobComponent, radixDeployment, batchScheduleDescription)
	if err != nil {
		return nil, apiErrors.NewFromError(err)
	}

	if err = model.common.CreateService(batchName, jobComponent, radixDeployment); err != nil {
		return nil, apiErrors.NewFromError(err)
	}

	batch, err := model.createBatch(batchName, jobComponent, radixDeployment, descriptionSecret, batchScheduleDescription)
	if err != nil {
		return nil, apiErrors.NewFromError(err)
	}

	log.Debug(fmt.Sprintf("created batch %s for component %s, environment %s, in namespace: %s", batch.Name, model.common.Env.RadixComponentName, radixDeployment.Spec.Environment, model.common.Env.RadixDeploymentNamespace))
	return GetBatchStatusFromJob(model.common.KubeClient, batch, nil)
}

//DeleteBatch Delete a batch
func (model *batchModel) DeleteBatch(batchName string) error {
	log.Debugf("delete batch %s for namespace: %s", batchName, model.common.Env.RadixDeploymentNamespace)
	return model.garbageCollectBatch(batchName)
}

//MaintainHistoryLimit Delete outdated batches
func (model *batchModel) MaintainHistoryLimit() error {
	batchList, err := model.getAllBatches()
	if err != nil {
		return err
	}

	log.Debug("maintain history limit for succeeded batches")
	succeededBatches := batchList.Where(func(j *batchv1.Job) bool { return j.Status.Succeeded > 0 })
	if err = model.maintainHistoryLimitForBatches(succeededBatches,
		model.common.Env.RadixJobSchedulersPerEnvironmentHistoryLimit); err != nil {
		return err
	}

	log.Debug("maintain history limit for failed batches")
	failedBatches := batchList.Where(func(j *batchv1.Job) bool { return j.Status.Failed > 0 })
	if err = model.maintainHistoryLimitForBatches(failedBatches,
		model.common.Env.RadixJobSchedulersPerEnvironmentHistoryLimit); err != nil {
		return err
	}

	return nil
}

func (model *batchModel) maintainHistoryLimitForBatches(batches []*batchv1.Job, historyLimit int) error {
	numToDelete := len(batches) - historyLimit
	if numToDelete <= 0 {
		log.Debug("no history batches to delete")
		return nil
	}
	log.Debugf("history batches to delete: %v", numToDelete)

	sortedBatches := sortRJSchByCompletionTimeAsc(batches)
	for i := 0; i < numToDelete; i++ {
		batch := sortedBatches[i]
		log.Debugf("deleting batch %s", batch.Name)
		if err := model.garbageCollectBatch(batch.Name); err != nil {
			return err
		}
	}
	return nil
}

func (model *batchModel) garbageCollectBatch(batchName string) (err error) {
	//TODO
	//batch, err := model.getBatchByName(batchName)
	//if err != nil {
	//    return
	//}
	//    secrets, err := model.getSecretsForBatch(batchName)
	//    if err != nil {
	//        return
	//    }
	//
	//    for _, secret := range secrets.Items {
	//        if err = model.deleteSecret(&secret); err != nil {
	//            return
	//        }
	//    }
	//
	//    services, err := model.getServiceForBatch(batchName)
	//    if err != nil {
	//        return
	//    }
	//
	//    for _, service := range services.Items {
	//        if err = model.deleteService(&service); err != nil {
	//            return
	//        }
	//    }
	//
	//    err = model.deleteBatch(batch)
	//    if err != nil {
	//        return err
	//    }
	//
	return nil
}

func sortRJSchByCompletionTimeAsc(batches []*batchv1.Job) []*batchv1.Job {
	sort.Slice(batches, func(i, j int) bool {
		batch1 := (batches)[i]
		batch2 := (batches)[j]
		return isRJS1CompletedBeforeRJS2(batch1, batch2)
	})
	return batches
}

func isRJS1CompletedBeforeRJS2(batch1 *batchv1.Job, batch2 *batchv1.Job) bool {
	rd1ActiveFrom := getCompletionTimeFrom(batch1)
	rd2ActiveFrom := getCompletionTimeFrom(batch2)

	return rd1ActiveFrom.Before(rd2ActiveFrom)
}

func getCompletionTimeFrom(batch *batchv1.Job) *metav1.Time {
	if batch.Status.CompletionTime.IsZero() {
		return &batch.CreationTimestamp
	}
	return batch.Status.CompletionTime
}

func generateBatchName(jobComponent *radixv1.RadixDeployJobComponent) string {
	timestamp := time.Now().Format("20060102150405")
	jobTag := strings.ToLower(commonUtils.RandString(8))
	return fmt.Sprintf("batch-%s-%s-%s", jobComponent.Name, timestamp, jobTag)
}

func (model *batchModel) createBatch(jobName string, jobComponent *radixv1.RadixDeployJobComponent, rd *radixv1.RadixDeployment, batchScheduleDescriptionSecret *corev1.Secret, batchScheduleDescription *models.BatchScheduleDescription) (*batchv1.Job, error) {
	var defaultJobComponentConfig *models.RadixJobComponentConfig
	if batchScheduleDescription != nil {
		defaultJobComponentConfig = &batchScheduleDescription.DefaultRadixJobComponentConfig
	}

	job, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap, err := model.buildBatchJobSpec(jobName, rd, jobComponent, batchScheduleDescriptionSecret, model.common.Kube, defaultJobComponentConfig)
	if err != nil {
		return nil, err
	}
	namespace := model.common.Env.RadixDeploymentNamespace
	//TODO - revise
	createdJobEnvVarsConfigMap, createdJobEnvVarsMetadataConfigMap, err := model.createEnvVarsConfigMaps(namespace, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap)
	if err != nil {
		return nil, err
	}
	createdJob, err := model.common.KubeClient.BatchV1().Jobs(namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	err = model.updateOwnerReferenceOfConfigMaps(createdJob, createdJobEnvVarsConfigMap, createdJobEnvVarsMetadataConfigMap)
	if err != nil {
		return nil, err
	}
	return createdJob, nil
}

func (model *batchModel) createEnvVarsConfigMaps(namespace string, jobEnvVarsConfigMap *corev1.ConfigMap, jobEnvVarsMetadataConfigMap *corev1.ConfigMap) (*corev1.ConfigMap, *corev1.ConfigMap, error) {
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

func (model *batchModel) updateOwnerReferenceOfConfigMaps(ownerJob *batchv1.Job, configMaps ...*corev1.ConfigMap) error {
	jobOwnerReferences := getJobOwnerReferences(ownerJob)
	for _, configMap := range configMaps {
		configMap.OwnerReferences = jobOwnerReferences
	}
	return model.common.Kube.UpdateConfigMap(ownerJob.ObjectMeta.GetNamespace(), configMaps...)
}

func (model *batchModel) deleteJob(job *batchv1.Job) error {
	fg := metav1.DeletePropagationBackground
	return model.common.KubeClient.BatchV1().Jobs(job.Namespace).Delete(context.TODO(), job.Name, metav1.DeleteOptions{PropagationPolicy: &fg})
}

func (model *batchModel) getBatchByName(batchName string) (*batchv1.Job, error) {
	batches, err := model.getAllBatches()
	if err != nil {
		return nil, err
	}

	batches = batches.Where(func(j *batchv1.Job) bool { return j.Name == batchName })

	if len(batches) == 1 {
		return batches[0], nil
	}

	return nil, apiErrors.NewNotFound("batch", batchName)
}

func (model *batchModel) getAllBatches() (models.JobList, error) {
	kubeBatches, err := model.common.KubeClient.
		BatchV1().
		Jobs(model.common.Env.RadixDeploymentNamespace).
		List(
			context.TODO(),
			metav1.ListOptions{
				LabelSelector: getLabelSelectorForJobComponentBatches(model.common.Env.RadixComponentName),
			},
		)

	if err != nil {
		return nil, err
	}

	return slice.PointersOf(kubeBatches.Items).([]*batchv1.Job), nil
}

//getJobPods jobName is optional, when empty - returns all job-pods for the namespace
func (model *batchModel) getJobPods(jobName string) ([]corev1.Pod, error) {
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

func (model *batchModel) buildBatchJobSpec(batchName string, rd *radixv1.RadixDeployment, radixJobComponent *radixv1.RadixDeployJobComponent, batchScheduleDescriptionSecret *corev1.Secret, kubeutil *kube.Kube, defaultJobComponentConfig *models.RadixJobComponentConfig) (*batchv1.Job, *corev1.ConfigMap, *corev1.ConfigMap, error) {
	podSecurityContext := model.common.SecurityContextBuilder.BuildPodSecurityContext(radixJobComponent)
	volumes, err := model.getVolumes(rd.ObjectMeta.Namespace, rd.Spec.Environment, radixJobComponent, rd.Name, batchScheduleDescriptionSecret)
	if err != nil {
		return nil, nil, nil, err
	}
	containers, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap, err := getContainersWithEnvVarsConfigMaps(kubeutil, rd, batchName, radixJobComponent, batchScheduleDescriptionSecret, defaultJobComponentConfig, model.common.SecurityContextBuilder)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(containers) != 1 {
		return nil, nil, nil, fmt.Errorf("expected one container, but created %d", len(containers))
	}

	containers[0].Env = append(containers[0].Env, []corev1.EnvVar{
		{
			Name:  schedulerDefaults.BatchNameEnvVarName,
			Value: batchName,
		},
		{
			Name:  schedulerDefaults.BatchScheduleDescriptionPath,
			Value: path.Join(schedulerDefaults.BatchSecretsMountPath, batchScheduleDescriptionSecret.Name),
		},
	}...)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: batchName,
			Labels: map[string]string{
				kube.RadixAppLabel:       rd.Spec.AppName,
				kube.RadixComponentLabel: radixJobComponent.Name,
				kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
				"radix-batch-name":       batchName,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: numbers.Int32Ptr(0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						kube.RadixAppLabel:     rd.Spec.AppName,
						kube.RadixJobTypeLabel: kube.RadixJobTypeJobSchedule,
						"radix-batch-name":     batchName,
					},
					Namespace: rd.ObjectMeta.Namespace,
				},
				Spec: corev1.PodSpec{
					Containers:       containers,
					Volumes:          volumes,
					SecurityContext:  podSecurityContext,
					RestartPolicy:    corev1.RestartPolicyNever,
					ImagePullSecrets: rd.Spec.ImagePullSecrets,
				},
			},
		},
	}, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap, nil
}

func getContainersWithEnvVarsConfigMaps(kubeUtils *kube.Kube, rd *radixv1.RadixDeployment, jobName string, radixJobComponent *radixv1.RadixDeployJobComponent, batchScheduleDescriptionSecret *corev1.Secret, jobComponentConfig *models.RadixJobComponentConfig, securityContextBuilder deployment.SecurityContextBuilder) ([]corev1.Container, *corev1.ConfigMap, *corev1.ConfigMap, error) {
	environmentVariables, jobEnvVarsConfigMap, jobEnvVarsMetadataConfigMap, err := buildEnvironmentVariablesWithEnvVarsConfigMaps(kubeUtils, rd, jobName, radixJobComponent)
	if err != nil {
		return nil, nil, nil, err
	}
	ports := getContainerPorts(radixJobComponent)
	volumeMounts, err := getVolumeMounts(batchScheduleDescriptionSecret)
	if err != nil {
		return nil, nil, nil, err
	}
	resources := getResourceRequirements(radixJobComponent, jobComponentConfig)
	containerSecurityContext := securityContextBuilder.BuildContainerSecurityContext(radixJobComponent)

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

func getJobOwnerReferences(job *batchv1.Job) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: "batch/v1",
			Kind:       "Job",
			Name:       job.GetName(),
			UID:        job.UID,
			Controller: commonUtils.BoolPtr(true),
		},
	}
}

func getVolumeMounts(batchScheduleDescriptionSecret *corev1.Secret) ([]corev1.VolumeMount, error) {
	volumeMounts := make([]corev1.VolumeMount, 0)
	if batchScheduleDescriptionSecret != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      schedulerDefaults.BatchScheduleDescriptionPropertyName,
			ReadOnly:  true,
			MountPath: schedulerDefaults.BatchSecretsMountPath,
		})
	}

	return volumeMounts, nil
}

func (model *batchModel) getVolumes(namespace, environment string, radixJobComponent *radixv1.RadixDeployJobComponent, radixDeploymentName string, batchScheduleDescriptionSecret *corev1.Secret) ([]corev1.Volume, error) {
	volumes, err := deployment.GetVolumes(model.common.KubeClient, model.common.Kube, namespace, environment, radixJobComponent, radixDeploymentName)
	if err != nil {
		return nil, err
	}

	if batchScheduleDescriptionSecret != nil {
		volumes = append(volumes, *getBatchScheduleDescriptionVolume(batchScheduleDescriptionSecret.Name))
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

func getBatchScheduleDescriptionVolume(secretName string) *corev1.Volume {
	volume := &corev1.Volume{
		Name: schedulerDefaults.BatchScheduleDescriptionPropertyName,
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

func getLabelSelectorForJobComponentBatches(componentName string) string {
	componentRequirement, _ := labels.NewRequirement(kube.RadixComponentLabel, selection.Equals, []string{componentName})
	jobTypeRequirement, _ := labels.NewRequirement(kube.RadixJobTypeLabel, selection.Equals, []string{kube.RadixJobTypeJobSchedule})
	batchNameRequirement, _ := labels.NewRequirement("radix-batch-name", selection.Exists, nil) //TODO kube.Label...
	return labels.NewSelector().Add(*componentRequirement, *jobTypeRequirement, *batchNameRequirement).String()
}

func getLabelSelectorForJobPods(jobName string) string {
	return labels.SelectorFromSet(map[string]string{
		schedulerDefaults.K8sJobNameLabel: jobName,
	}).String()
}
