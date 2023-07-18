package jobs

import (
	"context"
	"sort"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	apiv1 "github.com/equinor/radix-job-scheduler/api/v1"
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	defaultsv1 "github.com/equinor/radix-job-scheduler/models/v1/defaults"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type jobHandler struct {
	common *apiv1.Handler
}

type JobHandler interface {
	// GetJobs Get status of all jobs
	GetJobs(context.Context) ([]modelsv1.JobStatus, error)
	// GetJob Get status of a job
	GetJob(context.Context, string) (*modelsv1.JobStatus, error)
	// CreateJob Create a job with parameters
	CreateJob(context.Context, *common.JobScheduleDescription) (*modelsv1.JobStatus, error)
	// MaintainHistoryLimit Delete outdated jobs
	MaintainHistoryLimit(context.Context) error
	// DeleteJob Delete a job
	DeleteJob(context.Context, string) error
	// StopJob Stop a job
	StopJob(context.Context, string) error
}

type completedBatchOrJobVersioned struct {
	jobNameV1      string
	batchNameV2    string
	completionTime string
}

// New Constructor for job handler
func New(kube *kube.Kube, env *models.Env) JobHandler {
	return &jobHandler{
		common: &apiv1.Handler{
			Kube:         kube,
			Env:          env,
			HandlerApiV2: apiv2.New(kube, env),
		},
	}
}

// GetJobs Get status of all jobs
func (handler *jobHandler) GetJobs(ctx context.Context) ([]modelsv1.JobStatus, error) {
	log.Debugf("Get Jobs for namespace: %s", handler.common.Env.RadixDeploymentNamespace)

	// Use Kubernetes jobs for backward compatibility
	kubeJobs, err := handler.getAllJobs(ctx)
	if err != nil {
		return nil, err
	}

	pods, err := handler.getJobPods(ctx, "")
	if err != nil {
		return nil, err
	}
	podsMap := apiv1.GetPodsToJobNameMap(pods)
	jobStatuses := make([]modelsv1.JobStatus, len(kubeJobs))
	for idx, k8sJob := range kubeJobs {
		jobStatuses[idx] = *GetJobStatusFromJob(ctx, handler.common.Kube.KubeClient(), k8sJob, podsMap[k8sJob.Name])
	}

	// get all single jobs
	radixBatches, err := handler.common.HandlerApiV2.GetRadixBatchSingleJobs(ctx)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	jobStatuses = append(jobStatuses, apiv1.GetJobStatusFromRadixBatchJobsStatuses(radixBatches...)...)

	// get all batch jobs
	radixBatches, err = handler.common.HandlerApiV2.GetRadixBatches(ctx)
	if err != nil {
		return nil, err
	}
	jobStatuses = append(jobStatuses, apiv1.GetJobStatusFromRadixBatchJobsStatuses(radixBatches...)...)

	labelSelectorForAllRadixBatchesPods := apiv1.GetLabelSelectorForAllRadixBatchesPods(handler.common.Env.RadixComponentName)
	eventMessageForPods, batchJobPodsMap, err := handler.common.GetRadixBatchJobMessagesAndPodMaps(ctx, labelSelectorForAllRadixBatchesPods)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(jobStatuses); i++ {
		apiv1.SetBatchJobEventMessageToBatchJobStatus(&jobStatuses[i], batchJobPodsMap, eventMessageForPods)
	}

	log.Debugf("Found %v jobs for namespace %s", len(jobStatuses), handler.common.Env.RadixDeploymentNamespace)
	return jobStatuses, nil
}

// GetJob Get status of a job
func (handler *jobHandler) GetJob(ctx context.Context, jobName string) (*modelsv1.JobStatus, error) {
	log.Debugf("get job %s for namespace: %s", jobName, handler.common.Env.RadixDeploymentNamespace)
	if batchName, _, ok := apiv1.ParseBatchAndJobNameFromScheduledJobName(jobName); ok {
		jobStatus, err := apiv1.GetBatchJob(ctx, handler.common.HandlerApiV2, batchName, jobName)
		if err != nil {
			return nil, err
		}
		labelSelectorForRadixBatchesPods := apiv1.GetLabelSelectorForRadixBatchesPods(handler.common.Env.RadixComponentName, batchName)
		eventMessageForPods, batchJobPodsMap, err := handler.common.GetRadixBatchJobMessagesAndPodMaps(ctx, labelSelectorForRadixBatchesPods)
		if err != nil {
			return nil, err
		}
		apiv1.SetBatchJobEventMessageToBatchJobStatus(jobStatus, batchJobPodsMap, eventMessageForPods)
		return jobStatus, nil
	}

	// Use Kubernetes jobs for backward compatibility
	job, err := handler.getJobByName(ctx, jobName)
	if err != nil {
		return nil, err
	}
	log.Debugf("found Job %s for namespace: %s", jobName, handler.common.Env.RadixDeploymentNamespace)
	pods, err := handler.getJobPods(ctx, job.Name)
	if err != nil {
		return nil, err
	}
	jobStatus := GetJobStatusFromJob(ctx, handler.common.Kube.KubeClient(), job, pods)
	return jobStatus, nil
}

// CreateJob Create a job with parameters
func (handler *jobHandler) CreateJob(ctx context.Context, jobScheduleDescription *common.JobScheduleDescription) (*modelsv1.JobStatus, error) {
	log.Debugf("create job for namespace: %s", handler.common.Env.RadixDeploymentNamespace)
	radixBatch, err := handler.common.HandlerApiV2.CreateRadixBatchSingleJob(ctx, jobScheduleDescription)
	if err != nil {
		return nil, err
	}
	return GetSingleJobStatusFromRadixBatchJob(radixBatch)
}

// DeleteJob Delete a job
func (handler *jobHandler) DeleteJob(ctx context.Context, jobName string) error {
	log.Debugf("delete job %s for namespace: %s", jobName, handler.common.Env.RadixDeploymentNamespace)
	if batchName, _, ok := apiv1.ParseBatchAndJobNameFromScheduledJobName(jobName); ok {
		radixBatch, err := handler.common.HandlerApiV2.GetRadixBatch(ctx, batchName)
		if err == nil && radixBatch.BatchType == string(kube.RadixBatchTypeJob) {
			// only job in a single job batch can be deleted
			return handler.common.HandlerApiV2.DeleteRadixBatch(ctx, batchName)
		}
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	// Use Kubernetes jobs for backward compatibility
	return handler.garbageCollectJob(ctx, jobName)
}

// StopJob Stop a job
func (handler *jobHandler) StopJob(ctx context.Context, jobName string) error {
	log.Debugf("stop the job %s for namespace: %s", jobName, handler.common.Env.RadixDeploymentNamespace)
	return apiv1.StopJob(ctx, handler.common.HandlerApiV2, jobName)
}

// MaintainHistoryLimit Delete outdated jobs
func (handler *jobHandler) MaintainHistoryLimit(ctx context.Context) error {
	completedRadixBatches, err := handler.common.HandlerApiV2.GetCompletedRadixBatchesSortedByCompletionTimeAsc(ctx)
	if err != nil {
		return err
	}
	jobList, err := handler.getAllJobs(ctx)
	if err != nil {
		return err
	}

	completedBatches := convertRadixBatchesToCompletedBatchVersioned(completedRadixBatches.SucceededSingleJobs)
	historyLimit := handler.common.Env.RadixJobSchedulersPerEnvironmentHistoryLimit
	log.Debug("maintain history limit for succeeded batches")
	succeededJobs := slice.FindAll(jobList, func(j *batchv1.Job) bool {
		_, batchNameLabelExists := j.ObjectMeta.Labels[kube.RadixBatchNameLabel]
		return j.Status.Succeeded > 0 && !batchNameLabelExists
	})
	completedBatches = append(completedBatches, convertBatchJobsToCompletedBatchVersioned(succeededJobs)...)
	if err = handler.maintainHistoryLimitForJobs(ctx, completedBatches,
		historyLimit); err != nil {
		return err
	}

	completedBatches = convertRadixBatchesToCompletedBatchVersioned(completedRadixBatches.NotSucceededSingleJobs)
	log.Debug("maintain history limit for failed jobs")
	failedJobs := slice.FindAll(jobList, func(j *batchv1.Job) bool {
		_, batchNameLabelExists := j.ObjectMeta.Labels[kube.RadixBatchNameLabel]
		return j.Status.Failed > 0 && !batchNameLabelExists
	})
	completedBatches = append(completedBatches, convertBatchJobsToCompletedBatchVersioned(failedJobs)...)
	if err = handler.maintainHistoryLimitForJobs(ctx, completedBatches, historyLimit); err != nil {
		return err
	}
	err = handler.common.HandlerApiV2.GarbageCollectPayloadSecrets(ctx)
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

func convertBatchJobsToCompletedBatchVersioned(jobs []*batchv1.Job) []completedBatchOrJobVersioned {
	var completedBatches []completedBatchOrJobVersioned
	for _, job := range jobs {
		completedBatches = append(completedBatches, completedBatchOrJobVersioned{
			jobNameV1:      job.GetName(),
			completionTime: utils.FormatTime(job.Status.CompletionTime),
		})
	}
	return completedBatches
}

func (handler *jobHandler) maintainHistoryLimitForJobs(ctx context.Context, completedBatchesVersioned []completedBatchOrJobVersioned, historyLimit int) error {
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
			if err := handler.DeleteJob(ctx, batchVersioned.jobNameV1); err != nil {
				return err
			}
			continue
		}
		log.Debugf("deleting batch for simple job %s", batchVersioned.batchNameV2)
		if err := handler.common.HandlerApiV2.DeleteRadixBatch(ctx, batchVersioned.batchNameV2); err != nil {
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

func (handler *jobHandler) garbageCollectJob(ctx context.Context, jobName string) (err error) {
	job, err := handler.getJobByName(ctx, jobName)
	if err != nil {
		return
	}

	secrets, err := handler.common.GetSecretsForJob(ctx, jobName)
	if err != nil {
		return
	}

	for _, secret := range secrets.Items {
		if err = handler.common.DeleteSecret(ctx, &secret); err != nil {
			return
		}
	}

	services, err := handler.common.GetServiceForJob(ctx, jobName)
	if err != nil {
		return
	}

	for _, service := range services.Items {
		if err = handler.common.DeleteService(ctx, &service); err != nil {
			return
		}
	}

	err = handler.deleteJob(ctx, job)
	if err != nil {
		return err
	}

	return
}

func (handler *jobHandler) deleteJob(ctx context.Context, job *batchv1.Job) error {
	fg := metav1.DeletePropagationBackground
	return handler.common.Kube.KubeClient().BatchV1().Jobs(job.Namespace).Delete(ctx, job.Name, metav1.DeleteOptions{PropagationPolicy: &fg})
}

func (handler *jobHandler) getJobByName(ctx context.Context, jobName string) (*batchv1.Job, error) {
	allJobs, err := handler.getAllJobs(ctx)
	if err != nil {
		return nil, err
	}

	allJobs = slice.FindAll(allJobs, func(j *batchv1.Job) bool { return j.Name == jobName })

	if len(allJobs) == 1 {
		return allJobs[0], nil
	}

	return nil, apiErrors.NewNotFound("job", jobName)
}

func (handler *jobHandler) getAllJobs(ctx context.Context) ([]*batchv1.Job, error) {
	kubeJobs, err := handler.common.Kube.KubeClient().
		BatchV1().
		Jobs(handler.common.Env.RadixDeploymentNamespace).
		List(
			ctx,
			metav1.ListOptions{
				LabelSelector: getLabelSelectorForJobComponentForObsoleteJobs(handler.common.Env.RadixComponentName),
			},
		)
	if err != nil {
		return nil, err
	}

	var jobs []*batchv1.Job
	for _, job := range kubeJobs.Items {
		job := job
		jobs = append(jobs, &job)
	}
	return jobs, nil
}

// getJobPods jobName is optional, when empty - returns all job-pods for the namespace
func (handler *jobHandler) getJobPods(ctx context.Context, jobName string) ([]corev1.Pod, error) {
	listOptions := metav1.ListOptions{}
	if jobName != "" {
		listOptions.LabelSelector = getLabelSelectorForJobPods(jobName)
	}
	podList, err := handler.common.Kube.KubeClient().
		CoreV1().
		Pods(handler.common.Env.RadixDeploymentNamespace).
		List(
			ctx,
			listOptions,
		)

	if err != nil {
		return nil, err
	}

	return podList.Items, nil
}

func getLabelSelectorForJobComponentForObsoleteJobs(componentName string) string {
	reqNoBatchJobName, _ := labels.NewRequirement(kube.RadixBatchJobNameLabel, selection.DoesNotExist, []string{})
	reqComponentName, _ := labels.NewRequirement(kube.RadixComponentLabel, selection.Equals, []string{componentName})
	reqJobTypeJobScheduler, _ := labels.NewRequirement(kube.RadixJobTypeLabel, selection.Equals, []string{kube.RadixJobTypeJobSchedule})
	return labels.NewSelector().
		Add(*reqNoBatchJobName).
		Add(*reqComponentName).
		Add(*reqJobTypeJobScheduler).
		String()
}

func getLabelSelectorForJobPods(jobName string) string {
	return labels.SelectorFromSet(map[string]string{
		defaultsv1.K8sJobNameLabel: jobName,
	}).String()
}
