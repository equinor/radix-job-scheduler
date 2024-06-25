package apiv2

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"dario.cat/mergo"
	"github.com/equinor/radix-common/utils"
	mergoutils "github.com/equinor/radix-common/utils/mergo"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-job-scheduler/utils/radix/jobs"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const (
	// Max size of the secret description, including description, metadata, base64 encodes secret values, etc.
	maxPayloadSecretSize = 1024 * 512 // 0.5MB
	// Standard secret description, metadata, etc.
	payloadSecretAuxDataSize = 600
	// Each entry in a secret Data has name, etc.
	payloadSecretEntryAuxDataSize = 128
)

var (
	authTransformer mergo.Transformers = mergoutils.CombinedTransformer{Transformers: []mergo.Transformers{mergoutils.BoolPtrTransformer{}}}
	defaultSrc                         = rand.NewSource(time.Now().UnixNano())
)

type handler struct {
	kubeUtil                *kube.Kube
	env                     *models.Env
	radixDeployJobComponent *radixv1.RadixDeployJobComponent
}

type Handler interface {
	// GetRadixBatches Get status of all batches
	GetRadixBatches(ctx context.Context) ([]modelsv2.RadixBatch, error)
	// GetRadixBatchSingleJobs Get status of all single jobs
	GetRadixBatchSingleJobs(ctx context.Context) ([]modelsv2.RadixBatch, error)
	// GetRadixBatch Get a batch
	GetRadixBatch(ctx context.Context, batchName string) (*modelsv2.RadixBatch, error)
	// CreateRadixBatch Create a batch with parameters
	CreateRadixBatch(ctx context.Context, batchScheduleDescription *common.BatchScheduleDescription) (*modelsv2.RadixBatch, error)
	// CopyRadixBatch Copy a batch with deployment and optional parameters
	CopyRadixBatch(ctx context.Context, batchName, deploymentName string) (*modelsv2.RadixBatch, error)
	// CreateRadixBatchSingleJob Create a batch with single job parameters
	CreateRadixBatchSingleJob(ctx context.Context, jobScheduleDescription *common.JobScheduleDescription) (*modelsv2.RadixBatch, error)
	// CopyRadixBatchSingleJob Copy a batch with single job parameters
	CopyRadixBatchSingleJob(ctx context.Context, batchName, jobName, deploymentName string) (*modelsv2.RadixBatch, error)
	// MaintainHistoryLimit Delete outdated batches
	MaintainHistoryLimit(ctx context.Context) error
	// GarbageCollectPayloadSecrets Delete orphaned payload secrets
	GarbageCollectPayloadSecrets(ctx context.Context) error
	// DeleteRadixBatch Delete a batch
	DeleteRadixBatch(ctx context.Context, batchName string) error
	// StopRadixBatch Stop a batch
	StopRadixBatch(ctx context.Context, batchName string) error
	// StopRadixBatchJob Stop a batch job
	StopRadixBatchJob(ctx context.Context, batchName string, jobName string) error
}

// CompletedRadixBatches Completed RadixBatch lists
type CompletedRadixBatches struct {
	SucceededRadixBatches    []*modelsv2.RadixBatch
	NotSucceededRadixBatches []*modelsv2.RadixBatch
	SucceededSingleJobs      []*modelsv2.RadixBatch
	NotSucceededSingleJobs   []*modelsv2.RadixBatch
}

// New Constructor of the batch handler
func New(kube *kube.Kube, env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) Handler {
	return &handler{
		kubeUtil:                kube,
		env:                     env,
		radixDeployJobComponent: radixDeployJobComponent,
	}
}

type radixBatchJobWithDescription struct {
	radixBatchJob          *radixv1.RadixBatchJob
	jobScheduleDescription *common.JobScheduleDescription
}

// GetRadixBatches Get statuses of all batches
func (h *handler) GetRadixBatches(ctx context.Context) ([]modelsv2.RadixBatch, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Get batches for the namespace: %s", h.env.RadixDeploymentNamespace)
	return h.getRadixBatchStatus(ctx, kube.RadixBatchTypeBatch)
}

// GetRadixBatchSingleJobs Get statuses of all single jobs
func (h *handler) GetRadixBatchSingleJobs(ctx context.Context) ([]modelsv2.RadixBatch, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Get sigle jobs for the namespace: %s", h.env.RadixDeploymentNamespace)
	return h.getRadixBatchStatus(ctx, kube.RadixBatchTypeJob)
}

func (h *handler) getRadixBatchStatus(ctx context.Context, radixBatchType kube.RadixBatchType) ([]modelsv2.RadixBatch, error) {
	logger := log.Ctx(ctx)
	radixBatches, err := h.getRadixBatches(ctx,
		radixLabels.ForComponentName(h.env.RadixComponentName),
		radixLabels.ForBatchType(radixBatchType),
	)
	if err != nil {
		return nil, err
	}
	logger.Debug().Msgf("Found %v batches for namespace %s", len(radixBatches), h.env.RadixDeploymentNamespace)

	radixBatchStatuses := make([]modelsv2.RadixBatch, 0, len(radixBatches))
	for _, radixBatch := range radixBatches {
		radixBatchStatuses = append(radixBatchStatuses, h.getRadixBatchModelFromRadixBatch(radixBatch))
	}
	return radixBatchStatuses, nil
}

func (h *handler) getRadixBatchModelFromRadixBatch(radixBatch *radixv1.RadixBatch) modelsv2.RadixBatch {
	return modelsv2.RadixBatch{
		Name:           radixBatch.GetName(),
		BatchType:      radixBatch.Labels[kube.RadixBatchTypeLabel],
		CreationTime:   utils.FormatTime(pointers.Ptr(radixBatch.GetCreationTimestamp())),
		Started:        utils.FormatTime(radixBatch.Status.Condition.ActiveTime),
		Ended:          utils.FormatTime(radixBatch.Status.Condition.CompletionTime),
		Status:         jobs.GetRadixBatchStatus(radixBatch, h.radixDeployJobComponent),
		Message:        radixBatch.Status.Condition.Message,
		JobStatuses:    getRadixBatchJobStatusesFromRadixBatch(radixBatch, radixBatch.Status.JobStatuses),
		DeploymentName: radixBatch.Spec.RadixDeploymentJobRef.Name,
	}
}

func getRadixBatchJobStatusesFromRadixBatch(radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus) []modelsv2.RadixBatchJobStatus {
	radixBatchJobsStatuses := getRadixBatchJobsStatusesMap(radixBatchJobStatuses)
	jobStatuses := make([]modelsv2.RadixBatchJobStatus, 0, len(radixBatch.Spec.Jobs))
	for _, radixBatchJob := range radixBatch.Spec.Jobs {
		stopJob := radixBatchJob.Stop != nil && *radixBatchJob.Stop
		jobName := fmt.Sprintf("%s-%s", radixBatch.Name, radixBatchJob.Name) // composed name in models are always consist of a batchName and original jobName
		radixBatchJobStatus := modelsv2.RadixBatchJobStatus{
			Name:  jobName,
			JobId: radixBatchJob.JobId,
		}
		if jobStatus, ok := radixBatchJobsStatuses[radixBatchJob.Name]; ok {
			radixBatchJobStatus.CreationTime = utils.FormatTime(jobStatus.CreationTime)
			radixBatchJobStatus.Started = utils.FormatTime(jobStatus.StartTime)
			radixBatchJobStatus.Ended = utils.FormatTime(jobStatus.EndTime)
			radixBatchJobStatus.Status = jobs.GetScheduledJobStatus(jobStatus, stopJob)
			radixBatchJobStatus.Message = jobStatus.Message
			radixBatchJobStatus.Failed = jobStatus.Failed
			radixBatchJobStatus.Restart = jobStatus.Restart
			radixBatchJobStatus.PodStatuses = getPodStatusByRadixBatchJobPodStatus(jobStatus.RadixBatchJobPodStatuses)

		}
		jobStatuses = append(jobStatuses, radixBatchJobStatus)
	}
	return jobStatuses
}

func getRadixBatchJobsStatusesMap(radixBatchJobStatuses []radixv1.RadixBatchJobStatus) map[string]radixv1.RadixBatchJobStatus {
	radixBatchJobsStatuses := make(map[string]radixv1.RadixBatchJobStatus)
	for _, jobStatus := range radixBatchJobStatuses {
		jobStatus := jobStatus
		radixBatchJobsStatuses[jobStatus.Name] = jobStatus
	}
	return radixBatchJobsStatuses
}

// GetRadixBatch Get status of a batch
func (h *handler) GetRadixBatch(ctx context.Context, batchName string) (*modelsv2.RadixBatch, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("get batch status for the batch %s for namespace: %s", batchName, h.env.RadixDeploymentNamespace)
	radixBatch, err := h.getRadixBatch(ctx, batchName)
	if err != nil {
		return nil, err
	}
	return pointers.Ptr(h.getRadixBatchModelFromRadixBatch(radixBatch)), nil
}

// CreateRadixBatch Create a batch with parameters
func (h *handler) CreateRadixBatch(ctx context.Context, batchScheduleDescription *common.BatchScheduleDescription) (*modelsv2.RadixBatch, error) {
	logger := log.Ctx(ctx)
	if batchScheduleDescription == nil {
		return nil, apiErrors.NewInvalidWithReason("BatchScheduleDescription", "empty request body")
	}
	logger.Info().Msgf("Create Radix Batch for %d jobs", len(batchScheduleDescription.JobScheduleDescriptions))

	if len(batchScheduleDescription.JobScheduleDescriptions) == 0 {
		return nil, apiErrors.NewInvalidWithReason("BatchScheduleDescription", "empty job description list ")
	}

	radixBatch, err := h.createRadixBatchOrJob(ctx, *batchScheduleDescription, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	logger.Info().Msgf("Radix Batch %s has been created", radixBatch.Name)
	return radixBatch, nil
}

// CopyRadixBatch Copy a batch with deployment and optional parameters
func (h *handler) CopyRadixBatch(ctx context.Context, batchName, deploymentName string) (*modelsv2.RadixBatch, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Copy Radix Batch %s for the deployment %s", batchName, deploymentName)
	radixBatch, err := h.getRadixBatch(ctx, batchName)
	if err != nil {
		return nil, err
	}
	return h.copyRadixBatchOrJob(ctx, radixBatch, "", deploymentName)
}

func (h *handler) createRadixBatchOrJob(ctx context.Context, batchScheduleDescription common.BatchScheduleDescription, radixBatchType kube.RadixBatchType) (*modelsv2.RadixBatch, error) {
	logger := log.Ctx(ctx)
	namespace := h.env.RadixDeploymentNamespace
	radixComponentName := h.env.RadixComponentName
	radixDeploymentName := h.env.RadixDeploymentName
	logger.Info().Msgf("Create batch for namespace %s, component %s, deployment %s", namespace, radixComponentName, radixDeploymentName)

	radixDeployment, err := h.kubeUtil.RadixClient().RadixV1().RadixDeployments(namespace).
		Get(ctx, radixDeploymentName, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil, apiErrors.NewNotFound("radix deployment", radixDeploymentName)
		}
		return nil, apiErrors.NewFromError(err)
	}

	radixJobComponent := radixDeployment.GetJobComponentByName(radixComponentName)
	if radixJobComponent == nil {
		return nil, apiErrors.NewNotFound("job component", radixComponentName)
	}

	appName := radixDeployment.Spec.AppName

	createdRadixBatch, err := h.createRadixBatch(ctx, namespace, appName, radixDeployment.GetName(), *radixJobComponent, batchScheduleDescription, radixBatchType)
	if err != nil {
		return nil, apiErrors.NewFromError(err)
	}

	logger.Debug().Msgf("created batch %s for component %s, environment %s, in namespace: %s", createdRadixBatch.GetName(),
		radixComponentName, radixDeployment.Spec.Environment, namespace)
	return pointers.Ptr(h.getRadixBatchModelFromRadixBatch(createdRadixBatch)), nil
}

// CreateRadixBatchSingleJob Create a batch single job with parameters
func (h *handler) CreateRadixBatchSingleJob(ctx context.Context, jobScheduleDescription *common.JobScheduleDescription) (*modelsv2.RadixBatch, error) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Create Radix Batch single job")
	if jobScheduleDescription == nil {
		return nil, apiErrors.NewInvalidWithReason("JobScheduleDescription", "empty request body")
	}
	radixBatchJob, err := h.createRadixBatchOrJob(ctx, common.BatchScheduleDescription{
		JobScheduleDescriptions:        []common.JobScheduleDescription{*jobScheduleDescription},
		DefaultRadixJobComponentConfig: nil,
	}, kube.RadixBatchTypeJob)
	if err != nil {
		return nil, err
	}
	logger.Info().Msgf("Radix single job %s has been created", radixBatchJob.Name)
	return radixBatchJob, err
}

// CopyRadixBatchSingleJob Copy a batch with single job parameters
func (h *handler) CopyRadixBatchSingleJob(ctx context.Context, batchName, jobName, deploymentName string) (*modelsv2.RadixBatch, error) {
	radixBatch, err := h.getRadixBatch(ctx, batchName)
	if err != nil {
		return nil, err
	}
	return h.copyRadixBatchOrJob(ctx, radixBatch, jobName, deploymentName)
}

// DeleteRadixBatch Delete a batch
func (h *handler) DeleteRadixBatch(ctx context.Context, batchName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("delete batch %s for namespace: %s", batchName, h.env.RadixDeploymentNamespace)
	fg := metav1.DeletePropagationBackground
	err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(h.env.RadixDeploymentNamespace).Delete(ctx, batchName, metav1.DeleteOptions{PropagationPolicy: &fg})
	if err != nil {
		return apiErrors.NewFromError(err)
	}
	return err
}

// StopRadixBatch Stop a batch
func (h *handler) StopRadixBatch(ctx context.Context, batchName string) error {
	logger := log.Ctx(ctx)
	namespace := h.env.RadixDeploymentNamespace
	logger.Debug().Msgf("stop batch %s for namespace: %s", batchName, namespace)
	radixBatch, err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(namespace).Get(ctx, batchName, metav1.GetOptions{})
	if err != nil {
		return apiErrors.NewFromError(err)
	}
	if radixBatch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted {
		return apiErrors.NewBadRequest(fmt.Sprintf("cannot stop completed batch %s", batchName))
	}

	newRadixBatch := radixBatch.DeepCopy()
	radixBatchJobsStatusMap := getRadixBatchJobsStatusesMap(newRadixBatch.Status.JobStatuses)
	for jobIndex, radixBatchJob := range newRadixBatch.Spec.Jobs {
		if jobStatus, ok := radixBatchJobsStatusMap[radixBatchJob.Name]; ok &&
			(isRadixBatchJobSucceeded(jobStatus) || isRadixBatchJobFailed(jobStatus)) {
			continue
		}
		newRadixBatch.Spec.Jobs[jobIndex].Stop = pointers.Ptr(true)
	}
	return h.updateRadixBatch(ctx, newRadixBatch)
}

// StopRadixBatchJob Stop a batch job
func (h *handler) StopRadixBatchJob(ctx context.Context, batchName string, jobName string) error {
	logger := log.Ctx(ctx)
	namespace := h.env.RadixDeploymentNamespace
	logger.Debug().Msgf("stop a job %s in the batch %s for namespace: %s", jobName, batchName, namespace)
	radixBatch, err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(namespace).Get(ctx, batchName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if radixBatch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted {
		return apiErrors.NewBadRequest(fmt.Sprintf("cannot stop the job %s in the completed batch %s", jobName, batchName))
	}

	newRadixBatch := radixBatch.DeepCopy()
	radixBatchJobsStatusMap := getRadixBatchJobsStatusesMap(newRadixBatch.Status.JobStatuses)
	for jobIndex, radixBatchJob := range newRadixBatch.Spec.Jobs {
		if !strings.EqualFold(radixBatchJob.Name, jobName) {
			continue
		}
		if jobStatus, ok := radixBatchJobsStatusMap[radixBatchJob.Name]; ok &&
			(isRadixBatchJobSucceeded(jobStatus) || isRadixBatchJobFailed(jobStatus)) {
			return apiErrors.NewBadRequest(fmt.Sprintf("cannot stop the job %s with the status %s in the batch %s", jobName, string(jobStatus.Phase), batchName))
		}
		newRadixBatch.Spec.Jobs[jobIndex].Stop = pointers.Ptr(true)
		return h.updateRadixBatch(ctx, newRadixBatch)
	}
	return apiErrors.NewNotFound("batch job", jobName)
}

func (h *handler) updateRadixBatch(ctx context.Context, radixBatch *radixv1.RadixBatch) error {
	logger := log.Ctx(ctx)
	namespace := h.env.RadixDeploymentNamespace
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(namespace).Update(ctx, radixBatch, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return apiErrors.NewFromError(fmt.Errorf("failed to patch RadixBatch object: %w", err))
	}
	logger.Debug().Msgf("Patched RadixBatch: %s in namespace %s", radixBatch.GetName(), namespace)
	return nil
}

// MaintainHistoryLimit Delete outdated batches
func (h *handler) MaintainHistoryLimit(ctx context.Context) error {
	logger := log.Ctx(ctx)
	const minimumAge = 3600 // TODO add as default env-var and/or job-component property
	completedBefore := time.Now().Add(-time.Second * minimumAge)
	completedRadixBatches, err := h.getCompletedRadixBatchesSortedByCompletionTimeAsc(ctx, completedBefore)
	if err != nil {
		return err
	}

	historyLimit := h.env.RadixJobSchedulersPerEnvironmentHistoryLimit
	logger.Debug().Msg("maintain history limit for succeeded batches")
	var errs []error
	if err := h.maintainHistoryLimitForBatches(ctx, completedRadixBatches.SucceededRadixBatches, historyLimit); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("maintain history limit for not succeeded batches")
	if err := h.maintainHistoryLimitForBatches(ctx, completedRadixBatches.NotSucceededRadixBatches, historyLimit); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("maintain history limit for succeeded single jobs")
	if err := h.maintainHistoryLimitForBatches(ctx, completedRadixBatches.SucceededSingleJobs, historyLimit); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("maintain history limit for not succeeded single jobs")
	if err := h.maintainHistoryLimitForBatches(ctx, completedRadixBatches.NotSucceededSingleJobs, historyLimit); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("delete orphaned payload secrets")
	err = h.GarbageCollectPayloadSecrets(ctx)
	if err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (h *handler) getCompletedRadixBatchesSortedByCompletionTimeAsc(ctx context.Context, completedBefore time.Time) (*CompletedRadixBatches, error) {
	radixBatches, err := h.getRadixBatches(ctx, radixLabels.ForComponentName(h.env.RadixComponentName))
	if err != nil {
		return nil, err
	}
	radixBatches = sortRJSchByCompletionTimeAsc(radixBatches)
	return &CompletedRadixBatches{
		SucceededRadixBatches:    h.getSucceededRadixBatches(radixBatches, completedBefore),
		NotSucceededRadixBatches: h.getNotSucceededRadixBatches(radixBatches, completedBefore),
		SucceededSingleJobs:      h.getSucceededSingleJobs(radixBatches, completedBefore),
		NotSucceededSingleJobs:   h.getNotSucceededSingleJobs(radixBatches, completedBefore),
	}, nil
}

func (h *handler) getNotSucceededRadixBatches(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.RadixBatch {
	return h.getRadixBatchModelsFromRadixBatches(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeBatch) && isRadixBatchNotSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch)
	}))
}

func (h *handler) getSucceededRadixBatches(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.RadixBatch {
	radixBatches = slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeBatch) && isRadixBatchSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch)
	})
	return h.getRadixBatchModelsFromRadixBatches(radixBatches)
}

func radixBatchIsCompletedBefore(completedBefore time.Time, radixBatch *radixv1.RadixBatch) bool {
	return radixBatch.Status.Condition.CompletionTime != nil && (*radixBatch.Status.Condition.CompletionTime).Before(&metav1.Time{Time: completedBefore})
}

func (h *handler) getNotSucceededSingleJobs(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.RadixBatch {
	return h.getRadixBatchModelsFromRadixBatches(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeJob) && isRadixBatchNotSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch)
	}))
}

func (h *handler) getSucceededSingleJobs(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.RadixBatch {
	return h.getRadixBatchModelsFromRadixBatches(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeJob) && isRadixBatchSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch)
	}))
}

func (h *handler) getRadixBatchModelsFromRadixBatches(radixBatches []*radixv1.RadixBatch) []*modelsv2.RadixBatch {
	batches := make([]*modelsv2.RadixBatch, 0, len(radixBatches))
	for _, radixBatch := range radixBatches {
		batches = append(batches, pointers.Ptr(h.getRadixBatchModelFromRadixBatch(radixBatch)))
	}
	return batches
}

func radixBatchHasType(radixBatch *radixv1.RadixBatch, radixBatchType kube.RadixBatchType) bool {
	return radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(radixBatchType)
}

func isRadixBatchSucceeded(batch *radixv1.RadixBatch) bool {
	return batch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted && slice.All(batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return isRadixBatchJobSucceeded(jobStatus)
	})
}

func isRadixBatchNotSucceeded(batch *radixv1.RadixBatch) bool {
	return batch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted && slice.Any(batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return !isRadixBatchJobSucceeded(jobStatus)
	})
}

func isRadixBatchJobSucceeded(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseSucceeded || jobStatus.Phase == radixv1.BatchJobPhaseStopped
}

func isRadixBatchJobFailed(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseFailed
}

func (h *handler) maintainHistoryLimitForBatches(ctx context.Context, radixBatchesSortedByCompletionTimeAsc []*modelsv2.RadixBatch, historyLimit int) error {
	logger := log.Ctx(ctx)
	numToDelete := len(radixBatchesSortedByCompletionTimeAsc) - historyLimit
	if numToDelete <= 0 {
		logger.Debug().Msgf("no history batches to delete: %d batches, %d history limit", len(radixBatchesSortedByCompletionTimeAsc), historyLimit)
		return nil
	}
	logger.Debug().Msgf("history batches to delete: %v", numToDelete)

	for i := 0; i < numToDelete; i++ {
		batch := radixBatchesSortedByCompletionTimeAsc[i]
		logger.Debug().Msgf("deleting batch %s", batch.Name)
		if err := h.DeleteRadixBatch(ctx, batch.Name); err != nil {
			return err
		}
	}
	return nil
}

func sortRJSchByCompletionTimeAsc(batches []*radixv1.RadixBatch) []*radixv1.RadixBatch {
	sort.Slice(batches, func(i, j int) bool {
		batch1 := (batches)[i]
		batch2 := (batches)[j]
		return isRJS1CompletedBeforeRJS2(batch1, batch2)
	})
	return batches
}

func isRJS1CompletedBeforeRJS2(batch1 *radixv1.RadixBatch, batch2 *radixv1.RadixBatch) bool {
	rd1ActiveFrom := getCompletionTimeFrom(batch1)
	rd2ActiveFrom := getCompletionTimeFrom(batch2)

	return rd1ActiveFrom.Before(rd2ActiveFrom)
}

func getCompletionTimeFrom(radixBatch *radixv1.RadixBatch) *metav1.Time {
	if radixBatch.Status.Condition.CompletionTime.IsZero() {
		return pointers.Ptr(radixBatch.GetCreationTimestamp())
	}
	return radixBatch.Status.Condition.CompletionTime
}

func generateBatchName(jobComponentName string) string {
	timestamp := time.Now().Format("20060102150405")
	return fmt.Sprintf("batch-%s-%s-%s", getJobComponentNamePart(jobComponentName), timestamp, strings.ToLower(utils.RandString(8)))
}

func getJobComponentNamePart(jobComponentName string) string {
	componentNamePart := jobComponentName
	if len(componentNamePart) > 12 {
		componentNamePart = componentNamePart[:12]
	}
	return fmt.Sprintf("%s%s", componentNamePart, strings.ToLower(utils.RandString(16-len(componentNamePart))))
}

func (h *handler) createRadixBatch(ctx context.Context, namespace, appName, radixDeploymentName string, radixJobComponent radixv1.RadixDeployJobComponent, batchScheduleDescription common.BatchScheduleDescription, radixBatchType kube.RadixBatchType) (*radixv1.RadixBatch, error) {
	logger := log.Ctx(ctx)
	batchName := generateBatchName(radixJobComponent.GetName())
	logger.Debug().Msgf("Create Radix Batch %s", batchName)
	radixJobComponentName := radixJobComponent.GetName()
	radixBatchJobs, err := h.buildRadixBatchJobs(ctx, namespace, appName, radixJobComponentName, batchName, batchScheduleDescription, radixJobComponent.Payload)
	if err != nil {
		return nil, err
	}
	logger.Debug().Msgf("Built Radix Batch with %d jobs", len(radixBatchJobs))
	radixBatch := radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name: batchName,
			Labels: radixLabels.Merge(
				radixLabels.ForApplicationName(appName),
				radixLabels.ForComponentName(radixJobComponentName),
				radixLabels.ForBatchType(radixBatchType),
			),
		},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: radixDeploymentName},
				Job:                  radixJobComponentName,
			},
		},
	}
	radixBatch.Spec.Jobs = radixBatchJobs
	logger.Debug().Msgf("Create Radix Batch in the cluster")
	createdRadixBatch, err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(namespace).Create(ctx, &radixBatch,
		metav1.CreateOptions{})
	if err != nil {
		return nil, apiErrors.NewFromError(err)
	}
	return createdRadixBatch, nil
}

func (h *handler) copyRadixBatchOrJob(ctx context.Context, sourceRadixBatch *radixv1.RadixBatch, sourceJobName string, radixDeploymentName string) (*modelsv2.RadixBatch, error) {
	logger := log.Ctx(ctx)
	namespace := h.env.RadixDeploymentNamespace
	logger.Debug().Msgf("copy batch %s for namespace: %s", sourceRadixBatch.GetName(), namespace)
	radixComponentName := h.env.RadixComponentName
	radixBatch := radixv1.RadixBatch{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateBatchName(radixComponentName),
			Labels: sourceRadixBatch.GetLabels(),
		},
		Spec: radixv1.RadixBatchSpec{
			RadixDeploymentJobRef: radixv1.RadixDeploymentJobComponentSelector{
				LocalObjectReference: radixv1.LocalObjectReference{Name: radixDeploymentName},
				Job:                  radixComponentName,
			},
			Jobs: h.copyBatchJobs(ctx, sourceRadixBatch, sourceJobName),
		},
	}

	logger.Debug().Msgf("Create the copied Radix Batch %s with %d jobs in the cluster", radixBatch.GetName(), len(radixBatch.Spec.Jobs))
	createdRadixBatch, err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(namespace).Create(ctx, &radixBatch, metav1.CreateOptions{})
	if err != nil {
		return nil, apiErrors.NewFromError(err)
	}

	logger.Debug().Msgf("copied batch %s from the batch %s for component %s, ein namespace: %s", radixBatch.GetName(), sourceRadixBatch.GetName(), radixComponentName, namespace)
	return pointers.Ptr(h.getRadixBatchModelFromRadixBatch(createdRadixBatch)), nil
}

func (h *handler) copyBatchJobs(ctx context.Context, sourceRadixBatch *radixv1.RadixBatch, sourceJobName string) []radixv1.RadixBatchJob {
	logger := log.Ctx(ctx)
	radixBatchJobs := make([]radixv1.RadixBatchJob, 0, len(sourceRadixBatch.Spec.Jobs))
	for _, sourceJob := range sourceRadixBatch.Spec.Jobs {
		if sourceJobName != "" && sourceJob.Name != sourceJobName {
			continue
		}
		job := sourceJob.DeepCopy()
		job.Name = createJobName()
		logger.Debug().Msgf("Copy Radxi Batch Job %s", job.Name)
		radixBatchJobs = append(radixBatchJobs, *job)
	}
	return radixBatchJobs
}

func (h *handler) buildRadixBatchJobs(ctx context.Context, namespace, appName, radixJobComponentName, batchName string, batchScheduleDescription common.BatchScheduleDescription, radixJobComponentPayload *radixv1.RadixJobComponentPayload) ([]radixv1.RadixBatchJob, error) {
	logger := log.Ctx(ctx)
	var radixBatchJobWithDescriptions []radixBatchJobWithDescription
	var errs []error
	logger.Debug().Msg("Build Radix Batch")
	for _, jobScheduleDescription := range batchScheduleDescription.JobScheduleDescriptions {
		jobScheduleDescription := jobScheduleDescription
		logger.Debug().Msgf("Build Radix Batch Job. JobId: '%s', Payload length: %d", jobScheduleDescription.JobId, len(jobScheduleDescription.Payload))
		radixBatchJob, err := buildRadixBatchJob(&jobScheduleDescription, batchScheduleDescription.DefaultRadixJobComponentConfig)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		logger.Debug().Msgf("Built  Radix Batch Job %s", radixBatchJob.Name)
		radixBatchJobWithDescriptions = append(radixBatchJobWithDescriptions, radixBatchJobWithDescription{
			radixBatchJob:          radixBatchJob,
			jobScheduleDescription: &jobScheduleDescription,
		})
	}
	if len(errs) > 0 {
		return nil, apiErrors.NewFromError(errors.Join(errs...))
	}
	radixJobComponentHasPayloadPath := radixJobComponentPayload != nil && len(radixJobComponentPayload.Path) > 0
	err := h.createRadixBatchJobPayloadSecrets(ctx, namespace, appName, radixJobComponentName, batchName, radixBatchJobWithDescriptions, radixJobComponentHasPayloadPath)
	if err != nil {
		return nil, err
	}
	radixBatchJobs := make([]radixv1.RadixBatchJob, 0, len(radixBatchJobWithDescriptions))
	for _, item := range radixBatchJobWithDescriptions {
		radixBatchJobs = append(radixBatchJobs, *item.radixBatchJob)
	}
	return radixBatchJobs, nil
}

func createJobName() string {
	return strings.ToLower(utils.RandStringSeed(8, defaultSrc))
}

func (h *handler) createRadixBatchJobPayloadSecrets(ctx context.Context, namespace, appName, radixJobComponentName, batchName string, radixJobWithDescriptions []radixBatchJobWithDescription, radixJobComponentHasPayloadPath bool) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Create Payload secrets for the batch %s", batchName)
	accumulatedSecretSize := 0
	var payloadSecrets []*corev1.Secret
	payloadsSecret := buildPayloadSecret(ctx, appName, radixJobComponentName, batchName, 0)
	payloadSecrets = append(payloadSecrets, payloadsSecret)
	var errs []error
	for jobIndex, radixJobWithDescriptions := range radixJobWithDescriptions {
		payload := []byte(strings.TrimSpace(radixJobWithDescriptions.jobScheduleDescription.Payload))
		if len(payload) == 0 {
			logger.Info().Msgf("No payload in the job #%d", jobIndex)
			continue
		}
		if !radixJobComponentHasPayloadPath {
			errs = append(errs, fmt.Errorf("missing an expected payload path, but there is a payload in the job #%d", jobIndex))
			continue
		}

		logger.Info().Msgf("Payload for the job #%d, JobId: '%s', length: %d", jobIndex, radixJobWithDescriptions.jobScheduleDescription.JobId, len(payload))
		radixBatchJob := radixJobWithDescriptions.radixBatchJob
		payloadBase64 := base64.RawStdEncoding.EncodeToString(payload)
		secretEntrySize := len(payloadBase64) + len(radixBatchJob.Name) + payloadSecretEntryAuxDataSize // preliminary estimate of a payload secret entry
		logger.Debug().Msgf("Prelimenary esptimated payload size: %d", secretEntrySize)
		newAccumulatedPayloadSecretSize := payloadSecretAuxDataSize + accumulatedSecretSize + secretEntrySize
		logger.Debug().Msgf("New evaluated accumulated payload size with aux secret data: %d", newAccumulatedPayloadSecretSize)
		if newAccumulatedPayloadSecretSize > maxPayloadSecretSize {
			if len(payloadsSecret.Data) == 0 {
				// this is the first entry in the secret, and it is too large to be stored to the secret - no reason to create new secret.
				return fmt.Errorf("payload is too large in the job #%d - its base64 size is %d bytes, but it is expected to be less then %d bytes", jobIndex, secretEntrySize, maxPayloadSecretSize)
			}
			logger.Debug().Msgf("New evaluated accumulated payload size is great then the max size %d - build a new payload secret", maxPayloadSecretSize)
			payloadsSecret = buildPayloadSecret(ctx, appName, radixJobComponentName, batchName, len(payloadSecrets))
			payloadSecrets = append(payloadSecrets, payloadsSecret)
			accumulatedSecretSize = 0
		}

		payloadsSecret.Data[radixBatchJob.Name] = payload
		accumulatedSecretSize = accumulatedSecretSize + secretEntrySize
		logger.Debug().Msgf("New accumulated payload size: %d", newAccumulatedPayloadSecretSize)
		logger.Debug().Msgf("Added a reference to the payload secret %s, key %s", payloadsSecret.GetName(), radixBatchJob.Name)
		radixBatchJob.PayloadSecretRef = &radixv1.PayloadSecretKeySelector{
			LocalObjectReference: radixv1.LocalObjectReference{Name: payloadsSecret.GetName()},
			Key:                  radixBatchJob.Name,
		}
	}
	if len(errs) > 0 {
		return apiErrors.NewFromError(errors.Join(errs...))
	}
	logger.Debug().Msg("Create payload secrets")
	return h.createSecrets(ctx, namespace, payloadSecrets)
}

func (h *handler) createSecrets(ctx context.Context, namespace string, secrets []*corev1.Secret) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Create %d secrets", len(secrets))
	for _, secret := range secrets {
		if secret.Data == nil || len(secret.Data) == 0 {
			logger.Debug().Msgf("Do not create a secret %s - Data is empty, the secret is not used in any jobs", secret.GetName())
			continue
		}
		logger.Debug().Msgf("Create a secret %s in the cluster", secret.GetName())
		_, err := h.kubeUtil.KubeClient().CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return apiErrors.NewFromError(err)
		}
	}
	return nil
}

func buildPayloadSecret(ctx context.Context, appName, radixJobComponentName, batchName string, secretIndex int) *corev1.Secret {
	logger := log.Ctx(ctx)
	secretName := fmt.Sprintf("%s-payloads-%d", batchName, secretIndex)
	logger.Debug().Msgf("build payload secret %s", secretName)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: radixLabels.Merge(
				radixLabels.ForApplicationName(appName),
				radixLabels.ForComponentName(radixJobComponentName),
				radixLabels.ForBatchName(batchName),
				radixLabels.ForJobScheduleJobType(),
				radixLabels.ForRadixSecretType(kube.RadixSecretJobPayload),
			),
		},
		Data: make(map[string][]byte),
	}
}

func buildRadixBatchJob(jobScheduleDescription *common.JobScheduleDescription, defaultJobScheduleDescription *common.RadixJobComponentConfig) (*radixv1.RadixBatchJob, error) {
	err := applyDefaultJobDescriptionProperties(jobScheduleDescription, defaultJobScheduleDescription)
	if err != nil {
		return nil, apiErrors.NewFromError(err)
	}
	return &radixv1.RadixBatchJob{
		Name:             createJobName(),
		JobId:            jobScheduleDescription.JobId,
		Resources:        jobScheduleDescription.Resources,
		Node:             jobScheduleDescription.Node,
		TimeLimitSeconds: jobScheduleDescription.TimeLimitSeconds,
		BackoffLimit:     jobScheduleDescription.BackoffLimit,
		ImageTagName:     jobScheduleDescription.ImageTagName,
	}, nil
}

func (h *handler) getRadixBatches(ctx context.Context, labels ...map[string]string) ([]*radixv1.RadixBatch, error) {
	radixBatchList, err := h.kubeUtil.RadixClient().
		RadixV1().
		RadixBatches(h.env.RadixDeploymentNamespace).
		List(
			ctx,
			metav1.ListOptions{
				LabelSelector: radixLabels.Merge(labels...).String(),
			},
		)

	if err != nil {
		return nil, apiErrors.NewFromError(err)
	}

	return slice.PointersOf(radixBatchList.Items).([]*radixv1.RadixBatch), nil
}

func (h *handler) getRadixBatch(ctx context.Context, batchName string) (*radixv1.RadixBatch, error) {
	radixBatch, err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(h.env.RadixDeploymentNamespace).Get(ctx, batchName, metav1.GetOptions{})
	if err != nil {
		return nil, apiErrors.NewFromError(err)
	}
	return radixBatch, nil
}

// GarbageCollectPayloadSecrets Delete orphaned payload secrets
func (h *handler) GarbageCollectPayloadSecrets(ctx context.Context) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Garbage collecting payload secrets")
	payloadSecretRefNames, _ := h.getJobComponentPayloadSecretRefNames(ctx)
	payloadSecrets, err := h.kubeUtil.ListSecretsWithSelector(ctx, h.env.RadixDeploymentNamespace, radixLabels.GetRadixBatchDescendantsSelector(h.env.RadixComponentName).String())
	if err != nil {
		return apiErrors.NewFromError(err)
	}
	logger.Debug().Msgf("%d payload secrets, %d secret reference unique names", len(payloadSecrets), len(payloadSecretRefNames))
	yesterday := time.Now().Add(time.Hour * -24)
	for _, payloadSecret := range payloadSecrets {
		if _, ok := payloadSecretRefNames[payloadSecret.GetName()]; !ok {
			if payloadSecret.GetCreationTimestamp().After(yesterday) {
				logger.Debug().Msgf("skipping deletion of an orphaned payload secret %s, created within 24 hours", payloadSecret.GetName())
				continue
			}
			err := h.DeleteSecret(ctx, payloadSecret)
			if err != nil {
				logger.Error().Err(err).Msgf("failed deleting of an orphaned payload secret %s", payloadSecret.GetName())
			}
			logger.Debug().Msgf("deleted an orphaned payload secret %s", payloadSecret.GetName())
		}
	}
	return nil
}

func (h *handler) getJobComponentPayloadSecretRefNames(ctx context.Context) (map[string]bool, error) {
	radixBatches, err := h.getRadixBatches(ctx, radixLabels.ForComponentName(h.env.RadixComponentName))
	if err != nil {
		return nil, err
	}
	payloadSecretRefNames := make(map[string]bool)
	for _, radixBatch := range radixBatches {
		for _, job := range radixBatch.Spec.Jobs {
			if job.PayloadSecretRef != nil {
				payloadSecretRefNames[job.PayloadSecretRef.Name] = true
			}
		}
	}
	return payloadSecretRefNames, nil
}

func applyDefaultJobDescriptionProperties(jobScheduleDescription *common.JobScheduleDescription, defaultRadixJobComponentConfig *common.RadixJobComponentConfig) error {
	if jobScheduleDescription == nil || defaultRadixJobComponentConfig == nil {
		return nil
	}
	return mergo.Merge(&jobScheduleDescription.RadixJobComponentConfig, defaultRadixJobComponentConfig, mergo.WithTransformers(authTransformer))
}

func getPodStatusByRadixBatchJobPodStatus(podStatuses []radixv1.RadixBatchJobPodStatus) []modelsv2.RadixBatchJobPodStatus {
	return slice.Map(podStatuses, func(status radixv1.RadixBatchJobPodStatus) modelsv2.RadixBatchJobPodStatus {
		return modelsv2.RadixBatchJobPodStatus{
			Name:             status.Name,
			Created:          utils.FormatTime(status.CreationTime),
			StartTime:        utils.FormatTime(status.StartTime),
			EndTime:          utils.FormatTime(status.EndTime),
			ContainerStarted: utils.FormatTime(status.StartTime),
			Status:           modelsv2.ReplicaStatus{Status: jobs.GetReplicaStatusByJobPodStatusPhase(status.Phase)},
			StatusMessage:    status.Message,
			RestartCount:     status.RestartCount,
			Image:            status.Image,
			ImageId:          status.ImageID,
			PodIndex:         status.PodIndex,
			ExitCode:         status.ExitCode,
			Reason:           status.Reason,
		}
	})
}
