package batchesv1

import (
	"context"

	"github.com/equinor/radix-job-scheduler/api/v1"
	apiInternal "github.com/equinor/radix-job-scheduler/api/v1/internal"
	"github.com/equinor/radix-job-scheduler/internal"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
)

type batchHandler struct {
	common v1.Handler
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
	StopBatchJob(ctx context.Context, batchName, jobName string) error
	// StopAllBatches Stop all batches
	StopAllBatches(ctx context.Context) error
}

// New Constructor of the batch handler
func New(kube *kube.Kube, env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) BatchHandler {
	return &batchHandler{
		common: v1.New(kube, env, radixDeployJobComponent),
	}
}

// GetBatches Get status of all batches
func (handler *batchHandler) GetBatches(ctx context.Context) ([]modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Get batches for the namespace: %s", handler.common.GetEnv().RadixDeploymentNamespace)

	batchesStatuses, err := handler.common.GetRadixBatchStatuses(ctx)
	if err != nil {
		return nil, err
	}
	if len(batchesStatuses) == 0 {
		logger.Debug().Msgf("No batches found for namespace %s", handler.common.GetEnv().RadixDeploymentNamespace)
		return nil, nil
	}

	labelSelectorForAllRadixBatchesPods := v1.GetLabelSelectorForAllRadixBatchesPods(handler.common.GetEnv().RadixComponentName)
	eventMessageForPods, batchJobPodsMap, err := handler.common.GetRadixBatchJobMessagesAndPodMaps(ctx, labelSelectorForAllRadixBatchesPods)
	if err != nil {
		return nil, err
	}
	for _, radixBatchStatus := range batchesStatuses {
		setBatchJobEventMessages(&radixBatchStatus, batchJobPodsMap, eventMessageForPods)
	}
	logger.Debug().Msgf("Found %v batches for namespace %s", len(batchesStatuses), handler.common.GetEnv().RadixDeploymentNamespace)
	return batchesStatuses, nil
}

// GetBatchJob Get status of a batch job
func (handler *batchHandler) GetBatchJob(ctx context.Context, batchName string, jobName string) (*modelsv1.JobStatus, error) {
	return apiInternal.GetBatchJob(ctx, handler.common, batchName, jobName)
}

// GetBatch Get status of a batch
func (handler *batchHandler) GetBatch(ctx context.Context, batchName string) (*modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("get batches for namespace: %s", handler.common.GetEnv().RadixDeploymentNamespace)
	batchStatus, err := handler.common.GetRadixBatchStatus(ctx, batchName)
	if err != nil {
		return nil, err
	}
	labelSelectorForRadixBatchesPods := v1.GetLabelSelectorForRadixBatchesPods(handler.common.GetEnv().RadixComponentName, batchName)
	eventMessageForPods, batchJobPodsMap, err := handler.common.GetRadixBatchJobMessagesAndPodMaps(ctx, labelSelectorForRadixBatchesPods)
	if err != nil {
		return nil, err
	}
	setBatchJobEventMessages(batchStatus, batchJobPodsMap, eventMessageForPods)
	return batchStatus, nil
}

// CreateBatch Create a batch with parameters
func (handler *batchHandler) CreateBatch(ctx context.Context, batchScheduleDescription *common.BatchScheduleDescription) (*modelsv1.BatchStatus, error) {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("create batch for namespace: %s", handler.common.GetEnv().RadixDeploymentNamespace)
	return handler.common.CreateRadixBatch(ctx, batchScheduleDescription)
}

// CopyBatch Copy a batch with  deployment and optional parameters
func (handler *batchHandler) CopyBatch(ctx context.Context, batchName string, deploymentName string) (*modelsv1.BatchStatus, error) {
	return handler.common.CopyRadixBatch(ctx, batchName, deploymentName)
}

// DeleteBatch Delete a batch
func (handler *batchHandler) DeleteBatch(ctx context.Context, batchName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("delete batch %s for namespace: %s", batchName, handler.common.GetEnv().RadixDeploymentNamespace)
	err := batch.DeleteRadixBatchByName(ctx, handler.common.GetKubeUtil().RadixClient(), handler.common.GetEnv().RadixDeploymentNamespace, batchName)
	if err != nil {
		return err
	}
	return internal.GarbageCollectPayloadSecrets(ctx, handler.common.GetKubeUtil(), handler.common.GetEnv().RadixDeploymentNamespace, handler.common.GetEnv().RadixComponentName)
}

// StopBatch Stop a batch
func (handler *batchHandler) StopBatch(ctx context.Context, batchName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("delete batch %s for namespace: %s", batchName, handler.common.GetEnv().RadixDeploymentNamespace)
	return handler.common.StopRadixBatch(ctx, batchName)
}

// StopAllBatches Stop all batches
func (handler *batchHandler) StopAllBatches(ctx context.Context) error {
	return handler.common.StopAllRadixBatches(ctx)
}

// StopBatchJob Stop a batch job
func (handler *batchHandler) StopBatchJob(ctx context.Context, batchName string, jobName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("delete the job %s in the batch %s for namespace: %s", jobName, batchName, handler.common.GetEnv().RadixDeploymentNamespace)
	return apiInternal.StopJob(ctx, handler.common, jobName)
}

func setBatchJobEventMessages(radixBatchStatus *modelsv1.BatchStatus, batchJobPodsMap map[string]corev1.Pod, eventMessageForPods map[string]string) {
	for i := 0; i < len(radixBatchStatus.JobStatuses); i++ {
		apiInternal.SetBatchJobEventMessageToBatchJobStatus(&radixBatchStatus.JobStatuses[i], batchJobPodsMap, eventMessageForPods)
	}
}
