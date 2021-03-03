package job

import (
	"context"
	"errors"
	"fmt"
	models "github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	utils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/git"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"sort"
	"strings"
	"time"
)

type Handler interface {
	GetJobs(ctx context.Context) (*[]models.JobStatus, error)
	GetJob(ctx context.Context, name string) (*models.JobStatus, error)
	CreateJob(ctx context.Context, jobScheduleDescription *models.JobScheduleDescription) (*models.JobStatus, error)
	DeleteJob(ctx context.Context, jobName string) error
}

type jobHandler struct {
	kubeUtil models.KubeUtil
}

func New(kubeUtil models.KubeUtil) Handler {
	return &jobHandler{
		kubeUtil: kubeUtil,
	}
}

func (jh *jobHandler) GetJobs(ctx context.Context) (*[]models.JobStatus, error) {
	kubeClient := *jh.kubeUtil.KubeClient()
	namespace := jh.kubeUtil.CurrentNamespace()
	log.Debugf("Get Jobs for namespace: %s", namespace)
	kubeJobs, err := kubeClient.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
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

func (jh *jobHandler) GetJob(ctx context.Context, jobName string) (*models.JobStatus, error) {
	kubeClient := *jh.kubeUtil.KubeClient()
	namespace := jh.kubeUtil.CurrentNamespace()
	log.Debugf("get jobs for namespace: %s", namespace)
	job, err := kubeClient.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, errors.New(fmt.Sprintf("not found job %s for namespace: %s", jobName, namespace))
	}
	log.Debugf("found Job %s for namespace: %s", jobName, namespace)
	jobStatus := models.GetJobStatusFromJob(job)
	return jobStatus, nil
}

func (jh *jobHandler) CreateJob(ctx context.Context, jobScheduleDescription *models.JobScheduleDescription) (*models.JobStatus, error) {
	kubeClient := *jh.kubeUtil.KubeClient()
	radixV1 := *jh.kubeUtil.RadixV1()
	namespace := jh.kubeUtil.CurrentNamespace()
	log.Debugf("create job for namespace: %s", namespace)
	radixDeployment, err := radixV1.RadixDeployments(namespace).Get(jobScheduleDescription.RadixDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("fail to get Radix deployment %s for namespace: %s", jobScheduleDescription.RadixDeploymentName, namespace))
	}
	job := jh.kubeUtil.createJob(radixDeployment, jobScheduleDescription)
	job, err := kubeClient.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, errors.New(fmt.Sprintf("not created job for component %s, environment %s, in namespace: %s", jobScheduleDescription.ComponentName, jobScheduleDescription.Environment, namespace))
	}
	log.Debugf("found Job %s for namespace: %s", jobName, namespace)
	return nil, nil
}

func (jh *jobHandler) DeleteJob(ctx context.Context, jobName string) error {
	kubeClient := *jh.kubeUtil.KubeClient()
	namespace := jh.kubeUtil.CurrentNamespace()
	log.Debugf("delete job %s for namespace: %s", jobName, namespace)
	err := kubeClient.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	log.Debugf("deleted job %s for namespace: %s", jobName, namespace)
	return nil
}

func (jh *jobHandler) createJob(rd *radixv1.RadixDeployment, jobDesc *models.JobScheduleDescription) (*batchv1.Job, error) {
	appName := rd.ObjectMeta.Labels[kube.RadixAppLabel]
	timestamp := time.Now().Format("20060102150405")
	jobTag := strings.ToLower(utils.RandString(8))
	componentName := jobDesc.ComponentName
	//environment := jobDesc.Environment
	radixComponent := getRadixComponentBy(rd, componentName)
	if radixComponent == nil {
		return nil, errors.New(fmt.Sprintf("radix component %s not found in Radix Deployment %s", componentName, rd.Name))
	}
	imageTag := radixComponent.Image
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
				},
				Spec: corev1.PodSpec{
					RestartPolicy: "Never",
					//InitContainers: initContainers,
					Containers: jh.kubeUtil.getContainers(rd, radixComponent),
					Volumes: []corev1.Volume{
						{
							Name: git.BuildContextVolumeName,
						},
						corev1.Volume{
							Name: git.GitSSHKeyVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  git.GitSSHKeyVolumeName,
									DefaultMode: &defaultMode,
								},
							},
						},
						{
							Name: azureServicePrincipleSecretName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: azureServicePrincipleSecretName,
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

func getRadixComponentBy(rd *radixv1.RadixDeployment, componentName string) *radixv1.RadixDeployComponent {
	for _, component := range rd.Spec.Components {
		if strings.EqualFold(component.Name, componentName) {
			return &component
		}
	}
	return nil
}

func (jh *jobHandler) getContainers(rd *radixv1.RadixDeployment, radixComponent *radixv1.RadixDeployComponent) *[]corev1.Container {
	container := corev1.Container{
		Name:         radixComponent.Name,
		Image:        radixComponent.Image,
		Env:          radixComponent.EnvironmentVariables,
		VolumeMounts: getVolumeMounts(radixComponent),
	}
	if radixComponent.AlwaysPullImageOnDeploy {
		container.ImagePullPolicy = corev1.PullAlways
	}
	containers := []corev1.Container{container}
	return &containers
}

func getVolumeMounts(deployComponent *radixv1.RadixDeployComponent) []corev1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, 0)

	if len(deployComponent.VolumeMounts) > 0 {
		for _, volumeMount := range deployComponent.VolumeMounts {
			if volumeMount.Type == radixv1.MountTypeBlob {
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name:      fmt.Sprintf(volumeName, deployComponent.Name, volumeMount.Name),
					MountPath: volumeMount.Path,
				})
			}
		}
	}

	return volumeMounts
}

func (jh *jobHandler) getEnvironmentVariables(radixEnvVars radixv1.EnvVarsMap, radixSecrets []string, isPublic bool, ports []v1.ComponentPort, radixDeployName, namespace, currentEnvironment, appName, componentName string) []corev1.EnvVar {
	var environmentVariables = appendAppEnvVariables(radixDeployName, radixEnvVars)
	environmentVariables = *jh.kubeUtil.appendDefaultVariables(currentEnvironment, environmentVariables, isPublic, namespace, appName, componentName, ports)

	// secrets
	if radixSecrets != nil {
		for _, v := range radixSecrets {
			componentSecretName := utils.GetComponentSecretName(componentName)
			secretKeySelector := corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: componentSecretName,
				},
				Key: v,
			}
			envVarSource := corev1.EnvVarSource{
				SecretKeyRef: &secretKeySelector,
			}
			secretEnvVar := corev1.EnvVar{
				Name:      v,
				ValueFrom: &envVarSource,
			}
			environmentVariables = append(environmentVariables, secretEnvVar)
		}
	} else {
		log.Debugf("No secret is set for this RadixDeployment %s", radixDeployName)
	}

	return environmentVariables
}

func appendAppEnvVariables(radixDeployName string, radixEnvVars radixv1.EnvVarsMap) []corev1.EnvVar {
	var environmentVariables []corev1.EnvVar
	if radixEnvVars != nil {
		// map is not sorted, which lead to random order of env variable in deployment
		// during stop/start/restart of a single component this lead to restart of several other components
		var keys []string
		for k := range radixEnvVars {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// environmentVariables
		for _, key := range keys {
			value := radixEnvVars[key]
			envVar := corev1.EnvVar{
				Name:  key,
				Value: value,
			}
			environmentVariables = append(environmentVariables, envVar)
		}
	} else {
		log.Debugf("No environment variable is set for this RadixDeployment %s", radixDeployName)
	}
	return environmentVariables
}

func (jh *jobHandler) appendDefaultVariables(currentEnvironment string, environmentVariables []corev1.EnvVar, isPublic bool, namespace, appName, componentName string, ports []radixv1.ComponentPort) []corev1.EnvVar {
	clusterName, err := *jh.kubeUtil.KubeClient().GetClusterName()
	if err != nil {
		return environmentVariables
	}

	dnsZone := os.Getenv(defaults.OperatorDNSZoneEnvironmentVariable)
	if dnsZone == "" {
		return nil
	}

	clusterType := os.Getenv(defaults.OperatorClusterTypeEnvironmentVariable)
	if clusterType != "" {
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.RadixClusterTypeEnvironmentVariable,
			Value: clusterType,
		})
	}

	containerRegistry, err := deploy.kubeutil.GetContainerRegistry()
	if err != nil {
		return environmentVariables
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.ContainerRegistryEnvironmentVariable,
		Value: containerRegistry,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.RadixDNSZoneEnvironmentVariable,
		Value: dnsZone,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.ClusternameEnvironmentVariable,
		Value: clusterName,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.EnvironmentnameEnvironmentVariable,
		Value: currentEnvironment,
	})

	if isPublic {
		canonicalHostName := getHostName(componentName, namespace, clusterName, dnsZone)
		publicHostName := ""

		if isActiveCluster(clusterName) {
			publicHostName = getActiveClusterHostName(componentName, namespace)
		} else {
			publicHostName = canonicalHostName
		}

		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.PublicEndpointEnvironmentVariable,
			Value: publicHostName,
		})
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.CanonicalEndpointEnvironmentVariable,
			Value: canonicalHostName,
		})
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.RadixAppEnvironmentVariable,
		Value: appName,
	})

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.RadixComponentEnvironmentVariable,
		Value: componentName,
	})

	if len(ports) > 0 {
		portNumbers, portNames := getPortNumbersAndNamesString(ports)
		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.RadixPortsEnvironmentVariable,
			Value: portNumbers,
		})

		environmentVariables = append(environmentVariables, corev1.EnvVar{
			Name:  defaults.RadixPortNamesEnvironmentVariable,
			Value: portNames,
		})
	}

	environmentVariables = append(environmentVariables, corev1.EnvVar{
		Name:  defaults.RadixCommitHashEnvironmentVariable,
		Value: deploy.radixDeployment.Labels[kube.RadixCommitLabel],
	})

	return environmentVariables
}
