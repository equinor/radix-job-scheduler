package jobs

import (
	"context"
	"fmt"
	apiErrors "github.com/equinor/radix-job-scheduler/pkg/errors"

	apiv1 "github.com/equinor/radix-job-scheduler/api/v1"
	"github.com/equinor/radix-job-scheduler/internal"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
)

type jobHandler struct {
	common apiv1.Handler
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
	// StopAllJobs Stop all jobs
	StopAllJobs(ctx context.Context) error
}

// New Constructor for job handler
func New(kube *kube.Kube, env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) JobHandler {
	return &jobHandler{
		common: apiv1.New(kube, env, radixDeployJobComponent),
	}
}

// GetJobs Get status of all jobs
func (handler *jobHandler) GetJobs(ctx context.Context) ([]modelsv1.JobStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Get Jobs for namespace: %s", handler.common.GetEnv().RadixDeploymentNamespace)

	singleJobBatchStatuses, err := handler.common.GetRadixBatchStatusSingleJobs(ctx)
	if err != nil {
		return nil, err
	}
	batchStatuses, err := handler.common.GetRadixBatchStatuses(ctx)
	if err != nil {
		return nil, err
	}
	jobStatuses := make([]modelsv1.BatchStatus, 0, len(singleJobBatchStatuses)+len(batchStatuses))
	jobStatuses = append(jobStatuses, singleJobBatchStatuses...)
	jobStatuses = append(jobStatuses, batchStatuses...)

	labelSelectorForAllRadixBatchesPods := apiv1.GetLabelSelectorForAllRadixBatchesPods(handler.common.GetEnv().RadixComponentName)
	eventMessageForPods, batchJobPodsMap, err := handler.common.GetRadixBatchJobMessagesAndPodMaps(ctx, labelSelectorForAllRadixBatchesPods)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(jobStatuses); i++ {
		apiv1.SetBatchJobEventMessageToBatchJobStatus(&jobStatuses[i], batchJobPodsMap, eventMessageForPods)
	}

	logger.Debug().Msgf("Found %v jobs for namespace %s", len(jobStatuses), handler.common.GetEnv().RadixDeploymentNamespace)
	return jobStatuses, nil
}

// GetJob Get status of a job
func (handler *jobHandler) GetJob(ctx context.Context, jobName string) (*modelsv1.JobStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("get job %s for namespace: %s", jobName, handler.common.GetEnv().RadixDeploymentNamespace)
	if batchName, _, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobName); ok {
		jobStatus, err := apiv1.GetBatchJob(ctx, handler.common, batchName, jobName)
		if err != nil {
			return nil, err
		}
		labelSelectorForRadixBatchesPods := apiv1.GetLabelSelectorForRadixBatchesPods(handler.common.GetEnv().RadixComponentName, batchName)
		eventMessageForPods, batchJobPodsMap, err := handler.common.GetRadixBatchJobMessagesAndPodMaps(ctx, labelSelectorForRadixBatchesPods)
		if err != nil {
			return nil, err
		}
		apiv1.SetBatchJobEventMessageToBatchJobStatus(jobStatus, batchJobPodsMap, eventMessageForPods)
		return jobStatus, nil
	}
	return nil, fmt.Errorf("job %s is not a valid job name", jobName)
}

// CreateJob Create a job with parameters
func (handler *jobHandler) CreateJob(ctx context.Context, jobScheduleDescription *common.JobScheduleDescription) (*modelsv1.JobStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Create job for namespace: %s", handler.common.GetEnv().RadixDeploymentNamespace)
	radixBatch, err := handler.common.CreateRadixBatchSingleJob(ctx, jobScheduleDescription)
	if err != nil {
		return nil, err
	}
	return getSingleJobStatusFromRadixBatchJob(radixBatch)
}

// CopyJob Copy a job with  deployment and optional parameters
func (handler *jobHandler) CopyJob(ctx context.Context, jobName string, deploymentName string) (*modelsv1.JobStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("stop the job %s for namespace: %s", jobName, handler.common.GetEnv().RadixDeploymentNamespace)
	radixBatch, err := apiv1.CopyJob(ctx, handler.common, jobName, deploymentName)
	if err != nil {
		return nil, err
	}
	return getSingleJobStatusFromRadixBatchJob(radixBatch)
}

// DeleteJob Delete a job
func (handler *jobHandler) DeleteJob(ctx context.Context, jobName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("delete job %s for namespace: %s", jobName, handler.common.GetEnv().RadixDeploymentNamespace)
	batchName, _, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return apiErrors.NewInvalidWithReason(jobName, "is not a valid job name")
	}
	radixBatchStatus, err := handler.common.GetRadixBatchStatus(ctx, batchName)
	if err != nil {
		if errors.IsNotFound(err) {
			return apiErrors.NewNotFound("batch job", jobName)
		}
		return apiErrors.NewFromError(err)
	}
	if radixBatchStatus.BatchType != string(kube.RadixBatchTypeJob) {
		return apiErrors.NewInvalidWithReason(jobName, "not a single job")
	}
	if !jobExistInBatch(radixBatchStatus, jobName) {
		return apiErrors.NewNotFound("batch job", jobName)
	}
	err = batch.DeleteRadixBatchByName(ctx, handler.common.GetKubeUtil().RadixClient(), handler.common.GetEnv().RadixDeploymentNamespace, batchName)
	if err != nil {
		if errors.IsNotFound(err) {
			return apiErrors.NewNotFound("batch job", jobName)
		}
		return apiErrors.NewFromError(err)
	}
	return internal.GarbageCollectPayloadSecrets(ctx, handler.common.GetKubeUtil(), handler.common.GetEnv().RadixDeploymentNamespace, handler.common.GetEnv().RadixComponentName)
}

func jobExistInBatch(radixBatch *modelsv1.RadixBatchStatus, jobName string) bool {
	for _, jobStatus := range radixBatch.JobStatuses {
		if jobStatus.Name == jobName {
			return true
		}
	}
	return false
}

// StopJob Stop a job
func (handler *jobHandler) StopJob(ctx context.Context, jobName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("stop the job %s for namespace: %s", jobName, handler.common.GetEnv().RadixDeploymentNamespace)
	return apiv1.StopJob(ctx, handler.common, jobName)
}

// StopAllJobs Stop all jobs
func (handler *jobHandler) StopAllJobs(ctx context.Context) error {
	return apiv1.StopAllSingleJobs(ctx, handler.common, handler.common.GetEnv().RadixComponentName)
}

func getSingleJobStatusFromRadixBatchJob(radixBatch *modelsv1.RadixBatchStatus) (*modelsv1.JobStatus, error) {
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
		Status:      string(radixBatchJobStatus.Status),
		Message:     radixBatchJobStatus.Message,
		Failed:      radixBatchJobStatus.Failed,
		Restart:     radixBatchJobStatus.Restart,
		PodStatuses: batch.GetPodStatus(radixBatchJobStatus.PodStatuses),
	}
	return &jobStatus, nil
}
