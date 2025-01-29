package batchesv1

import (
	"context"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	apiv1 "github.com/equinor/radix-job-scheduler/api/v1"
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"github.com/equinor/radix-job-scheduler/internal"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
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
	CreateBatch(ctx context.Context, batchScheduleDescription *common.BatchScheduleDescription) (*modelsv1.BatchStatus, error)
	// CopyBatch creates a copy of an existing batch with deploymentName as value for radixDeploymentJobRef.name
	CopyBatch(ctx context.Context, batchName string, deploymentName string) (*modelsv1.BatchStatus, error)
	// DeleteBatch Delete a batch
	DeleteBatch(ctx context.Context, batchName string) error
	// StopBatch Stop a batch
	StopBatch(ctx context.Context, batchName string) error
	// StopBatchJob Stop a batch job
	StopBatchJob(ctx context.Context, batchName string, jobName string) error
}

// New Constructor of the batch handler
func New(kube *kube.Kube, config *models.Config, radixDeployJobComponent *radixv1.RadixDeployJobComponent) BatchHandler {
	return &batchHandler{
		apiv1.Handler{
			Kube:                    kube,
			Config:                  config,
			HandlerApiV2:            apiv2.New(kube, config, radixDeployJobComponent),
			RadixDeployJobComponent: radixDeployJobComponent,
		},
	}
}

// GetBatches Get status of all batches
func (h *batchHandler) GetBatches(ctx context.Context) ([]modelsv1.BatchStatus, error) {
	// rbList, err := h.ListRadixBatches(ctx, kube.RadixBatchTypeBatch)
	// if err != nil {
	// 	return nil, err
	// }

	// batchStatuses := slice.Map(rbList, modelsv1.BuildBatchStatus)
	// return batchStatuses, nil
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Get batches for the namespace: %s", h.Config.RadixDeploymentNamespace)

	radixBatches, err := h.HandlerApiV2.GetRadixBatches(ctx)
	if err != nil {
		return nil, err
	}
	radixBatchStatuses := make([]modelsv1.BatchStatus, 0, len(radixBatches))
	if len(radixBatches) == 0 {
		logger.Debug().Msgf("No batches found for namespace %s", h.Config.RadixDeploymentNamespace)
		return radixBatchStatuses, nil
	}

	labelSelectorForAllRadixBatchesPods := apiv1.GetLabelSelectorForAllRadixBatchesPods(h.Config.RadixComponentName)
	eventMessageForPods, batchJobPodsMap, err := h.GetRadixBatchJobMessagesAndPodMaps(ctx, labelSelectorForAllRadixBatchesPods)
	if err != nil {
		return nil, err
	}
	for _, radixBatch := range radixBatches {
		radixBatchStatus := h.getBatchStatusFromRadixBatch(&radixBatch)
		setBatchJobEventMessages(radixBatchStatus, batchJobPodsMap, eventMessageForPods)
		radixBatchStatuses = append(radixBatchStatuses, *radixBatchStatus)
	}
	logger.Debug().Msgf("Found %v batches for namespace %s", len(radixBatchStatuses), h.Config.RadixDeploymentNamespace)
	return radixBatchStatuses, nil
}

// GetBatchJob Get status of a batch job
func (h *batchHandler) GetBatchJob(ctx context.Context, batchName string, jobName string) (*modelsv1.JobStatus, error) {
	return apiv1.GetBatchJob(ctx, h.HandlerApiV2, batchName, jobName)
}

// GetBatch Get status of a batch
func (h *batchHandler) GetBatch(ctx context.Context, batchName string) (*modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("get batches for namespace: %s", h.Config.RadixDeploymentNamespace)
	radixBatch, err := h.HandlerApiV2.GetRadixBatch(ctx, batchName)
	if err != nil {
		return nil, err
	}
	radixBatchStatus := h.getBatchStatusFromRadixBatch(radixBatch)
	labelSelectorForRadixBatchesPods := apiv1.GetLabelSelectorForRadixBatchesPods(h.Config.RadixComponentName, batchName)
	eventMessageForPods, batchJobPodsMap, err := h.GetRadixBatchJobMessagesAndPodMaps(ctx, labelSelectorForRadixBatchesPods)
	if err != nil {
		return nil, err
	}
	setBatchJobEventMessages(radixBatchStatus, batchJobPodsMap, eventMessageForPods)
	return radixBatchStatus, nil
}

// CreateBatch Create a batch with parameters
func (h *batchHandler) CreateBatch(ctx context.Context, batchScheduleDescription *common.BatchScheduleDescription) (*modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("create batch for namespace: %s", h.Config.RadixDeploymentNamespace)
	radixBatch, err := h.HandlerApiV2.CreateRadixBatch(ctx, batchScheduleDescription)
	if err != nil {
		return nil, err
	}
	return h.getBatchStatusFromRadixBatch(radixBatch), nil
}

// CopyBatch Copy a batch with  deployment and optional parameters
func (h *batchHandler) CopyBatch(ctx context.Context, batchName string, deploymentName string) (*modelsv1.BatchStatus, error) {
	radixBatch, err := h.HandlerApiV2.CopyRadixBatch(ctx, batchName, deploymentName)
	if err != nil {
		return nil, err
	}
	return h.getBatchStatusFromRadixBatch(radixBatch), nil
}

// DeleteBatch Delete a batch
func (h *batchHandler) DeleteBatch(ctx context.Context, batchName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("delete batch %s for namespace: %s", batchName, h.Config.RadixDeploymentNamespace)
	err := batch.DeleteRadixBatchByName(ctx, h.Kube.RadixClient(), h.Config.RadixDeploymentNamespace, batchName)
	if err != nil {
		return err
	}
	return internal.GarbageCollectPayloadSecrets(ctx, h.Kube, h.Config.RadixDeploymentNamespace, h.Config.RadixComponentName)
}

// StopBatch Stop a batch
func (h *batchHandler) StopBatch(ctx context.Context, batchName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("delete batch %s for namespace: %s", batchName, h.Config.RadixDeploymentNamespace)
	return h.HandlerApiV2.StopRadixBatch(ctx, batchName)
}

// StopBatchJob Stop a batch job
func (h *batchHandler) StopBatchJob(ctx context.Context, batchName string, jobName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("delete the job %s in the batch %s for namespace: %s", jobName, batchName, h.Config.RadixDeploymentNamespace)
	return apiv1.StopJob(ctx, h.HandlerApiV2, jobName)
}

func (h *batchHandler) getBatchStatusFromRadixBatch(radixBatch *modelsv2.Batch) *modelsv1.BatchStatus {
	return &modelsv1.BatchStatus{
		Name:           radixBatch.Name,
		BatchId:        getBatchId(radixBatch),
		Created:        radixBatch.CreationTime,
		Started:        radixBatch.Started,
		Ended:          radixBatch.Ended,
		Message:        radixBatch.Message,
		DeploymentName: radixBatch.DeploymentName,
		Status:         modelsv1.BatchStatusEnum(h.getBatchStatus(radixBatch)),
		JobStatuses:    apiv1.GetJobStatusFromRadixBatchJobsStatuses(*radixBatch),
		BatchType:      radixBatch.BatchType,
	}
}

func (h *batchHandler) getBatchStatus(radixBatch *modelsv2.Batch) radixv1.RadixBatchJobApiStatus {
	isSingleJob := radixBatch.BatchType == string(kube.RadixBatchTypeJob)
	if isSingleJob {
		if len(radixBatch.JobStatuses) == 1 {
			return radixBatch.JobStatuses[0].Status
		}
		return radixBatch.Status
	}

	jobStatusPhases := slice.Reduce(radixBatch.JobStatuses, make([]radixv1.RadixBatchJobPhase, 0), func(acc []radixv1.RadixBatchJobPhase, jobStatus modelsv2.Job) []radixv1.RadixBatchJobPhase {
		return append(acc, radixv1.RadixBatchJobPhase(jobStatus.Status))
	})
	return internal.GetStatusFromStatusRules(jobStatusPhases, h.RadixDeployJobComponent, radixBatch.Status)
}

func getBatchId(radixBatch *modelsv2.Batch) string {
	return utils.TernaryString(radixBatch.BatchType == string(kube.RadixBatchTypeJob), "", radixBatch.BatchId)
}

func setBatchJobEventMessages(radixBatchStatus *modelsv1.BatchStatus, batchJobPodsMap map[string]corev1.Pod, eventMessageForPods map[string]string) {
	for i := 0; i < len(radixBatchStatus.JobStatuses); i++ {
		apiv1.SetBatchJobEventMessageToBatchJobStatus(&radixBatchStatus.JobStatuses[i], batchJobPodsMap, eventMessageForPods)
	}
}
