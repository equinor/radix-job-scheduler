package jobs

import (
	"context"
	"fmt"
	"slices"

	"github.com/equinor/radix-common/utils/slice"
	apierrors "github.com/equinor/radix-job-scheduler/api/errors"
	apiv1 "github.com/equinor/radix-job-scheduler/api/v1"
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"github.com/equinor/radix-job-scheduler/internal"
	"github.com/equinor/radix-job-scheduler/internal/config"
	"github.com/equinor/radix-job-scheduler/internal/names"
	"github.com/equinor/radix-job-scheduler/internal/predicates"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
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
	CreateJob(ctx context.Context, jobScheduleDescription *common.JobScheduleDescription) (*modelsv1.JobStatus, error)
	// CopyJob creates a copy of an existing job with deploymentName as value for radixDeploymentJobRef.name
	CopyJob(ctx context.Context, jobName string, deploymentName string) (*modelsv1.JobStatus, error)
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
			HandlerApiV2:            apiv2.New(kube, config, radixDeployJobComponent),
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
	batchName, _, ok := names.ParseRadixBatchAndJobNameFromJobStatusName(jobName)
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
func (h *jobHandler) CreateJob(ctx context.Context, jobScheduleDescription *common.JobScheduleDescription) (*modelsv1.JobStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Create job for namespace: %s", h.Config.RadixDeploymentNamespace)
	radixBatch, err := h.HandlerApiV2.CreateRadixBatchSingleJob(ctx, jobScheduleDescription)
	if err != nil {
		return nil, err
	}
	return getSingleJobStatusFromRadixBatchJob(radixBatch)
}

// CopyJob Copy a job with  deployment and optional parameters
func (h *jobHandler) CopyJob(ctx context.Context, jobName string, deploymentName string) (*modelsv1.JobStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("stop the job %s for namespace: %s", jobName, h.Config.RadixDeploymentNamespace)
	radixBatch, err := apiv1.CopyJob(ctx, h.HandlerApiV2, jobName, deploymentName)
	if err != nil {
		return nil, err
	}
	return getSingleJobStatusFromRadixBatchJob(radixBatch)
}

// DeleteJob Delete a job
func (h *jobHandler) DeleteJob(ctx context.Context, jobName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("delete job %s for namespace: %s", jobName, h.Config.RadixDeploymentNamespace)
	batchName, _, ok := names.ParseRadixBatchAndJobNameFromJobStatusName(jobName)
	if !ok {
		return apierrors.NewInvalidWithReason(jobName, "is not a valid job name")
	}
	radixBatchStatus, err := h.HandlerApiV2.GetRadixBatch(ctx, batchName)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return apierrors.NewNotFoundError("batch job", jobName, nil)
		}
		return apierrors.NewFromError(err)
	}
	if radixBatchStatus.BatchType != string(kube.RadixBatchTypeJob) {
		return apierrors.NewInvalidWithReason(jobName, "not a single job")
	}
	if !jobExistInBatch(radixBatchStatus, jobName) {
		return apierrors.NewNotFoundError("batch job", jobName, nil)
	}
	err = batch.DeleteRadixBatchByName(ctx, h.Kube.RadixClient(), h.Config.RadixDeploymentNamespace, batchName)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return apierrors.NewNotFoundError("batch job", jobName, nil)
		}
		return apierrors.NewFromError(err)
	}
	return internal.GarbageCollectPayloadSecrets(ctx, h.Kube, h.Config.RadixDeploymentNamespace, h.Config.RadixComponentName)
}

// StopJob Stop a job
func (h *jobHandler) StopJob(ctx context.Context, jobName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("stop the job %s for namespace: %s", jobName, h.Config.RadixDeploymentNamespace)
	return apiv1.StopJob(ctx, h.HandlerApiV2, jobName)
}

func getSingleJobStatusFromRadixBatchJob(radixBatch *modelsv2.Batch) (*modelsv1.JobStatus, error) {
	if len(radixBatch.JobStatuses) != 1 {
		return nil, fmt.Errorf("batch should have only one job")
	}
	radixBatchJobStatus := radixBatch.JobStatuses[0]

	jobStatus := modelsv1.JobStatus{
		JobId:       radixBatchJobStatus.JobId,
		Name:        radixBatchJobStatus.Name,
		Created:     radixBatchJobStatus.CreationTime,
		Started:     radixBatchJobStatus.Started,
		Ended:       radixBatchJobStatus.Ended,
		Status:      modelsv1.JobStatusEnum(radixBatchJobStatus.Status),
		Message:     radixBatchJobStatus.Message,
		Failed:      radixBatchJobStatus.Failed,
		Restart:     radixBatchJobStatus.Restart,
		PodStatuses: apiv1.GetPodStatus(radixBatchJobStatus.PodStatuses),
	}
	return &jobStatus, nil
}

func jobExistInBatch(radixBatch *modelsv2.Batch, jobName string) bool {
	for _, jobStatus := range radixBatch.JobStatuses {
		if jobStatus.Name == jobName {
			return true
		}
	}
	return false
}
