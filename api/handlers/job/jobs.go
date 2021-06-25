package job

import (
	"context"

	jobErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (jh *jobHandler) createJob(jobName string, jobComponent *v1.RadixDeployJobComponent, rd *v1.RadixDeployment, payloadSecret *corev1.Secret, jobScheduleDescription *models.JobScheduleDescription) (*batchv1.Job, error) {
	var jobComponentConfig *models.RadixJobComponentConfig
	if jobScheduleDescription != nil {
		jobComponentConfig = &jobScheduleDescription.RadixJobComponentConfig
	}

	job, err := jh.buildJobSpec(jobName, rd, jobComponent, payloadSecret, jh.kube, jobComponentConfig)
	if err != nil {
		return nil, err
	}
	createdJob, err := jh.kubeClient.BatchV1().Jobs(jh.env.RadixDeploymentNamespace).Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdJob, nil
}

func (jh *jobHandler) deleteJob(job *batchv1.Job) error {
	fg := metav1.DeletePropagationBackground
	return jh.kubeClient.BatchV1().Jobs(job.Namespace).Delete(context.TODO(), job.Name, metav1.DeleteOptions{PropagationPolicy: &fg})
}

func (jh *jobHandler) getJobByName(jobName string) (*batchv1.Job, error) {
	jobs, err := jh.getAllJobs()
	if err != nil {
		return nil, err
	}

	jobs = jobs.Where(func(j *batchv1.Job) bool { return j.Name == jobName })

	if len(jobs) == 1 {
		return jobs[0], nil
	}

	return nil, jobErrors.NewNotFound("job", jobName)
}

func (jh *jobHandler) getAllJobs() (jobList, error) {
	kubeJobs, err := jh.kubeClient.
		BatchV1().
		Jobs(jh.env.RadixDeploymentNamespace).
		List(
			context.TODO(),
			metav1.ListOptions{
				LabelSelector: getLabelSelectorForJobComponent(jh.env.RadixComponentName),
			},
		)

	if err != nil {
		return nil, err
	}

	return jobList(slice.PointersOf(kubeJobs.Items).([]*batchv1.Job)), nil
}

func (jh *jobHandler) buildJobSpec(jobName string, rd *v1.RadixDeployment, radixJobComponent *v1.RadixDeployJobComponent, payloadSecret *corev1.Secret, kubeutil *kube.Kube, jobComponentConfig *models.RadixJobComponentConfig) (*batchv1.Job, error) {
	podSecurityContext := getSecurityContextForPod(radixJobComponent.RunAsNonRoot)
	volumes, err := jh.getVolumes(rd.ObjectMeta.Namespace, rd.Spec.Environment, radixJobComponent, payloadSecret)
	if err != nil {
		return nil, err
	}
	containers := getContainers(kubeutil, rd, radixJobComponent, payloadSecret, jobComponentConfig)

	var affinity *corev1.Affinity
	if jobComponentConfig != nil && jobComponentConfig.Node != nil {
		affinity = operatorUtils.GetPodSpecAffinity(jobComponentConfig.Node)
	} else {
		affinity = operatorUtils.GetPodSpecAffinity(&radixJobComponent.Node)
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
					Containers:       containers,
					Volumes:          volumes,
					SecurityContext:  podSecurityContext,
					RestartPolicy:    corev1.RestartPolicyNever,
					ImagePullSecrets: rd.Spec.ImagePullSecrets,
					Affinity:         affinity,
				},
			},
		},
	}, nil
}

func getContainers(kube *kube.Kube, rd *radixv1.RadixDeployment, radixJobComponent *radixv1.RadixDeployJobComponent, payloadSecret *corev1.Secret, jobComponentConfig *models.RadixJobComponentConfig) []corev1.Container {
	environmentVariables := deployment.GetEnvironmentVariablesFrom(rd.Spec.AppName, rd, radixJobComponent)
	ports := getContainerPorts(radixJobComponent)
	containerSecurityContext := getSecurityContextForContainer(radixJobComponent.RunAsNonRoot)
	volumeMounts := getVolumeMounts(radixJobComponent, payloadSecret)
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

	return []corev1.Container{container}
}

func getVolumeMounts(radixJobComponent *radixv1.RadixDeployJobComponent, payloadSecret *corev1.Secret) []corev1.VolumeMount {
	volumeMounts := deployment.GetRadixDeployComponentVolumeMounts(radixJobComponent)

	if payloadSecret != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      JOB_PAYLOAD_PROPERTY_NAME,
			ReadOnly:  true,
			MountPath: radixJobComponent.Payload.Path,
		})
	}

	return volumeMounts
}

func (jh *jobHandler) getVolumes(namespace, environment string, radixJobComponent *v1.RadixDeployJobComponent, payloadSecret *corev1.Secret) ([]corev1.Volume, error) {
	volumes, err := deployment.GetVolumes(jh.kubeClient, namespace, environment, radixJobComponent.Name, radixJobComponent.VolumeMounts)
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
		Name: JOB_PAYLOAD_PROPERTY_NAME,
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
			ContainerPort: int32(v.Port),
		}
		ports = append(ports, containerPort)
	}
	return ports
}

func getSecurityContextForContainer(runAsNonRoot bool) *corev1.SecurityContext {
	// runAsNonRoot is false by default
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: operatorUtils.BoolPtr(false),
		Privileged:               operatorUtils.BoolPtr(false),
		RunAsNonRoot:             operatorUtils.BoolPtr(runAsNonRoot),
	}
}

func getSecurityContextForPod(runAsNonRoot bool) *corev1.PodSecurityContext {
	// runAsNonRoot is false by default
	return &corev1.PodSecurityContext{
		RunAsNonRoot: operatorUtils.BoolPtr(runAsNonRoot),
	}
}

func getLabelSelectorForJobComponent(componentName string) string {
	return labels.SelectorFromSet(labels.Set(map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
	})).String()
}
