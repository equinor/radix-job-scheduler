package jobs

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/equinor/radix-common/utils/slice"
	apierrors "github.com/equinor/radix-job-scheduler/api/errors"
	apiv1 "github.com/equinor/radix-job-scheduler/api/v1"
	"github.com/equinor/radix-job-scheduler/internal/config"
	"github.com/equinor/radix-job-scheduler/internal/names"
	"github.com/equinor/radix-job-scheduler/internal/predicates"
	"github.com/equinor/radix-job-scheduler/internal/query"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-job-scheduler/pkg/actions"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type jobHandler struct {
	apiv1.Handler
}

type JobHandler interface {
	// GetJobs Get status of all jobs
	GetJobs(ctx context.Context) ([]modelsv1.JobStatus, error)
	// GetJob Get status of a job
	GetJob(ctx context.Context, jobName string) (*modelsv1.JobStatus, error)
	// CreateJob Create a job with parameters
	CreateJob(ctx context.Context, jobScheduleDescription common.JobScheduleDescription) (*modelsv1.JobStatus, error)
	// DeleteJob Delete a job
	DeleteJob(ctx context.Context, jobName string) error
	// StopJob Stop a job
	StopJob(ctx context.Context, jobName string) error
}

// New Constructor for job handler
func New(kube *kube.Kube, config *config.Config, radixDeployJobComponent *radixv1.RadixDeployJobComponent) JobHandler {
	return &jobHandler{
		apiv1.Handler{
			Kube:                    kube,
			Config:                  config,
			RadixDeployJobComponent: radixDeployJobComponent,
		},
	}
}

// GetJobs Get status of all jobs
func (h *jobHandler) GetJobs(ctx context.Context) ([]modelsv1.JobStatus, error) {
	rbList, err := h.ListRadixBatches(ctx, kube.RadixBatchTypeJob, kube.RadixBatchTypeBatch)
	if err != nil {
		return nil, fmt.Errorf("failed to list batches: %w", err)
	}
	modelMapperFunc, err := h.CreateBatchStatusMapper(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create model mapper: %w", err)
	}
	batchStatuses := slice.Map(rbList, modelMapperFunc)
	jobStatuses := slices.Concat(slice.Map(batchStatuses, func(b modelsv1.BatchStatus) []modelsv1.JobStatus { return b.JobStatuses })...)
	return jobStatuses, nil
}

// GetJob Get status of a job
func (h *jobHandler) GetJob(ctx context.Context, jobName string) (*modelsv1.JobStatus, error) {
	batchName, _, ok := names.ParseRadixBatchAndJobNameFromFullyQualifiedJobName(jobName)
	if !ok {
		return nil, apierrors.NewNotFoundError("job", jobName, fmt.Errorf("failed to parse job name %s", jobName))
	}
	rb, err := h.GetRadixBatch(ctx, batchName)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil, apierrors.NewNotFoundError("job", jobName, err)
		}
		return nil, fmt.Errorf("failed to get batch: %w", err)
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

// CreateJob Create a job with parameters
func (h *jobHandler) CreateJob(ctx context.Context, jobScheduleDescription common.JobScheduleDescription) (*modelsv1.JobStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Create job for namespace: %s", h.Config.RadixDeploymentNamespace)

	batchStatus, err := h.CreateNewBatchFromRequest(
		ctx,
		common.BatchScheduleDescription{JobScheduleDescriptions: []common.JobScheduleDescription{jobScheduleDescription}},
		kube.RadixBatchTypeJob,
	)
	if err != nil {
		return nil, apierrors.NewFromError(err)
	}

	return &batchStatus.JobStatuses[0], nil
}

// DeleteJob Delete a job
func (h *jobHandler) DeleteJob(ctx context.Context, jobName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("delete job %s for namespace: %s", jobName, h.Config.RadixDeploymentNamespace)
	parsedBatchName, parsedJobName, ok := names.ParseRadixBatchAndJobNameFromFullyQualifiedJobName(jobName)
	if !ok {
		return apierrors.NewInvalidWithReason(jobName, "is not a valid job name")
	}
	rb, err := query.GetRadixBatch(ctx, h.Kube.RadixClient(), h.Config.RadixDeploymentNamespace, parsedBatchName)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return apierrors.NewNotFoundError("job", jobName, nil)
		}
		return apierrors.NewFromError(err)
	}
	if rb.Labels[kube.RadixBatchTypeLabel] != string(kube.RadixBatchTypeJob) {
		return apierrors.NewInvalidWithReason(jobName, "not a single job")
	}
	if !slice.Any(rb.Spec.Jobs, predicates.IsRadixBatchJobWithName(parsedJobName)) {
		return apierrors.NewNotFoundError("job", jobName, nil)
	}
	if err := h.Kube.RadixClient().RadixV1().RadixBatches(h.Config.RadixDeploymentNamespace).Delete(ctx, parsedBatchName, v1.DeleteOptions{}); err != nil {
		if kubeerrors.IsNotFound(err) {
			return apierrors.NewNotFoundError("job", jobName, nil)
		}
		return apierrors.NewFromError(err)
	}
	return nil
}

// StopJob Stop a job
func (h *jobHandler) StopJob(ctx context.Context, jobName string) error {
	batchName, parsedJobName, ok := names.ParseRadixBatchAndJobNameFromFullyQualifiedJobName(jobName)
	if !ok {
		return apierrors.NewNotFoundError("job", jobName, fmt.Errorf("failed to parse job name %s", jobName))
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
