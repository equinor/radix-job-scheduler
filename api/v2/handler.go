package apiv2

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils"
	commonErrors "github.com/equinor/radix-common/utils/errors"
	mergoutils "github.com/equinor/radix-common/utils/mergo"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/imdario/mergo"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
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
	kubeUtil *kube.Kube
	env      *models.Env
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
	// GetCompletedRadixBatchesSortedByCompletionTimeAsc Gets completed RadixBatch lists for env.RadixComponentName
	GetCompletedRadixBatchesSortedByCompletionTimeAsc(ctx context.Context) (*CompletedRadixBatches, error)
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
func New(kube *kube.Kube, env *models.Env) Handler {
	return &handler{
		kubeUtil: kube,
		env:      env,
	}
}

type radixBatchJobWithDescription struct {
	radixBatchJob          *radixv1.RadixBatchJob
	jobScheduleDescription *common.JobScheduleDescription
}

// GetRadixBatches Get statuses of all batches
func (h *handler) GetRadixBatches(ctx context.Context) ([]modelsv2.RadixBatch, error) {
	log.Debugf("Get batches for the namespace: %s", h.env.RadixDeploymentNamespace)
	return h.getRadixBatchStatus(ctx, kube.RadixBatchTypeBatch)
}

// GetRadixBatchSingleJobs Get statuses of all single jobs
func (h *handler) GetRadixBatchSingleJobs(ctx context.Context) ([]modelsv2.RadixBatch, error) {
	log.Debugf("Get sigle jobs for the namespace: %s", h.env.RadixDeploymentNamespace)
	return h.getRadixBatchStatus(ctx, kube.RadixBatchTypeJob)
}

func (h *handler) getRadixBatchStatus(ctx context.Context, radixBatchType kube.RadixBatchType) ([]modelsv2.RadixBatch, error) {
	radixBatches, err := h.getRadixBatches(ctx,
		radixLabels.ForComponentName(h.env.RadixComponentName),
		radixLabels.ForBatchType(radixBatchType),
	)
	if err != nil {
		return nil, err
	}
	log.Debugf("Found %v batches for namespace %s", len(radixBatches), h.env.RadixDeploymentNamespace)

	var radixBatchStatuses []modelsv2.RadixBatch
	for _, radixBatch := range radixBatches {
		radixBatchStatuses = append(radixBatchStatuses, getRadixBatchModelFromRadixBatch(radixBatch))
	}
	return radixBatchStatuses, nil
}

func getRadixBatchModelFromRadixBatch(radixBatch *radixv1.RadixBatch) modelsv2.RadixBatch {
	return modelsv2.RadixBatch{
		Name:         radixBatch.GetName(),
		BatchType:    radixBatch.Labels[kube.RadixBatchTypeLabel],
		CreationTime: utils.FormatTime(pointers.Ptr(radixBatch.GetCreationTimestamp())),
		Started:      utils.FormatTime(radixBatch.Status.Condition.ActiveTime),
		Ended:        utils.FormatTime(radixBatch.Status.Condition.CompletionTime),
		Status:       GetRadixBatchStatus(radixBatch).String(),
		Message:      radixBatch.Status.Condition.Message,
		JobStatuses:  getRadixBatchJobStatusesFromRadixBatch(radixBatch, radixBatch.Status.JobStatuses),
	}
}

func getRadixBatchJobStatusesFromRadixBatch(radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus) []modelsv2.RadixBatchJobStatus {
	radixBatchJobsStatuses := getRadixBatchJobsStatusesMap(radixBatchJobStatuses)
	var jobStatuses []modelsv2.RadixBatchJobStatus
	for _, radixBatchJob := range radixBatch.Spec.Jobs {
		jobName := fmt.Sprintf("%s-%s", radixBatch.Name, radixBatchJob.Name) // composed name in models are always consist of a batchName and original jobName
		radixBatchJobStatus := modelsv2.RadixBatchJobStatus{
			Name:  jobName,
			JobId: radixBatchJob.JobId,
		}
		if jobStatus, ok := radixBatchJobsStatuses[radixBatchJob.Name]; ok {
			radixBatchJobStatus.CreationTime = utils.FormatTime(jobStatus.CreationTime)
			radixBatchJobStatus.Started = utils.FormatTime(jobStatus.StartTime)
			radixBatchJobStatus.Ended = utils.FormatTime(jobStatus.EndTime)
			radixBatchJobStatus.Status = GetRadixBatchJobStatusFromPhase(radixBatchJob, jobStatus.Phase).String()
			radixBatchJobStatus.Message = jobStatus.Message
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
	log.Debugf("get batch status for the batch %s for namespace: %s", batchName, h.env.RadixDeploymentNamespace)
	radixBatch, err := h.getRadixBatch(ctx, batchName)
	if err != nil {
		return nil, err
	}
	return pointers.Ptr(getRadixBatchModelFromRadixBatch(radixBatch)), nil
}

// CreateRadixBatch Create a batch with parameters
func (h *handler) CreateRadixBatch(ctx context.Context, batchScheduleDescription *common.BatchScheduleDescription) (*modelsv2.RadixBatch, error) {
	if batchScheduleDescription == nil {
		return nil, apiErrors.NewInvalidWithReason("BatchScheduleDescription", "empty request body")
	}

	if len(batchScheduleDescription.JobScheduleDescriptions) == 0 {
		return nil, apiErrors.NewInvalidWithReason("BatchScheduleDescription", "empty job description list ")
	}

	return h.createRadixBatchOrJob(ctx, *batchScheduleDescription, kube.RadixBatchTypeBatch)
}

// CopyRadixBatch Copy a batch with deployment and optional parameters
func (h *handler) CopyRadixBatch(ctx context.Context, batchName, deploymentName string) (*modelsv2.RadixBatch, error) {
	radixBatch, err := h.getRadixBatch(ctx, batchName)
	if err != nil {
		return nil, err
	}
	return h.copyRadixBatchOrJob(ctx, radixBatch, "", deploymentName)
}

func (h *handler) createRadixBatchOrJob(ctx context.Context, batchScheduleDescription common.BatchScheduleDescription, radixBatchType kube.RadixBatchType) (*modelsv2.RadixBatch, error) {
	namespace := h.env.RadixDeploymentNamespace
	radixComponentName := h.env.RadixComponentName
	radixDeploymentName := h.env.RadixDeploymentName
	log.Debugf("create batch for namespace: %s", namespace)

	radixDeployment, err := h.kubeUtil.RadixClient().RadixV1().RadixDeployments(namespace).
		Get(ctx, radixDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, apiErrors.NewNotFound("radix deployment", radixDeploymentName)
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

	log.Debug(fmt.Sprintf("created batch %s for component %s, environment %s, in namespace: %s", createdRadixBatch.GetName(),
		radixComponentName, radixDeployment.Spec.Environment, namespace))
	return pointers.Ptr(getRadixBatchModelFromRadixBatch(createdRadixBatch)), nil
}

// CreateRadixBatchSingleJob Create a batch single job with parameters
func (h *handler) CreateRadixBatchSingleJob(ctx context.Context, jobScheduleDescription *common.JobScheduleDescription) (*modelsv2.RadixBatch, error) {
	if jobScheduleDescription == nil {
		return nil, apiErrors.NewInvalidWithReason("JobScheduleDescription", "empty request body")
	}
	return h.createRadixBatchOrJob(ctx, common.BatchScheduleDescription{
		JobScheduleDescriptions:        []common.JobScheduleDescription{*jobScheduleDescription},
		DefaultRadixJobComponentConfig: nil,
	}, kube.RadixBatchTypeJob)
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
	log.Debugf("delete batch %s for namespace: %s", batchName, h.env.RadixDeploymentNamespace)
	fg := metav1.DeletePropagationBackground
	err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(h.env.RadixDeploymentNamespace).Delete(ctx, batchName, metav1.DeleteOptions{PropagationPolicy: &fg})
	if err != nil {
		return err
	}
	secrets, err := h.GetSecretsForRadixBatch(batchName)
	if err != nil {
		return err
	}
	var errs []error
	for _, secret := range secrets {
		err := h.DeleteSecret(secret)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return commonErrors.Concat(errs)
}

// StopRadixBatch Stop a batch
func (h *handler) StopRadixBatch(ctx context.Context, batchName string) error {
	namespace := h.env.RadixDeploymentNamespace
	log.Debugf("stop batch %s for namespace: %s", batchName, namespace)
	radixBatch, err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(namespace).Get(ctx, batchName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if radixBatch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted {
		return fmt.Errorf("cannot stop completed batch %s", batchName)
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
	namespace := h.env.RadixDeploymentNamespace
	log.Debugf("stop a job %s in the batch %s for namespace: %s", jobName, batchName, namespace)
	radixBatch, err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(namespace).Get(ctx, batchName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if radixBatch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted {
		return fmt.Errorf("cannot stop the job %s in the completed batch %s", jobName, batchName)
	}

	newRadixBatch := radixBatch.DeepCopy()
	radixBatchJobsStatusMap := getRadixBatchJobsStatusesMap(newRadixBatch.Status.JobStatuses)
	for jobIndex, radixBatchJob := range newRadixBatch.Spec.Jobs {
		if !strings.EqualFold(radixBatchJob.Name, jobName) {
			continue
		}
		if jobStatus, ok := radixBatchJobsStatusMap[radixBatchJob.Name]; ok &&
			(isRadixBatchJobSucceeded(jobStatus) || isRadixBatchJobFailed(jobStatus)) {
			return fmt.Errorf("cannot stop the job %s with the status %s in the batch %s", jobName, string(jobStatus.Phase), batchName)
		}
		newRadixBatch.Spec.Jobs[jobIndex].Stop = pointers.Ptr(true)
		return h.updateRadixBatch(ctx, newRadixBatch)
	}
	return fmt.Errorf("not found a job %s in the batch %s", jobName, batchName)
}

func (h *handler) updateRadixBatch(ctx context.Context, radixBatch *radixv1.RadixBatch) error {
	namespace := h.env.RadixDeploymentNamespace
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(namespace).Update(ctx, radixBatch, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to patch RadixBatch object: %v", err)
	}
	log.Debugf("Patched RadixBatch: %s in namespace %s", radixBatch.GetName(), namespace)
	return nil
}

// MaintainHistoryLimit Delete outdated batches
func (h *handler) MaintainHistoryLimit(ctx context.Context) error {
	completedRadixBatches, _ := h.GetCompletedRadixBatchesSortedByCompletionTimeAsc(ctx)

	historyLimit := h.env.RadixJobSchedulersPerEnvironmentHistoryLimit
	log.Debug("maintain history limit for succeeded batches")
	var errs []error
	if err := h.maintainHistoryLimitForBatches(ctx, completedRadixBatches.SucceededRadixBatches, historyLimit); err != nil {
		errs = append(errs, err)
	}
	log.Debug("maintain history limit for not succeeded batches")
	if err := h.maintainHistoryLimitForBatches(ctx, completedRadixBatches.NotSucceededRadixBatches, historyLimit); err != nil {
		errs = append(errs, err)
	}
	log.Debug("maintain history limit for succeeded single jobs")
	if err := h.maintainHistoryLimitForBatches(ctx, completedRadixBatches.SucceededSingleJobs, historyLimit); err != nil {
		errs = append(errs, err)
	}
	log.Debug("maintain history limit for not succeeded single jobs")
	if err := h.maintainHistoryLimitForBatches(ctx, completedRadixBatches.NotSucceededSingleJobs, historyLimit); err != nil {
		errs = append(errs, err)
	}
	log.Debug("delete orphaned payload secrets")
	err := h.GarbageCollectPayloadSecrets(ctx)
	if err != nil {
		errs = append(errs, err)
	}
	return commonErrors.Concat(errs)
}

// GetCompletedRadixBatchesSortedByCompletionTimeAsc Gets completed RadixBatch lists for env.RadixComponentName
func (h *handler) GetCompletedRadixBatchesSortedByCompletionTimeAsc(ctx context.Context) (*CompletedRadixBatches, error) {
	radixBatches, err := h.getRadixBatches(ctx, radixLabels.ForComponentName(h.env.RadixComponentName))
	if err != nil {
		return nil, err
	}
	radixBatches = sortRJSchByCompletionTimeAsc(radixBatches)
	return &CompletedRadixBatches{
		SucceededRadixBatches:    getSucceededRadixBatches(radixBatches),
		NotSucceededRadixBatches: getNotSucceededRadixBatches(radixBatches),
		SucceededSingleJobs:      getSucceededSingleJobs(radixBatches),
		NotSucceededSingleJobs:   getNotSucceededSingleJobs(radixBatches),
	}, nil
}

func getNotSucceededRadixBatches(radixBatches []*radixv1.RadixBatch) []*modelsv2.RadixBatch {
	return getRadixBatchModelsFromRadixBatches(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeBatch) && isRadixBatchNotSucceeded(radixBatch)
	}))
}

func getSucceededRadixBatches(radixBatches []*radixv1.RadixBatch) []*modelsv2.RadixBatch {
	radixBatches = slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeBatch) && isRadixBatchSucceeded(radixBatch)
	})
	return getRadixBatchModelsFromRadixBatches(radixBatches)
}

func getNotSucceededSingleJobs(radixBatches []*radixv1.RadixBatch) []*modelsv2.RadixBatch {
	return getRadixBatchModelsFromRadixBatches(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeJob) && isRadixBatchNotSucceeded(radixBatch)
	}))
}

func getSucceededSingleJobs(radixBatches []*radixv1.RadixBatch) []*modelsv2.RadixBatch {
	return getRadixBatchModelsFromRadixBatches(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeJob) && isRadixBatchSucceeded(radixBatch)
	}))
}

func getRadixBatchModelsFromRadixBatches(radixBatches []*radixv1.RadixBatch) []*modelsv2.RadixBatch {
	var batches []*modelsv2.RadixBatch
	for _, radixBatch := range radixBatches {
		batches = append(batches, pointers.Ptr(getRadixBatchModelFromRadixBatch(radixBatch)))
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
	return batch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted && !slice.Any(batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return isRadixBatchJobSucceeded(jobStatus)
	})
}

func isRadixBatchJobSucceeded(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseSucceeded || jobStatus.Phase == radixv1.BatchJobPhaseStopped
}

func isRadixBatchJobFailed(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseFailed
}

func (h *handler) maintainHistoryLimitForBatches(ctx context.Context, radixBatchesSortedByCompletionTimeAsc []*modelsv2.RadixBatch, historyLimit int) error {
	numToDelete := len(radixBatchesSortedByCompletionTimeAsc) - historyLimit
	if numToDelete <= 0 {
		log.Debug("no history batches to delete")
		return nil
	}
	log.Debugf("history batches to delete: %v", numToDelete)

	for i := 0; i < numToDelete; i++ {
		batch := radixBatchesSortedByCompletionTimeAsc[i]
		log.Debugf("deleting batch %s", batch.Name)
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
	batchName := generateBatchName(radixJobComponent.GetName())
	radixJobComponentName := radixJobComponent.GetName()
	radixBatchJobs, err := h.buildRadixBatchJobs(ctx, namespace, appName, radixJobComponentName, batchName, batchScheduleDescription, radixJobComponent.Payload)
	if err != nil {
		return nil, err
	}
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
	return h.kubeUtil.RadixClient().RadixV1().RadixBatches(namespace).Create(ctx, &radixBatch,
		metav1.CreateOptions{})
}

func (h *handler) copyRadixBatchOrJob(ctx context.Context, sourceRadixBatch *radixv1.RadixBatch, sourceJobName string, radixDeploymentName string) (*modelsv2.RadixBatch, error) {
	namespace := h.env.RadixDeploymentNamespace
	log.Debugf("copy batch %s for namespace: %s", sourceRadixBatch.GetName(), namespace)
	radixDeployment, err := h.kubeUtil.RadixClient().RadixV1().RadixDeployments(namespace).
		Get(ctx, radixDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, apiErrors.NewNotFound("radix deployment", radixDeploymentName)
	}
	appName := radixDeployment.Spec.AppName
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
		},
	}

	sourceJobNameToNewNameMap := h.copyBatchJobs(sourceRadixBatch, &radixBatch, sourceJobName)
	sourceSecretNameToNewNameMap, err := h.copyPayloadSecrets(ctx, sourceRadixBatch, namespace, appName, radixComponentName, radixBatch.GetName(), sourceJobNameToNewNameMap, sourceJobName)
	if err != nil {
		return nil, err
	}

	err = h.setJobPayloadSecretReferences(&radixBatch, sourceSecretNameToNewNameMap)
	if err != nil {
		return nil, err
	}

	createdRadixBatch, err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(namespace).Create(ctx, &radixBatch, metav1.CreateOptions{})
	if err != nil {
		return nil, apiErrors.NewFromError(err)
	}

	log.Debug(fmt.Sprintf("copied batch %s from the batch %s for component %s, environment %s, in namespace: %s", radixBatch.GetName(), sourceRadixBatch.GetName(), radixComponentName, radixDeployment.Spec.Environment, namespace))
	return pointers.Ptr(getRadixBatchModelFromRadixBatch(createdRadixBatch)), nil
}

func (h *handler) setJobPayloadSecretReferences(radixBatch *radixv1.RadixBatch, sourceSecretNameToNewNameMap map[string]string) error {
	for i := 0; i < len(radixBatch.Spec.Jobs); i++ {
		if radixBatch.Spec.Jobs[i].PayloadSecretRef == nil {
			continue
		}
		sourceSecretName := radixBatch.Spec.Jobs[i].PayloadSecretRef.Name
		if newSecretName, ok := sourceSecretNameToNewNameMap[sourceSecretName]; ok {
			radixBatch.Spec.Jobs[i].PayloadSecretRef.Name = newSecretName
			radixBatch.Spec.Jobs[i].PayloadSecretRef.Key = radixBatch.Spec.Jobs[i].Name
			continue
		}
		return fmt.Errorf("could not find source secret name %s", sourceSecretName)
	}
	return nil
}

func (h *handler) copyBatchJobs(sourceRadixBatch *radixv1.RadixBatch, radixBatch *radixv1.RadixBatch, sourceJobName string) map[string]string {
	sourceJobNameToNewNameMap := make(map[string]string, len(sourceRadixBatch.Spec.Jobs))
	for _, sourceJob := range sourceRadixBatch.Spec.Jobs {
		if sourceJobName != "" && sourceJob.Name != sourceJobName {
			continue
		}
		job := sourceJob.DeepCopy()
		job.Name = createJobName()
		sourceJobNameToNewNameMap[sourceJob.Name] = job.Name
		radixBatch.Spec.Jobs = append(radixBatch.Spec.Jobs, *job)
	}
	return sourceJobNameToNewNameMap
}

func (h *handler) copyPayloadSecrets(ctx context.Context, sourceRadixBatch *radixv1.RadixBatch, namespace, appName, radixComponentName, batchName string, sourceJobNameToNewNameMap map[string]string, sourceJobName string) (map[string]string, error) {
	sourceBatchSecrets, err := h.GetSecretsForRadixBatch(sourceRadixBatch.GetName())
	if err != nil {
		return nil, err
	}
	var payloadSecrets []*corev1.Secret
	sourceSecretNameToNewNameMap := make(map[string]string, len(sourceBatchSecrets))
	for _, sourceSecret := range sourceBatchSecrets {
		payloadsSecret := buildSecret(appName, radixComponentName, batchName, len(payloadSecrets))
		for sourceJobNameAsKey, value := range sourceSecret.Data {
			if sourceJobName != "" && sourceJobName != sourceJobNameAsKey {
				continue
			}
			if jobName, ok := sourceJobNameToNewNameMap[sourceJobNameAsKey]; ok {
				payloadsSecret.Data[jobName] = value
			}
		}
		if len(payloadsSecret.Data) == 0 {
			continue
		}
		sourceSecretNameToNewNameMap[sourceSecret.GetName()] = payloadsSecret.GetName()
		payloadSecrets = append(payloadSecrets, payloadsSecret)
	}
	err = h.createSecrets(ctx, namespace, payloadSecrets)
	if err != nil {
		return nil, err
	}
	return sourceSecretNameToNewNameMap, nil
}

func (h *handler) buildRadixBatchJobs(ctx context.Context, namespace, appName, radixJobComponentName, batchName string, batchScheduleDescription common.BatchScheduleDescription, radixJobComponentPayload *radixv1.RadixJobComponentPayload) ([]radixv1.RadixBatchJob, error) {
	var radixBatchJobWithDescriptions []radixBatchJobWithDescription
	var errs []error

	for _, jobScheduleDescription := range batchScheduleDescription.JobScheduleDescriptions {
		jobScheduleDescription := jobScheduleDescription
		radixBatchJob, err := buildRadixBatchJob(&jobScheduleDescription, batchScheduleDescription.DefaultRadixJobComponentConfig)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		radixBatchJobWithDescriptions = append(radixBatchJobWithDescriptions, radixBatchJobWithDescription{
			radixBatchJob:          radixBatchJob,
			jobScheduleDescription: &jobScheduleDescription,
		})
	}
	if len(errs) > 0 {
		return nil, commonErrors.Concat(errs)
	}
	radixJobComponentHasPayloadPath := radixJobComponentPayload != nil && len(radixJobComponentPayload.Path) > 0
	err := h.createRadixBatchJobPayloadSecrets(ctx, namespace, appName, radixJobComponentName, batchName, radixBatchJobWithDescriptions, radixJobComponentHasPayloadPath)
	if err != nil {
		return nil, err
	}
	var radixBatchJobs []radixv1.RadixBatchJob
	for _, item := range radixBatchJobWithDescriptions {
		radixBatchJobs = append(radixBatchJobs, *item.radixBatchJob)
	}
	return radixBatchJobs, nil
}

func createJobName() string {
	return strings.ToLower(utils.RandStringSeed(8, defaultSrc))
}

func (h *handler) createRadixBatchJobPayloadSecrets(ctx context.Context, namespace, appName, radixJobComponentName, batchName string, radixJobWithPayloadEntries []radixBatchJobWithDescription, radixJobComponentHasPayloadPath bool) error {
	accumulatedSecretSize := 0
	var payloadSecrets []*corev1.Secret
	payloadsSecret := buildSecret(appName, radixJobComponentName, batchName, 0)
	payloadSecrets = append(payloadSecrets, payloadsSecret)
	var errs []error
	for jobIndex, radixJobWithDescriptions := range radixJobWithPayloadEntries {
		payload := []byte(strings.TrimSpace(radixJobWithDescriptions.jobScheduleDescription.Payload))
		if len(payload) == 0 {
			log.Debugf("no payload in the job #%d", jobIndex)
			continue
		}
		if !radixJobComponentHasPayloadPath {
			errs = append(errs, fmt.Errorf("missing an expected payload path, but there is a payload in the job #%d", jobIndex))
			continue
		}

		radixBatchJob := radixJobWithDescriptions.radixBatchJob
		payloadBase64 := base64.RawStdEncoding.EncodeToString(payload)
		secretEntrySize := len(payloadBase64) + len(radixBatchJob.Name) + payloadSecretEntryAuxDataSize // preliminary estimate of a payload secret entry
		if payloadSecretAuxDataSize+accumulatedSecretSize+secretEntrySize > maxPayloadSecretSize {
			if len(payloadsSecret.Data) == 0 {
				// this is the first entry in the secret, and it is too large to be stored to the secret - no reason to create new secret.
				return fmt.Errorf("payload is too large in the job #%d - its base64 size is %d bytes, but it is expected to be less then %d bytes", jobIndex, secretEntrySize, maxPayloadSecretSize)
			}
			payloadsSecret = buildSecret(appName, radixJobComponentName, batchName, len(payloadSecrets))
			payloadSecrets = append(payloadSecrets, payloadsSecret)
			accumulatedSecretSize = 0
		}

		payloadsSecret.Data[radixBatchJob.Name] = payload
		accumulatedSecretSize = accumulatedSecretSize + secretEntrySize

		radixBatchJob.PayloadSecretRef = &radixv1.PayloadSecretKeySelector{
			LocalObjectReference: radixv1.LocalObjectReference{Name: payloadsSecret.GetName()},
			Key:                  radixBatchJob.Name,
		}
	}
	if len(errs) > 0 {
		return commonErrors.Concat(errs)
	}
	return h.createSecrets(ctx, namespace, payloadSecrets)
}

func (h *handler) createSecrets(ctx context.Context, namespace string, secrets []*corev1.Secret) error {
	for _, secret := range secrets {
		if secret.Data == nil || len(secret.Data) == 0 {
			continue // if Data is empty, the secret is not used in any jobs
		}
		_, err := h.kubeUtil.KubeClient().CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func buildSecret(appName, radixJobComponentName, batchName string, secretIndex int) *corev1.Secret {
	secretName := fmt.Sprintf("%s-payloads-%d", batchName, secretIndex)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: radixLabels.Merge(
				radixLabels.ForApplicationName(appName),
				radixLabels.ForComponentName(radixJobComponentName),
				radixLabels.ForBatchName(batchName),
				radixLabels.ForJobScheduleJobType(),
			),
		},
		Data: make(map[string][]byte),
	}
}

func buildRadixBatchJob(jobScheduleDescription *common.JobScheduleDescription, defaultJobScheduleDescription *common.RadixJobComponentConfig) (*radixv1.RadixBatchJob, error) {
	err := applyDefaultJobDescriptionProperties(jobScheduleDescription, defaultJobScheduleDescription)
	if err != nil {
		return nil, err
	}
	return &radixv1.RadixBatchJob{
		Name:             createJobName(),
		JobId:            jobScheduleDescription.JobId,
		Resources:        jobScheduleDescription.Resources,
		Node:             jobScheduleDescription.Node,
		TimeLimitSeconds: jobScheduleDescription.TimeLimitSeconds,
		BackoffLimit:     jobScheduleDescription.BackoffLimit,
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
		return nil, err
	}

	return slice.PointersOf(radixBatchList.Items).([]*radixv1.RadixBatch), nil
}

func (h *handler) getRadixBatch(ctx context.Context, batchName string) (*radixv1.RadixBatch, error) {
	return h.kubeUtil.RadixClient().RadixV1().RadixBatches(h.env.RadixDeploymentNamespace).Get(ctx, batchName, metav1.GetOptions{})
}

// GarbageCollectPayloadSecrets Delete orphaned payload secrets
func (h *handler) GarbageCollectPayloadSecrets(ctx context.Context) error {
	radixBatchNameLabelExists, err := radixLabels.RequirementRadixBatchNameLabelExists()
	if err != nil {
		return err
	}
	payloadSecrets, err := h.kubeUtil.ListSecretsWithSelector(h.env.RadixDeploymentNamespace, radixBatchNameLabelExists.String())
	if err != nil {
		return err
	}
	radixBatches, err := h.getRadixBatches(ctx, radixLabels.ForComponentName(h.env.RadixComponentName))
	if err != nil {
		return err
	}
	batchNames := make(map[string]bool)
	for _, radixBatch := range radixBatches {
		batchNames[radixBatch.Name] = true
	}
	yesterday := time.Now().Add(time.Hour * -24)
	for _, payloadSecret := range payloadSecrets {
		if payloadSecret.GetCreationTimestamp().After(yesterday) {
			continue
		}
		payloadSecretBatchName, ok := payloadSecret.GetLabels()[kube.RadixBatchNameLabel]
		if !ok {
			continue
		}
		if _, ok := batchNames[payloadSecretBatchName]; !ok {
			err := h.DeleteSecret(payloadSecret)
			if err != nil {
				log.Errorf("failed deleting of an orphaned payload secret %s in the namespace %s", payloadSecret.GetName(), payloadSecret.GetNamespace())
			}
		}
	}
	return nil
}

func applyDefaultJobDescriptionProperties(jobScheduleDescription *common.JobScheduleDescription, defaultRadixJobComponentConfig *common.RadixJobComponentConfig) error {
	if jobScheduleDescription == nil || defaultRadixJobComponentConfig == nil {
		return nil
	}
	return mergo.Merge(&jobScheduleDescription.RadixJobComponentConfig, defaultRadixJobComponentConfig, mergo.WithTransformers(authTransformer))
}
