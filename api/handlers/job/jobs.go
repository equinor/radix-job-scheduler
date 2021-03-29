package job

import (
	strconv "strconv"

	jobErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-operator/pkg/apis/deployment"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/numbers"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

func (jh *jobHandler) createJob(jobName string, jobComponent *v1.RadixDeployJobComponent, rd *v1.RadixDeployment, payloadSecret *corev1.Secret) (*batchv1.Job, error) {
	job := buildJobSpec(jobName, rd, jobComponent, payloadSecret, jh.kube)
	createdJob, err := jh.kubeClient.BatchV1().Jobs(jh.env.RadixDeploymentNamespace).Create(job)
	if err != nil {
		return nil, err
	}

	return createdJob, nil
}

func (jh *jobHandler) deleteJob(job *batchv1.Job) error {
	fg := metav1.DeletePropagationBackground
	return jh.kubeClient.BatchV1().Jobs(job.Namespace).Delete(job.Name, &metav1.DeleteOptions{PropagationPolicy: &fg})
}

func (jh *jobHandler) getCompletedJobs() (*batchv1.JobList, error) {
	return jh.getJobsWithFieldSelector(getFieldSelectorForCompletedJobComponent())
}

func (jh *jobHandler) getAllJobs() (*batchv1.JobList, error) {
	return jh.getJobsWithFieldSelector("")
}

func (jh *jobHandler) getJobByName(jobName string) (*batchv1.Job, error) {
	jobs, err := jh.getJobsWithFieldSelector("")
	if err != nil {
		return nil, err
	}

	for _, job := range jobs.Items {
		if job.Name == jobName {
			return &job, nil
		}
	}

	return nil, jobErrors.NewNotFound("job", jobName)
}

func (jh *jobHandler) getJobsWithFieldSelector(fieldSelector string) (*batchv1.JobList, error) {
	return jh.kubeClient.BatchV1().Jobs(jh.env.RadixDeploymentNamespace).List(metav1.ListOptions{
		LabelSelector: getLabelSelectorForJobComponent(jh.env.RadixComponentName),
		FieldSelector: fieldSelector,
	})
}

func buildJobSpec(jobName string, rd *radixv1.RadixDeployment, radixJobComponent *radixv1.RadixDeployJobComponent, payloadSecret *corev1.Secret, kubeutil *kube.Kube) *batchv1.Job {
	podSecurityContext := getSecurityContextForPod(radixJobComponent.RunAsNonRoot)
	volumes := getVolumes(rd.ObjectMeta.Namespace, rd.Spec.Environment, jobName, rd, radixJobComponent, payloadSecret)
	containers := getContainers(kubeutil, rd, radixJobComponent, payloadSecret)

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
						kube.RadixComponentLabel: radixJobComponent.Name,
						kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
					},
					Namespace: rd.ObjectMeta.Namespace,
				},
				Spec: corev1.PodSpec{
					Containers:      containers,
					Volumes:         volumes,
					SecurityContext: podSecurityContext,
					RestartPolicy:   corev1.RestartPolicyNever, //TODO: decide what to do with failed job
				},
			},
		},
	}
}

func getContainers(kube *kube.Kube, rd *radixv1.RadixDeployment, radixJobComponent *radixv1.RadixDeployJobComponent, payloadSecret *corev1.Secret) []corev1.Container {
	environmentVariables := deployment.GetEnvironmentVariablesFrom(rd.Spec.AppName, kube, rd, radixJobComponent)
	ports := getContainerPorts(radixJobComponent)
	containerSecurityContext := getSecurityContextForContainer(radixJobComponent.RunAsNonRoot)
	volumeMounts := getVolumeMounts(radixJobComponent, payloadSecret)

	container := corev1.Container{
		Name:  radixJobComponent.Name,
		Image: radixJobComponent.Image,
		//TODO: add setting to be PullAlways, PullIfNotPresent, PullNever. Image should be pulled on job's InitContainer
		ImagePullPolicy: corev1.PullAlways,
		Env:             environmentVariables,
		Ports:           ports,
		VolumeMounts:    volumeMounts,
		SecurityContext: containerSecurityContext,
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

func getVolumes(namespace, environment, jobName string, rd *radixv1.RadixDeployment, radixJobComponent *radixv1.RadixDeployJobComponent, payloadSecret *corev1.Secret) []corev1.Volume {
	volumes := deployment.GetVolumes(namespace, environment, radixJobComponent.Name, radixJobComponent.VolumeMounts)

	if payloadSecret != nil {
		volumes = append(volumes, *getPayloadVolume(payloadSecret.Name))
	}

	return volumes
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
		AllowPrivilegeEscalation: utils.BoolPtr(false),
		Privileged:               utils.BoolPtr(false),
		RunAsNonRoot:             utils.BoolPtr(runAsNonRoot),
	}
}

func getSecurityContextForPod(runAsNonRoot bool) *corev1.PodSecurityContext {
	// runAsNonRoot is false by default
	return &corev1.PodSecurityContext{
		RunAsNonRoot: utils.BoolPtr(runAsNonRoot),
	}
}

func getLabelSelectorForJobComponent(componentName string) string {
	return labels.SelectorFromSet(labels.Set(map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
	})).String()
}

func getFieldSelectorForCompletedJobComponent() string {
	return fields.SelectorFromSet(fields.Set{"status.successful": strconv.Itoa(1)}).String()
}
