package jobs

import (
	"context"
	"fmt"
	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	apiv1 "github.com/equinor/radix-job-scheduler/api/v1"
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
)

type jobHandler struct {
	common *apiv1.Handler
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
	// MaintainHistoryLimit Delete outdated jobs
	MaintainHistoryLimit(ctx context.Context) error
	// DeleteJob Delete a job
	DeleteJob(ctx context.Context, jobName string) error
	// StopJob Stop a job
	StopJob(ctx context.Context, jobName string) error
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
	var jobStatuses []modelsv1.JobStatus
	// get all single jobs
	radixBatches, err := handler.common.HandlerApiV2.GetRadixBatchSingleJobs(ctx)
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
	return nil, fmt.Errorf("job %s is not a valid job name", jobName)
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

// CopyJob Copy a job with  deployment and optional parameters
func (handler *jobHandler) CopyJob(ctx context.Context, jobName string, deploymentName string) (*modelsv1.JobStatus, error) {
	log.Debugf("stop the job %s for namespace: %s", jobName, handler.common.Env.RadixDeploymentNamespace)
	radixBatch, err := apiv1.CopyJob(ctx, handler.common.HandlerApiV2, jobName, deploymentName)
	if err != nil {
		return nil, err
	}
	return GetSingleJobStatusFromRadixBatchJob(radixBatch)
}

// DeleteJob Delete a job
func (handler *jobHandler) DeleteJob(ctx context.Context, jobName string) error {
	log.Debugf("delete job %s for namespace: %s", jobName, handler.common.Env.RadixDeploymentNamespace)
	batchName, _, ok := apiv1.ParseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		return apiErrors.NewInvalidWithReason(jobName, "is not a valid job name")
	}
	radixBatchStatus, err := handler.common.HandlerApiV2.GetRadixBatch(ctx, batchName)
	if err != nil {
		if errors.IsNotFound(err) {
			return apiErrors.NewNotFound("batch job", jobName)
		}
		return err
	}
	if radixBatchStatus.BatchType != string(kube.RadixBatchTypeJob) {
		return apiErrors.NewInvalidWithReason(jobName, "not a single job")
	}
	if !jobExistInBatch(radixBatchStatus, jobName) {
		return apiErrors.NewNotFound("batch job", jobName)
	}
	err = handler.common.HandlerApiV2.DeleteRadixBatch(ctx, batchName)
	if err != nil {
		return err
	}
	return handler.common.HandlerApiV2.GarbageCollectPayloadSecrets(ctx)
}

func jobExistInBatch(radixBatch *modelsv2.RadixBatch, jobName string) bool {
	for _, jobStatus := range radixBatch.JobStatuses {
		if jobStatus.Name == jobName {
			return true
		}
	}
	return false
}

// StopJob Stop a job
func (handler *jobHandler) StopJob(ctx context.Context, jobName string) error {
	log.Debugf("stop the job %s for namespace: %s", jobName, handler.common.Env.RadixDeploymentNamespace)
	return apiv1.StopJob(ctx, handler.common.HandlerApiV2, jobName)
}

// MaintainHistoryLimit Delete outdated jobs
func (handler *jobHandler) MaintainHistoryLimit(ctx context.Context) error {
	return handler.common.HandlerApiV2.MaintainHistoryLimit(ctx)
}
