package apiv2

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"dario.cat/mergo"
	mergoutils "github.com/equinor/radix-common/utils/mergo"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/internal"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	CopyRadixBatchSingleJob(ctx context.Context, jobName, deploymentName string) (*modelsv2.RadixBatch, error)
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
	return h.getRadixBatchStatuses(ctx, kube.RadixBatchTypeBatch)
}

// GetRadixBatchSingleJobs Get statuses of all single jobs
func (h *handler) GetRadixBatchSingleJobs(ctx context.Context) ([]modelsv2.RadixBatch, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Get sigle jobs for the namespace: %s", h.env.RadixDeploymentNamespace)
	return h.getRadixBatchStatuses(ctx, kube.RadixBatchTypeJob)
}

// GetRadixBatch Get status of a batch
func (h *handler) GetRadixBatch(ctx context.Context, batchName string) (*modelsv2.RadixBatch, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("get batch status for the batch %s for namespace: %s", batchName, h.env.RadixDeploymentNamespace)
	radixBatch, err := internal.GetRadixBatch(ctx, h.kubeUtil.RadixClient(), h.env.RadixDeploymentNamespace, batchName)
	if err != nil {
		return nil, err
	}
	return pointers.Ptr(batch.GetRadixBatchStatus(radixBatch, h.radixDeployJobComponent)), nil
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
	return batch.CopyRadixBatchOrJob(ctx, h.kubeUtil.RadixClient(), h.env.RadixDeploymentNamespace, batchName, "", h.radixDeployJobComponent, deploymentName)
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
	return pointers.Ptr(batch.GetRadixBatchStatus(createdRadixBatch, h.radixDeployJobComponent)), nil
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
func (h *handler) CopyRadixBatchSingleJob(ctx context.Context, jobName, deploymentName string) (*modelsv2.RadixBatch, error) {
	return batch.CopyRadixBatchOrJob(ctx, h.kubeUtil.RadixClient(), h.env.RadixDeploymentNamespace, "", jobName, h.radixDeployJobComponent, deploymentName)
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
	namespace := h.env.RadixDeploymentNamespace
	radixClient := h.kubeUtil.RadixClient()
	radixBatch, err := radixClient.RadixV1().RadixBatches(namespace).Get(ctx, batchName, v1.GetOptions{})
	if err != nil {
		return errors.NewFromError(err)
	}
	return batch.StopRadixBatch(ctx, radixClient, namespace, radixBatch)
}

// StopRadixBatchJob Stop a batch job
func (h *handler) StopRadixBatchJob(ctx context.Context, batchName string, jobName string) error {
	namespace := h.env.RadixDeploymentNamespace
	radixClient := h.kubeUtil.RadixClient()
	radixBatch, err := radixClient.RadixV1().RadixBatches(namespace).Get(ctx, batchName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	return batch.StopRadixBatchJob(ctx, radixClient, namespace, radixBatch, jobName)
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
	radixBatches, err := internal.GetRadixBatches(ctx, h.env.RadixDeploymentNamespace, h.kubeUtil.RadixClient(), radixLabels.ForComponentName(h.env.RadixComponentName))
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
	return internal.GetRadixBatchModelsFromRadixBatches(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeBatch) && internal.IsRadixBatchNotSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch)
	}), h.radixDeployJobComponent)
}

func (h *handler) getSucceededRadixBatches(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.RadixBatch {
	radixBatches = slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeBatch) && internal.IsRadixBatchSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch)
	})
	return internal.GetRadixBatchModelsFromRadixBatches(radixBatches, h.radixDeployJobComponent)
}

func radixBatchIsCompletedBefore(completedBefore time.Time, radixBatch *radixv1.RadixBatch) bool {
	return radixBatch.Status.Condition.CompletionTime != nil && (*radixBatch.Status.Condition.CompletionTime).Before(&metav1.Time{Time: completedBefore})
}

func (h *handler) getNotSucceededSingleJobs(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.RadixBatch {
	return internal.GetRadixBatchModelsFromRadixBatches(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeJob) && internal.IsRadixBatchNotSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch)
	}), h.radixDeployJobComponent)
}

func (h *handler) getSucceededSingleJobs(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.RadixBatch {
	return internal.GetRadixBatchModelsFromRadixBatches(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeJob) && internal.IsRadixBatchSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch)
	}), h.radixDeployJobComponent)
}

func radixBatchHasType(radixBatch *radixv1.RadixBatch, radixBatchType kube.RadixBatchType) bool {
	return radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(radixBatchType)
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
		radixBatch := radixBatchesSortedByCompletionTimeAsc[i]
		logger.Debug().Msgf("deleting batch %s", radixBatch.Name)
		if err := h.DeleteRadixBatch(ctx, radixBatch.Name); err != nil {
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

func (h *handler) createRadixBatch(ctx context.Context, namespace, appName, radixDeploymentName string, radixJobComponent radixv1.RadixDeployJobComponent, batchScheduleDescription common.BatchScheduleDescription, radixBatchType kube.RadixBatchType) (*radixv1.RadixBatch, error) {
	logger := log.Ctx(ctx)
	batchName := internal.GenerateBatchName(radixJobComponent.GetName())
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
		Name:             internal.CreateJobName(),
		JobId:            jobScheduleDescription.JobId,
		Resources:        jobScheduleDescription.Resources,
		Node:             jobScheduleDescription.Node,
		TimeLimitSeconds: jobScheduleDescription.TimeLimitSeconds,
		BackoffLimit:     jobScheduleDescription.BackoffLimit,
		ImageTagName:     jobScheduleDescription.ImageTagName,
	}, nil
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
	radixBatches, err := internal.GetRadixBatches(ctx, h.env.RadixDeploymentNamespace, h.kubeUtil.RadixClient(), radixLabels.ForComponentName(h.env.RadixComponentName))
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

// getRadixBatchStatuses Get Radix batch statuses
func (h *handler) getRadixBatchStatuses(ctx context.Context, radixBatchType kube.RadixBatchType) ([]modelsv2.RadixBatch, error) {
	logger := log.Ctx(ctx)
	namespace := h.env.RadixDeploymentNamespace
	radixBatches, err := internal.GetRadixBatches(ctx, namespace, h.kubeUtil.RadixClient(),
		radixLabels.ForComponentName(h.radixDeployJobComponent.GetName()),
		radixLabels.ForBatchType(radixBatchType),
	)
	if err != nil {
		return nil, err
	}
	logger.Debug().Msgf("Found %v batches for namespace %s", len(radixBatches), namespace)
	radixBatchStatuses := batch.GetRadixBatchStatuses(radixBatches, h.radixDeployJobComponent)
	return radixBatchStatuses, nil
}
