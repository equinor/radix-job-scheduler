package job

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	jobErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	JOB_PAYLOAD_PROPERTY_NAME          = "payload"
	OBSOLETE_RADIX_APP_NAME_LABEL_NAME = "radix-app-name"
)

type Handler interface {
	GetJobs() ([]models.JobStatus, error)
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

func (jh *jobHandler) GetJobs() ([]models.JobStatus, error) {
	log.Debugf("Get Jobs for namespace: %s", jh.env.RadixDeploymentNamespace)

	kubeJobs, err := jh.getAllJobs()
	if err != nil {
		return nil, err
	}

	jobs := make([]models.JobStatus, len(kubeJobs))
	for idx, k8sJob := range kubeJobs {
		jobs[idx] = *models.GetJobStatusFromJob(k8sJob)
	}

	log.Debugf("Found %v jobs for namespace %s", len(jobs), jh.env.RadixDeploymentNamespace)
	return jobs, nil
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

	radixDeployment, err := jh.radixClient.RadixV1().RadixDeployments(jh.env.RadixDeploymentNamespace).Get(context.TODO(), jh.env.RadixDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, jobErrors.NewNotFound("radix deployment", jh.env.RadixDeploymentName)
	}

	jobComponent := radixDeployment.GetJobComponentByName(jh.env.RadixComponentName)
	if jobComponent == nil {
		return nil, jobErrors.NewNotFound("job component", jh.env.RadixComponentName)
	}

	jobName := generateJobName(jobComponent)

	payloadSecret, err := jh.createPayloadSecret(jobName, jobComponent, radixDeployment, jobScheduleDescription)
	if err != nil {
		return nil, jobErrors.NewFromError(err)
	}

	if err = jh.createService(jobName, jobComponent, radixDeployment); err != nil {
		return nil, jobErrors.NewFromError(err)
	}

	job, err := jh.createJob(jobName, jobComponent, radixDeployment, payloadSecret, jobScheduleDescription)
	if err != nil {
		return nil, jobErrors.NewFromError(err)
	}

	log.Debug(fmt.Sprintf("created job %s for component %s, environment %s, in namespace: %s", job.Name, jh.env.RadixComponentName, radixDeployment.Spec.Environment, jh.env.RadixDeploymentNamespace))
	return models.GetJobStatusFromJob(job), nil
}

func (jh *jobHandler) DeleteJob(jobName string) error {
	log.Debugf("delete job %s for namespace: %s", jobName, jh.env.RadixDeploymentNamespace)
	return jh.garbageCollectJob(jobName)
}

func (jh *jobHandler) MaintainHistoryLimit() error {
	jobList, err := jh.getAllJobs()
	if err != nil {
		return err
	}

	log.Debug("maintain history limit for succeeded jobs")
	succeededJobs := jobList.Where(func(j *batchv1.Job) bool { return j.Status.Succeeded > 0 })
	if err = jh.maintainHistoryLimitForJobs(succeededJobs, jh.env.RadixJobSchedulersPerEnvironmentHistoryLimit); err != nil {
		return err
	}

	log.Debug("maintain history limit for failed jobs")
	failedJobs := jobList.Where(func(j *batchv1.Job) bool { return j.Status.Failed > 0 })
	if err = jh.maintainHistoryLimitForJobs(failedJobs, jh.env.RadixJobSchedulersPerEnvironmentHistoryLimit); err != nil {
		return err
	}

	return nil
}

func (jh *jobHandler) maintainHistoryLimitForJobs(jobs []*batchv1.Job, historyLimit int) error {
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
		if err := jh.garbageCollectJob(job.Name); err != nil {
			return err
		}
	}
	return nil
}

func (jh *jobHandler) garbageCollectJob(jobName string) (err error) {
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
			return
		}
	}

	services, err := jh.getServiceForJob(jobName)
	if err != nil {
		return
	}

	for _, service := range services.Items {
		if err = jh.deleteService(&service); err != nil {
			return
		}
	}

	err = jh.deleteJob(job)
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

func generateJobName(jobComponent *v1.RadixDeployJobComponent) string {
	timestamp := time.Now().Format("20060102150405")
	jobTag := strings.ToLower(utils.RandString(8))
	return fmt.Sprintf("%s-%s-%s", jobComponent.Name, timestamp, jobTag)
}

func isPayloadDefinedForJobComponent(radixJobComponent *radixv1.RadixDeployJobComponent) bool {
	return radixJobComponent.Payload != nil && strings.TrimSpace(radixJobComponent.Payload.Path) != ""
}
