package jobs

import (
	"context"
	"sort"

	"github.com/equinor/radix-common/utils"
	jobErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/api/v1"
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"github.com/equinor/radix-job-scheduler/models"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	defaultsv1 "github.com/equinor/radix-job-scheduler/models/v1/defaults"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
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
	GetJobs() ([]modelsv1.JobStatus, error)
	//GetJob Get status of a job
	GetJob(string) (*modelsv1.JobStatus, error)
	//CreateJob Create a job with parameters
	CreateJob(*models.JobScheduleDescription) (*modelsv1.JobStatus, error)
	//MaintainHistoryLimit Delete outdated jobs
	MaintainHistoryLimit() error
	//DeleteJob Delete a job
	DeleteJob(string) error
}

type completedBatchOrJobVersioned struct {
	jobNameV1      string
	batchNameV2    string
	completionTime string
}

// New Constructor for job handler
func New(env *models.Env, kube *kube.Kube) JobHandler {
	kubeClient := kube.KubeClient()
	radixClient := kube.RadixClient()
	return &jobHandler{
		common: &v1.Handler{
			Kube:         kube,
			KubeClient:   kubeClient,
			RadixClient:  radixClient,
			Env:          env,
			HandlerApiV2: apiv2.New(env, kube, kubeClient, radixClient),
		},
	}
}

// GetJobs Get status of all jobs
func (handler *jobHandler) GetJobs() ([]modelsv1.JobStatus, error) {
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
	jobs := make([]modelsv1.JobStatus, len(kubeJobs))
	for idx, k8sJob := range kubeJobs {
		jobs[idx] = *GetJobStatusFromJob(handler.common.KubeClient, k8sJob, podsMap[k8sJob.Name])
	}

	log.Debugf("Found %v jobs for namespace %s", len(jobs), handler.common.Env.RadixDeploymentNamespace)
	return jobs, nil
}

// GetJob Get status of a job
func (handler *jobHandler) GetJob(jobName string) (*modelsv1.JobStatus, error) {
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
func (handler *jobHandler) CreateJob(jobScheduleDescription *models.JobScheduleDescription) (*modelsv1.JobStatus, error) {
	//TODO remove batchName ?
	log.Debugf("create job for namespace: %s", handler.common.Env.RadixDeploymentNamespace)
	radixBatch, err := handler.common.HandlerApiV2.CreateRadixBatchSingleJob(jobScheduleDescription)
	if err != nil {
		return nil, err
	}
	return GetSingleJobStatusFromRadixBatchJob(radixBatch), nil
}

// DeleteJob Delete a job
func (handler *jobHandler) DeleteJob(jobName string) error {
	log.Debugf("delete job %s for namespace: %s", jobName, handler.common.Env.RadixDeploymentNamespace)
	return handler.garbageCollectJob(jobName)
}

// MaintainHistoryLimit Delete outdated jobs
func (handler *jobHandler) MaintainHistoryLimit() error {
	completedRadixBatches, err := handler.common.HandlerApiV2.GetCompletedRadixBatchesSortedByCompletionTimeAsc()
	if err != nil {
		return err
	}
	jobList, err := handler.getAllJobs()
	if err != nil {
		return err
	}

	completedBatches := convertRadixBatchesToCompletedBatchVersioned(completedRadixBatches.SucceededSingleJobs)
	historyLimit := handler.common.Env.RadixJobSchedulersPerEnvironmentHistoryLimit
	log.Debug("maintain history limit for succeeded batches")
	succeededJobs := jobList.Where(func(j *batchv1.Job) bool {
		_, batchNameLabelExists := j.ObjectMeta.Labels[kube.RadixBatchNameLabel]
		return j.Status.Succeeded > 0 && !batchNameLabelExists
	})
	completedBatches = append(completedBatches, convertBatchJobsToCompletedBatchVersioned(succeededJobs)...)
	if err = handler.maintainHistoryLimitForJobs(completedBatches,
		historyLimit); err != nil {
		return err
	}

	completedBatches = convertRadixBatchesToCompletedBatchVersioned(completedRadixBatches.NotSucceededSingleJobs)
	log.Debug("maintain history limit for failed jobs")
	failedJobs := jobList.Where(func(j *batchv1.Job) bool {
		_, batchNameLabelExists := j.ObjectMeta.Labels[kube.RadixBatchNameLabel]
		return j.Status.Failed > 0 && !batchNameLabelExists
	})
	completedBatches = append(completedBatches, convertBatchJobsToCompletedBatchVersioned(failedJobs)...)
	if err = handler.maintainHistoryLimitForJobs(completedBatches,
		historyLimit); err != nil {
		return err
	}
	err = handler.common.HandlerApiV2.GarbageCollectPayloadSecrets()
	if err != nil {
		return err
	}
	return nil
}

func convertRadixBatchesToCompletedBatchVersioned(radixBatches []*modelsv2.RadixBatch) []completedBatchOrJobVersioned {
	var completedBatches []completedBatchOrJobVersioned
	for _, radixBatch := range radixBatches {
		completedBatches = append(completedBatches, completedBatchOrJobVersioned{
			batchNameV2:    radixBatch.Name,
			completionTime: radixBatch.Ended,
		})
	}
	return completedBatches
}

func convertBatchJobsToCompletedBatchVersioned(jobs modelsv1.JobList) []completedBatchOrJobVersioned {
	var completedBatches []completedBatchOrJobVersioned
	for _, job := range jobs {
		completedBatches = append(completedBatches, completedBatchOrJobVersioned{
			jobNameV1:      job.GetName(),
			completionTime: utils.FormatTime(job.Status.CompletionTime),
		})
	}
	return completedBatches
}

func (handler *jobHandler) maintainHistoryLimitForJobs(completedBatchesVersioned []completedBatchOrJobVersioned, historyLimit int) error {
	numToDelete := len(completedBatchesVersioned) - historyLimit
	if numToDelete <= 0 {
		log.Debug("no history batches to delete")
		return nil
	}
	log.Debugf("history batches to delete: %v", numToDelete)

	sortedCompletedBatchesVersioned := sortCompletedBatchesByCompletionTimeAsc(completedBatchesVersioned)
	for i := 0; i < numToDelete; i++ {
		batchVersioned := sortedCompletedBatchesVersioned[i]
		if len(batchVersioned.jobNameV1) > 0 {
			log.Debugf("deleting job %s", batchVersioned.jobNameV1)
			if err := handler.DeleteJob(batchVersioned.jobNameV1); err != nil {
				return err
			}
			continue
		}
		log.Debugf("deleting batch for simple job %s", batchVersioned.batchNameV2)
		if err := handler.common.HandlerApiV2.DeleteRadixBatch(batchVersioned.batchNameV2); err != nil {
			return err
		}
	}
	return nil
}

func sortCompletedBatchesByCompletionTimeAsc(completedBatchesVersioned []completedBatchOrJobVersioned) []completedBatchOrJobVersioned {
	sort.Slice(completedBatchesVersioned, func(i, j int) bool {
		batch1 := (completedBatchesVersioned)[i]
		batch2 := (completedBatchesVersioned)[j]
		return isCompletedBatch1CompletedBefore2(batch1, batch2)
	})
	return completedBatchesVersioned
}

func isCompletedBatch1CompletedBefore2(batchVersioned1 completedBatchOrJobVersioned, batchVersioned2 completedBatchOrJobVersioned) bool {
	return batchVersioned1.completionTime < batchVersioned2.completionTime
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

func (handler *jobHandler) getAllJobs() (modelsv1.JobList, error) {
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
		defaultsv1.K8sJobNameLabel: jobName,
	}).String()
}
