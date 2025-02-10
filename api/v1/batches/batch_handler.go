package batchesv1

import (
	"context"
	"errors"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	apierrors "github.com/equinor/radix-job-scheduler/api/errors"
	apiv1 "github.com/equinor/radix-job-scheduler/api/v1"
	"github.com/equinor/radix-job-scheduler/internal/config"
	"github.com/equinor/radix-job-scheduler/internal/names"
	"github.com/equinor/radix-job-scheduler/internal/predicates"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-job-scheduler/pkg/actions"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type batchHandler struct {
	apiv1.Handler
}

type BatchHandler interface {
	// GetBatches Get status of all batches
	GetBatches(ctx context.Context) ([]modelsv1.BatchStatus, error)
	// GetBatch Get status of a batch
	GetBatch(ctx context.Context, batchName string) (*modelsv1.BatchStatus, error)
	// GetBatchJob Get status of a batch job
	GetBatchJob(ctx context.Context, batchName string, jobName string) (*modelsv1.JobStatus, error)
	// CreateBatch Create a batch with parameters
	CreateBatch(ctx context.Context, batchScheduleDescription common.BatchScheduleDescription) (*modelsv1.BatchStatus, error)
	// DeleteBatch Delete a batch
	DeleteBatch(ctx context.Context, batchName string) error
	// StopBatch Stop a batch
	StopBatch(ctx context.Context, batchName string) error
	// StopBatchJob Stop a batch job
	StopBatchJob(ctx context.Context, batchName string, jobName string) error
}

// New Constructor of the batch handler
func New(kube *kube.Kube, config *config.Config, radixDeployJobComponent *radixv1.RadixDeployJobComponent) BatchHandler {
	return &batchHandler{
		apiv1.Handler{
			Kube:                    kube,
			Config:                  config,
			RadixDeployJobComponent: radixDeployJobComponent,
		},
	}
}

// GetBatches Get status of all batches
func (h *batchHandler) GetBatches(ctx context.Context) ([]modelsv1.BatchStatus, error) {
	rbList, err := h.ListRadixBatches(ctx, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, fmt.Errorf("failed to list batches: %w", err)
	}
	modelMapperFunc, err := h.CreateBatchStatusMapper(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create model mapper: %w", err)
	}
	batchStatuses := slice.Map(rbList, modelMapperFunc)
	return batchStatuses, nil
}

// GetBatchJob Get status of a batch job
func (h *batchHandler) GetBatchJob(ctx context.Context, batchName string, jobName string) (*modelsv1.JobStatus, error) {
	rb, err := h.GetRadixBatch(ctx, batchName)
	if err != nil {
		return nil, apierrors.NewNotFoundError("job", jobName, err)
	}
	modelMapperFunc, err := h.CreateBatchStatusMapper(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create model mapper: %w", err)
	}
	batchStatus := modelMapperFunc(*rb)
	jobStatus, ok := slice.FindFirst(batchStatus.JobStatuses, predicates.IsJobStatusWithName(jobName))
	if !ok {
		return nil, apierrors.NewNotFoundError("job", jobName, nil)
	}
	return &jobStatus, nil
}

// GetBatch Get status of a batch
func (h *batchHandler) GetBatch(ctx context.Context, batchName string) (*modelsv1.BatchStatus, error) {
	rb, err := h.GetRadixBatch(ctx, batchName)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch: %w", err)
	}
	modelMapperFunc, err := h.CreateBatchStatusMapper(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create model mapper: %w", err)
	}
	batchStatus := modelMapperFunc(*rb)
	return &batchStatus, nil
}

// CreateBatch Create a batch with parameters
func (h *batchHandler) CreateBatch(ctx context.Context, batchScheduleDescription common.BatchScheduleDescription) (*modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("create batch for namespace: %s", h.Config.RadixDeploymentNamespace)
	batchStatus, err := h.CreateNewBatchFromRequest(ctx, batchScheduleDescription, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, apierrors.NewFromError(err)
	}
	return batchStatus, nil
}

// DeleteBatch Delete a batch
func (h *batchHandler) DeleteBatch(ctx context.Context, batchName string) error {
	if err := h.Kube.RadixClient().RadixV1().RadixBatches(h.Config.RadixDeploymentNamespace).Delete(ctx, batchName, v1.DeleteOptions{}); err != nil {
		if kubeerrors.IsNotFound(err) {
			return apierrors.NewNotFoundError("batch", batchName, err)
		}
		return fmt.Errorf("failed to delete batch: %w", err)
	}
	return nil
}

// StopBatch Stop a batch
func (h *batchHandler) StopBatch(ctx context.Context, batchName string) error {
	rb, err := h.GetRadixBatch(ctx, batchName)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return apierrors.NewNotFoundError("batch", batchName, err)
		}
		return fmt.Errorf("failed to get batch: %w", err)
	}

	stoppedRb, err := actions.StopAllRadixBatchJobs(rb)
	if err != nil {
		if errors.Is(err, actions.ErrStopCompletedRadixBatch) {
			return apierrors.NewBadRequestError("cannot stop completed batch", err)
		}
		return fmt.Errorf("failed to stop batch: %w", err)
	}

	if _, err = h.UpdateRadixBatch(ctx, stoppedRb); err != nil {
		return apierrors.NewInternalError(fmt.Errorf("failed to update batch: %w", err))
	}

	return nil
}

// StopBatchJob Stop a batch job
func (h *batchHandler) StopBatchJob(ctx context.Context, batchName string, jobName string) error {
	parsedBatchName, parsedJobName, ok := names.ParseRadixBatchAndJobNameFromFullyQualifiedJobName(jobName)
	if !ok {
		return apierrors.NewNotFoundError("job", jobName, fmt.Errorf("failed to parse job name %s", jobName))
	}
	if batchName != parsedBatchName {
		return apierrors.NewNotFoundError("job", jobName, nil)
	}

	rb, err := h.GetRadixBatch(ctx, batchName)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return apierrors.NewNotFoundError("job", jobName, err)
		}
		return fmt.Errorf("failed to get batch: %w", err)
	}

	stoppedRb, err := actions.StopNamedRadixBatchJob(rb, parsedJobName)
	if err != nil {
		switch {
		case errors.Is(err, actions.ErrStopCompletedRadixBatch):
			return apierrors.NewBadRequestError("cannot stop completed batch", err)
		case errors.Is(err, actions.ErrStopJobNotFound):
			return apierrors.NewNotFoundError("job", jobName, err)
		default:
			return fmt.Errorf("failed to stop job: %w", err)
		}
	}

	if _, err = h.UpdateRadixBatch(ctx, stoppedRb); err != nil {
		return apierrors.NewInternalError(fmt.Errorf("failed to update batch: %w", err))
	}

	return nil
}
