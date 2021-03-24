package job

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	JOB_PAYLOAD_PROPERTY_NAME          = "payload"
	OBSOLETE_RADIX_APP_NAME_LABEL_NAME = "radix-app-name"
)

type Handler interface {
	GetJobs() (*[]models.JobStatus, error)
	GetJob(name string) (*models.JobStatus, error)
	CreateJob(jobScheduleDescription *models.JobScheduleDescription) (*models.JobStatus, error)
	MaintainHistoryLimit() error
	DeleteJob(jobName string) error
}

type jobHandler struct {
	kube        *kube.Kube
	env         *models.Env
	kubeClient  kubernetes.Interface
	radixClient radixclient.Interface
}

func New(env *models.Env, kube *kube.Kube, kubeClient kubernetes.Interface, radixClient radixclient.Interface) Handler {
	return &jobHandler{
		kube:        kube,
		kubeClient:  kubeClient,
		radixClient: radixClient,
		env:         env,
	}
}

func (jh *jobHandler) GetJobs() (*[]models.JobStatus, error) {
	log.Debugf("Get Jobs for namespace: %s", jh.env.RadixDeploymentNamespace)

	kubeJobs, err := jh.getAllJobs()
	if err != nil {
		return nil, err
	}

	jobs := make([]models.JobStatus, len(kubeJobs.Items))
	for idx, k8sJob := range kubeJobs.Items {
		jobs[idx] = *models.GetJobStatusFromJob(&k8sJob)
	}

	log.Debugf("Found %v jobs for namespace %s", len(jobs), jh.env.RadixDeploymentNamespace)
	return &jobs, nil
}

func (jh *jobHandler) GetJob(jobName string) (*models.JobStatus, error) {
	log.Debugf("get jobs for namespace: %s", jh.env.RadixDeploymentNamespace)
	job, err := jh.getJobByName(jobName)
	if err != nil {
		return nil, err
	}
	log.Debugf("found Job %s for namespace: %s", jobName, jh.env.RadixDeploymentNamespace)
	jobStatus := models.GetJobStatusFromJob(job)
	return jobStatus, nil
}

func (jh *jobHandler) CreateJob(jobScheduleDescription *models.JobScheduleDescription) (*models.JobStatus, error) {
	log.Debugf("create job for namespace: %s", jh.env.RadixDeploymentNamespace)

	radixDeployment, err := jh.radixClient.RadixV1().RadixDeployments(jh.env.RadixDeploymentNamespace).Get(jh.env.RadixDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to get Radix Deployment %s for namespace: %s", jh.env.RadixDeploymentName, jh.env.RadixDeploymentNamespace))
	}

	jobComponent := radixDeployment.GetJobComponentByName(jh.env.RadixComponentName)
	if jobComponent == nil {
		return nil, errors.New(fmt.Sprintf("job component %s does not exist in Radix deployment %s", jh.env.RadixComponentName, radixDeployment.Name))
	}

	jobName := generateJobName(jobComponent)

	var payloadSecret *corev1.Secret
	if isPayloadDefinedForJobComponent(jobComponent) {
		if payloadSecret, err = jh.createPayloadSecret(jobName, jobComponent, radixDeployment, jobScheduleDescription); err != nil {
			return nil, err
		}
	}

	job, err := jh.createJob(jobName, jobComponent, radixDeployment, payloadSecret)
	if err != nil {
		return nil, err
	}

	log.Debug(fmt.Sprintf("created job %s for component %s, environment %s, in namespace: %s", job.Name, jh.env.RadixComponentName, radixDeployment.Spec.Environment, jh.env.RadixDeploymentNamespace))
	return models.GetJobStatusFromJob(job), nil
}

func (jh *jobHandler) DeleteJob(jobName string) (err error) {
	log.Debugf("delete job %s for namespace: %s", jobName, jh.env.RadixDeploymentNamespace)

	job, err := jh.getJobByName(jobName)
	if err != nil {
		return
	}

	secrets, err := jh.getSecretsForJob(jobName)
	if err != nil {
		return
	}

	for _, secret := range secrets.Items {
		if err = jh.deleteSecret(&secret); err != nil {
			return err
		}
	}

	jh.deleteJob(job)
	if err != nil {
		return
	}
	log.Debugf("deleted job %s for namespace: %s", jobName, jh.env.RadixDeploymentNamespace)
	return
}

func (jh *jobHandler) MaintainHistoryLimit() error {
	namespace := jh.env.RadixDeploymentNamespace
	jobList, err := jh.getCompletedJobs()
	if err != nil {
		log.Warnf("failed to get list of jobs: %v", err)
		return err
	}

	numToDelete := len(jobList.Items) - jh.env.RadixJobSchedulersPerEnvironmentHistoryLimit
	if numToDelete <= 0 {
		log.Debug("no history jobs to delete")
		return nil
	}
	log.Debugf("history jobs to delete: %v", numToDelete)

	sortedJobs := sortRJSchByCompletionTimeAsc(jobList.Items)
	for i := 0; i < numToDelete; i++ {
		job := sortedJobs[i]
		log.Infof("Removing Radix Job Schedule %s from %s", job.Name, namespace)
		if err := jh.deleteJob(&job); err != nil {
			log.Warnf("failed to delete old Radix Job Scheduler %s: %v", job.Name, err)
		}
	}
	return nil
}

func sortRJSchByCompletionTimeAsc(jobs []batchv1.Job) []batchv1.Job {
	sort.Slice(jobs, func(i, j int) bool {
		job1 := (jobs)[i]
		job2 := (jobs)[j]
		return isRJS1CompletedBeforeRJS2(&job1, &job2)
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

func generateJobName(jobComponent *v1.RadixDeployJobComponent) string {
	timestamp := time.Now().Format("20060102150405")
	jobTag := strings.ToLower(utils.RandString(8))
	return fmt.Sprintf("%s-%s-%s", jobComponent.Name, timestamp, jobTag)
}

func isPayloadDefinedForJobComponent(radixJobComponent *radixv1.RadixDeployJobComponent) bool {
	return radixJobComponent.Payload != nil && strings.TrimSpace(radixJobComponent.Payload.Path) != ""
}
