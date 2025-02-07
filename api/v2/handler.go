package apiv2

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"dario.cat/mergo"
	mergoutils "github.com/equinor/radix-common/utils/mergo"
	"github.com/equinor/radix-common/utils/pointers"
	apierrors "github.com/equinor/radix-job-scheduler/api/errors"
	"github.com/equinor/radix-job-scheduler/internal/config"
	"github.com/equinor/radix-job-scheduler/internal/names"
	"github.com/equinor/radix-job-scheduler/internal/query"
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
	cfg                     *config.Config
	radixDeployJobComponent *radixv1.RadixDeployJobComponent
}

type Handler interface {
	// GetRadixBatches Get status of all batches
	GetRadixBatches(ctx context.Context) ([]modelsv2.Batch, error)
	// GetRadixBatchSingleJobs Get status of all single jobs
	GetRadixBatchSingleJobs(ctx context.Context) ([]modelsv2.Batch, error)
	// GetRadixBatch Get a batch
	GetRadixBatch(ctx context.Context, batchName string) (*modelsv2.Batch, error)
	// CreateRadixBatch Create a batch with parameters
	CreateRadixBatch(ctx context.Context, batchScheduleDescription *common.BatchScheduleDescription) (*modelsv2.Batch, error)
	// CopyRadixBatch Copy a batch with deployment and optional parameters
	CopyRadixBatch(ctx context.Context, batchName, deploymentName string) (*modelsv2.Batch, error)
	// CreateRadixBatchSingleJob Create a batch with single job parameters
	CreateRadixBatchSingleJob(ctx context.Context, jobScheduleDescription *common.JobScheduleDescription) (*modelsv2.Batch, error)
	// DeleteRadixBatchJob Delete a single job
	DeleteRadixBatchJob(ctx context.Context, jobName string) error
	// StopRadixBatch Stop a batch
	StopRadixBatch(ctx context.Context, batchName string) error
	// StopRadixBatchJob Stop a batch job
	StopRadixBatchJob(ctx context.Context, batchName, jobName string) error
	// RestartRadixBatch Restart a batch
	RestartRadixBatch(ctx context.Context, batchName string) error
	// RestartRadixBatchJob Restart a batch job
	RestartRadixBatchJob(ctx context.Context, batchName, jobName string) error
}

// New Constructor of the batch handler
func New(kubeUtil *kube.Kube, cfg *config.Config, radixDeployJobComponent *radixv1.RadixDeployJobComponent) Handler {
	return &handler{
		kubeUtil:                kubeUtil,
		cfg:                     cfg,
		radixDeployJobComponent: radixDeployJobComponent,
	}
}

type radixBatchJobWithDescription struct {
	radixBatchJob          *radixv1.RadixBatchJob
	jobScheduleDescription *common.JobScheduleDescription
}

// GetRadixBatches Get statuses of all batches
func (h *handler) GetRadixBatches(ctx context.Context) ([]modelsv2.Batch, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Get batches for the namespace: %s", h.cfg.RadixDeploymentNamespace)
	return h.getRadixBatchStatuses(ctx, kube.RadixBatchTypeBatch)
}

// GetRadixBatchSingleJobs Get statuses of all single jobs
func (h *handler) GetRadixBatchSingleJobs(ctx context.Context) ([]modelsv2.Batch, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Get sigle jobs for the namespace: %s", h.cfg.RadixDeploymentNamespace)
	return h.getRadixBatchStatuses(ctx, kube.RadixBatchTypeJob)
}

// GetRadixBatch Get status of a batch
func (h *handler) GetRadixBatch(ctx context.Context, batchName string) (*modelsv2.Batch, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("get batch status for the batch %s", batchName)
	radixBatch, err := query.GetRadixBatch(ctx, h.kubeUtil.RadixClient(), h.cfg.RadixDeploymentNamespace, batchName)
	if err != nil {
		return nil, err
	}
	return pointers.Ptr(batch.GetRadixBatchStatus(radixBatch, h.radixDeployJobComponent)), nil
}

// CreateRadixBatch Create a batch with parameters
func (h *handler) CreateRadixBatch(ctx context.Context, batchScheduleDescription *common.BatchScheduleDescription) (*modelsv2.Batch, error) {
	logger := log.Ctx(ctx)
	if batchScheduleDescription == nil {
		return nil, apierrors.NewInvalidWithReason("BatchScheduleDescription", "empty request body")
	}
	logger.Info().Msgf("Create Radix Batch for %d jobs", len(batchScheduleDescription.JobScheduleDescriptions))

	if len(batchScheduleDescription.JobScheduleDescriptions) == 0 {
		return nil, apierrors.NewInvalidWithReason("BatchScheduleDescription", "empty job description list ")
	}

	radixBatch, err := h.createRadixBatchOrJob(ctx, *batchScheduleDescription, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, err
	}
	logger.Info().Msgf("Radix Batch %s has been created", radixBatch.Name)
	return radixBatch, nil
}

// CopyRadixBatch Copy a batch with deployment and optional parameters
func (h *handler) CopyRadixBatch(ctx context.Context, sourceBatchName, deploymentName string) (*modelsv2.Batch, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Copy Radix Batch %s for the deployment %s", sourceBatchName, deploymentName)
	sourceRadixBatch, err := query.GetRadixBatch(ctx, h.kubeUtil.RadixClient(), h.cfg.RadixDeploymentNamespace, sourceBatchName)
	if err != nil {
		return nil, err
	}
	rb, err := batch.CopyRadixBatchOrJob(ctx, h.kubeUtil.RadixClient(), sourceRadixBatch, "", deploymentName)
	if err != nil {
		return nil, err
	}
	batchStatus := batch.GetRadixBatchStatus(rb, h.radixDeployJobComponent)
	return &batchStatus, nil
}

func (h *handler) createRadixBatchOrJob(ctx context.Context, batchScheduleDescription common.BatchScheduleDescription, radixBatchType kube.RadixBatchType) (*modelsv2.Batch, error) {
	logger := log.Ctx(ctx)
	namespace := h.cfg.RadixDeploymentNamespace
	radixComponentName := h.cfg.RadixComponentName
	radixDeploymentName := h.cfg.RadixDeploymentName
	logger.Info().Msgf("Create batch for namespace %s, component %s, deployment %s", namespace, radixComponentName, radixDeploymentName)

	radixDeployment, err := h.kubeUtil.RadixClient().RadixV1().RadixDeployments(namespace).Get(ctx, radixDeploymentName, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil, apierrors.NewNotFoundError("radix deployment", radixDeploymentName, nil)
		}
		return nil, apierrors.NewFromError(err)
	}

	radixJobComponent := radixDeployment.GetJobComponentByName(radixComponentName)
	if radixJobComponent == nil {
		return nil, apierrors.NewNotFoundError("job component", radixComponentName, nil)
	}

	appName := radixDeployment.Spec.AppName

	createdRadixBatch, err := h.createRadixBatch(ctx, namespace, appName, radixDeployment.GetName(), *radixJobComponent, batchScheduleDescription, radixBatchType)
	if err != nil {
		return nil, apierrors.NewFromError(err)
	}

	logger.Debug().Msgf("created batch %s for component %s, environment %s, in namespace: %s", createdRadixBatch.GetName(),
		radixComponentName, radixDeployment.Spec.Environment, namespace)
	return pointers.Ptr(batch.GetRadixBatchStatus(createdRadixBatch, h.radixDeployJobComponent)), nil
}

// CreateRadixBatchSingleJob Create a batch single job with parameters
func (h *handler) CreateRadixBatchSingleJob(ctx context.Context, jobScheduleDescription *common.JobScheduleDescription) (*modelsv2.Batch, error) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Create Radix Batch single job")
	if jobScheduleDescription == nil {
		return nil, apierrors.NewInvalidWithReason("JobScheduleDescription", "empty request body")
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

// DeleteRadixBatchJob Delete a batch job
func (h *handler) DeleteRadixBatchJob(ctx context.Context, jobName string) error {
	batchName, _, ok := names.ParseRadixBatchAndJobNameFromFullyQualifiedJobName(jobName)
	if !ok {
		return fmt.Errorf("deleting of this job is not supported or invalid job name")
	}
	radixBatch, err := query.GetRadixBatch(ctx, h.kubeUtil.RadixClient(), h.cfg.RadixDeploymentNamespace, batchName)
	if err != nil {
		return err
	}
	if radixBatch.Labels[kube.RadixBatchTypeLabel] != string(kube.RadixBatchTypeJob) {
		return errors.New("not a single job")
	}
	return h.kubeUtil.RadixClient().RadixV1().RadixBatches(radixBatch.Namespace).Delete(ctx, radixBatch.Namespace, metav1.DeleteOptions{})
}

// StopRadixBatch Stop a batch
func (h *handler) StopRadixBatch(ctx context.Context, batchName string) error {
	radixBatch, err := query.GetRadixBatch(ctx, h.kubeUtil.RadixClient(), h.cfg.RadixDeploymentNamespace, batchName)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return apierrors.NewNotFoundError("batch", batchName, nil)
		}
		return apierrors.NewFromError(err)
	}
	return batch.StopRadixBatch(ctx, h.kubeUtil.RadixClient(), radixBatch)
}

// StopRadixBatchJob Stop a batch job
func (h *handler) StopRadixBatchJob(ctx context.Context, batchName, jobName string) error {
	radixBatch, err := query.GetRadixBatch(ctx, h.kubeUtil.RadixClient(), h.cfg.RadixDeploymentNamespace, batchName)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return apierrors.NewNotFoundError("job", jobName, err)
		}
		return err
	}
	return batch.StopRadixBatchJob(ctx, h.kubeUtil.RadixClient(), radixBatch, jobName)
}

// RestartRadixBatch Restart a batch
func (h *handler) RestartRadixBatch(ctx context.Context, batchName string) error {
	radixBatch, err := query.GetRadixBatch(ctx, h.kubeUtil.RadixClient(), h.cfg.RadixDeploymentNamespace, batchName)
	if err != nil {
		return apierrors.NewFromError(err)
	}
	return batch.RestartRadixBatch(ctx, h.kubeUtil.RadixClient(), radixBatch)
}

// RestartRadixBatchJob Restart a batch job
func (h *handler) RestartRadixBatchJob(ctx context.Context, batchName, jobName string) error {
	radixBatch, err := query.GetRadixBatch(ctx, h.kubeUtil.RadixClient(), h.cfg.RadixDeploymentNamespace, batchName)
	if err != nil {
		return err
	}
	return batch.RestartRadixBatchJob(ctx, h.kubeUtil.RadixClient(), radixBatch, jobName)
}

func (h *handler) createRadixBatch(ctx context.Context, namespace, appName, radixDeploymentName string, radixJobComponent radixv1.RadixDeployJobComponent, batchScheduleDescription common.BatchScheduleDescription, radixBatchType kube.RadixBatchType) (*radixv1.RadixBatch, error) {
	logger := log.Ctx(ctx)
	batchName := names.NewRadixBatchName(radixJobComponent.GetName())
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
			BatchId: batchScheduleDescription.BatchId,
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
		return nil, apierrors.NewFromError(err)
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
		return nil, apierrors.NewFromError(errors.Join(errs...))
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
		return apierrors.NewFromError(errors.Join(errs...))
	}
	logger.Debug().Msg("Create payload secrets")
	return h.createSecrets(ctx, namespace, payloadSecrets)
}

func (h *handler) createSecrets(ctx context.Context, namespace string, secrets []*corev1.Secret) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Create %d secrets", len(secrets))
	for _, secret := range secrets {
		if len(secret.Data) == 0 {
			logger.Debug().Msgf("Do not create a secret %s - Data is empty, the secret is not used in any jobs", secret.GetName())
			continue
		}
		logger.Debug().Msgf("Create a secret %s in the cluster", secret.GetName())
		_, err := h.kubeUtil.KubeClient().CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return apierrors.NewFromError(err)
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
		return nil, apierrors.NewFromError(err)
	}
	return &radixv1.RadixBatchJob{
		Name:             names.NewRadixBatchJobName(),
		JobId:            jobScheduleDescription.JobId,
		Resources:        jobScheduleDescription.Resources.MapToRadixResourceRequirements(),
		Node:             jobScheduleDescription.Node.MapToRadixNode(),
		TimeLimitSeconds: jobScheduleDescription.TimeLimitSeconds,
		BackoffLimit:     jobScheduleDescription.BackoffLimit,
		ImageTagName:     jobScheduleDescription.ImageTagName,
		FailurePolicy:    jobScheduleDescription.FailurePolicy.MapToRadixFailurePolicy(),
	}, nil
}

func (h *handler) getRadixBatchStatuses(ctx context.Context, radixBatchType kube.RadixBatchType) ([]modelsv2.Batch, error) {
	logger := log.Ctx(ctx)
	selector := radixLabels.Merge(
		radixLabels.ForComponentName(h.radixDeployJobComponent.GetName()),
		radixLabels.ForBatchType(radixBatchType),
	).AsSelector()
	radixBatches, err := query.ListRadixBatches(ctx, h.cfg.RadixDeploymentNamespace, h.kubeUtil.RadixClient(), selector)
	if err != nil {
		return nil, err
	}
	logger.Debug().Msgf("Found %v batches", len(radixBatches))
	radixBatchStatuses := batch.GetRadixBatchStatuses(radixBatches, h.radixDeployJobComponent)
	return radixBatchStatuses, nil
}

func applyDefaultJobDescriptionProperties(jobScheduleDescription *common.JobScheduleDescription, defaultRadixJobComponentConfig *common.RadixJobComponentConfig) error {
	if jobScheduleDescription == nil || defaultRadixJobComponentConfig == nil {
		return nil
	}

	return mergo.Merge(&jobScheduleDescription.RadixJobComponentConfig, defaultRadixJobComponentConfig, mergo.WithTransformers(authTransformer))
}
