package jobs

import (
	"context"
	"fmt"

	apiv1 "github.com/equinor/radix-job-scheduler/api/v1"
	apiInternal "github.com/equinor/radix-job-scheduler/api/v1/internal"
	"github.com/equinor/radix-job-scheduler/internal"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-job-scheduler/pkg/batch"
	apiErrors "github.com/equinor/radix-job-scheduler/pkg/errors"
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

	combinedBatchStatuses, err := handler.getCombinedBatchStatuses(ctx)
	if err != nil {
		return nil, err
	}
	jobStatuses, err := handler.getJobStatusesWithEvents(ctx, combinedBatchStatuses)
	if err != nil {
		return nil, err
	}
	logger.Debug().Msgf("Found %v jobs for namespace %s", len(jobStatuses), handler.common.GetEnv().RadixDeploymentNamespace)
	return jobStatuses, nil
}

func (handler *jobHandler) getCombinedBatchStatuses(ctx context.Context) ([]modelsv1.BatchStatus, error) {
	singleJobBatchStatuses, err := handler.common.GetRadixBatchStatusSingleJobs(ctx)
	if err != nil {
		return nil, err
	}
	batchStatuses, err := handler.common.GetRadixBatchStatuses(ctx)
	if err != nil {
		return nil, err
	}
	combinedBatchStatuses := make([]modelsv1.BatchStatus, 0, len(singleJobBatchStatuses)+len(batchStatuses))
	combinedBatchStatuses = append(combinedBatchStatuses, singleJobBatchStatuses...)
	combinedBatchStatuses = append(combinedBatchStatuses, batchStatuses...)
	return combinedBatchStatuses, nil
}

func (handler *jobHandler) getJobStatusesWithEvents(ctx context.Context, combinedBatchStatuses []modelsv1.BatchStatus) ([]modelsv1.JobStatus, error) {
	labelSelectorForAllRadixBatchesPods := apiv1.GetLabelSelectorForAllRadixBatchesPods(handler.common.GetEnv().RadixComponentName)
	eventMessageForPods, batchJobPodsMap, err := handler.common.GetRadixBatchJobMessagesAndPodMaps(ctx, labelSelectorForAllRadixBatchesPods)
	if err != nil {
		return nil, err
	}
	var jobStatuses []modelsv1.JobStatus
	for i := 0; i < len(combinedBatchStatuses); i++ {
		for j := 0; j < len(combinedBatchStatuses[i].JobStatuses); j++ {
			jobStatus := combinedBatchStatuses[i].JobStatuses[j]
			apiInternal.SetBatchJobEventMessageToBatchJobStatus(&jobStatus, batchJobPodsMap, eventMessageForPods)
			jobStatuses = append(jobStatuses, jobStatus)
		}
	}
	return jobStatuses, nil
}

// GetJob Get status of a job
func (handler *jobHandler) GetJob(ctx context.Context, jobName string) (*modelsv1.JobStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("get job %s for namespace: %s", jobName, handler.common.GetEnv().RadixDeploymentNamespace)
	if batchName, _, ok := internal.ParseBatchAndJobNameFromScheduledJobName(jobName); ok {
		jobStatus, err := apiInternal.GetBatchJob(ctx, handler.common, batchName, jobName)
		if err != nil {
			return nil, err
		}
		labelSelectorForRadixBatchesPods := apiv1.GetLabelSelectorForRadixBatchesPods(handler.common.GetEnv().RadixComponentName, batchName)
		eventMessageForPods, batchJobPodsMap, err := handler.common.GetRadixBatchJobMessagesAndPodMaps(ctx, labelSelectorForRadixBatchesPods)
		if err != nil {
			return nil, err
		}
		apiInternal.SetBatchJobEventMessageToBatchJobStatus(jobStatus, batchJobPodsMap, eventMessageForPods)
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
	radixBatch, err := handler.common.CopyRadixBatchJob(ctx, jobName, deploymentName)
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

func jobExistInBatch(radixBatch *modelsv1.BatchStatus, jobName string) bool {
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
	return apiInternal.StopJob(ctx, handler.common, jobName)
}

// StopAllJobs Stop all jobs
func (handler *jobHandler) StopAllJobs(ctx context.Context) error {
	return handler.common.StopAllSingleRadixJobs(ctx)
}

func getSingleJobStatusFromRadixBatchJob(radixBatch *modelsv1.BatchStatus) (*modelsv1.JobStatus, error) {
	if len(radixBatch.JobStatuses) != 1 {
		return nil, fmt.Errorf("batch should have only one job")
	}
	return &radixBatch.JobStatuses[0], nil
}
