package jobs

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	commonUtils "github.com/equinor/radix-common/utils"
	jobErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/api/v1"
	"github.com/equinor/radix-job-scheduler/models"
	modelsV1 "github.com/equinor/radix-job-scheduler/models/v1"
	defaultsV1 "github.com/equinor/radix-job-scheduler/models/v1/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils/slice"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type jobHandler struct {
	common *v1.Handler
}

type JobHandler interface {
	//GetJobs Get status of all jobs
	GetJobs() ([]modelsV1.JobStatus, error)
	//GetJob Get status of a job
	GetJob(name string) (*modelsV1.JobStatus, error)
	//CreateJob Create a job with parameters. `batchName` is optional.
	CreateJob(jobScheduleDescription *models.JobScheduleDescription, batchName string) (*modelsV1.JobStatus, error)
	//MaintainHistoryLimit Delete outdated jobs
	MaintainHistoryLimit() error
	//DeleteJob Delete a job
	DeleteJob(jobName string) error
}

// New Constructor for job handler
func New(env *models.Env, kube *kube.Kube) JobHandler {
	return &jobHandler{
		common: &v1.Handler{
			Kube:        kube,
			KubeClient:  kube.KubeClient(),
			RadixClient: kube.RadixClient(),
			Env:         env,
		},
	}
}

// GetJobs Get status of all jobs
func (handler *jobHandler) GetJobs() ([]modelsV1.JobStatus, error) {
	log.Debugf("Get Jobs for namespace: %s", handler.common.Env.RadixDeploymentNamespace)

	kubeJobs, err := handler.getAllJobs()
	if err != nil {
		return nil, err
	}

	pods, err := handler.getJobPods("")
	if err != nil {
		return nil, err
	}
	podsMap := v1.GetPodsToJobNameMap(pods)
	jobs := make([]modelsV1.JobStatus, len(kubeJobs))
	for idx, k8sJob := range kubeJobs {
		jobs[idx] = *GetJobStatusFromJob(handler.common.KubeClient, k8sJob, podsMap[k8sJob.Name])
	}

	log.Debugf("Found %v jobs for namespace %s", len(jobs), handler.common.Env.RadixDeploymentNamespace)
	return jobs, nil
}

// GetJob Get status of a job
func (handler *jobHandler) GetJob(jobName string) (*modelsV1.JobStatus, error) {
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
	batchName string) (*modelsV1.JobStatus, error) {
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
	//appName := rd.Spec.AppName
	//namespace := handler.common.Env.RadixDeploymentNamespace
	//var jobComponentConfig *models.RadixJobComponentConfig
	//if jobScheduleDescription != nil {
	//	jobComponentConfig = &jobScheduleDescription.RadixJobComponentConfig
	//}
	//TODO

	return nil, nil
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

func (handler *jobHandler) getAllJobs() (modelsV1.JobList, error) {
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

func getLabelSelectorForJobComponent(componentName string) string {
	return labels.SelectorFromSet(map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
	}).String()
}

func getLabelSelectorForJobPods(jobName string) string {
	return labels.SelectorFromSet(map[string]string{
		defaultsV1.K8sJobNameLabel: jobName,
	}).String()
}
