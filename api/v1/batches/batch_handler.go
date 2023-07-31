package batchesv1

import (
	"context"
	apiv1 "github.com/equinor/radix-job-scheduler/api/v1"
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type batchHandler struct {
	common *apiv1.Handler
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
	// MaintainHistoryLimit Delete outdated batches
	MaintainHistoryLimit(ctx context.Context) error
	// DeleteBatch Delete a batch
	DeleteBatch(ctx context.Context, batchName string) error
	// StopBatch Stop a batch
	StopBatch(ctx context.Context, batchName string) error
	// StopBatchJob Stop a batch job
	StopBatchJob(ctx context.Context, batchName string, jobName string) error
}

// New Constructor of the batch handler
func New(kube *kube.Kube, env *models.Env) BatchHandler {
	return &batchHandler{
		common: &apiv1.Handler{
			Kube:         kube,
			Env:          env,
			HandlerApiV2: apiv2.New(kube, env),
		},
	}
}

// GetBatches Get status of all batches
func (handler *batchHandler) GetBatches(ctx context.Context) ([]modelsv1.BatchStatus, error) {
	log.Debugf("Get batches for the namespace: %s", handler.common.Env.RadixDeploymentNamespace)

	var allRadixBatchStatuses []modelsv1.BatchStatus
	radixBatches, err := handler.common.HandlerApiV2.GetRadixBatches(ctx)
	if err != nil {
		return nil, err
	}
	if len(radixBatches) == 0 {
		log.Debugf("No batches found for namespace %s", handler.common.Env.RadixDeploymentNamespace)
		return allRadixBatchStatuses, nil
	}

	labelSelectorForAllRadixBatchesPods := apiv1.GetLabelSelectorForAllRadixBatchesPods(handler.common.Env.RadixComponentName)
	eventMessageForPods, batchJobPodsMap, err := handler.common.GetRadixBatchJobMessagesAndPodMaps(ctx, labelSelectorForAllRadixBatchesPods)
	if err != nil {
		return nil, err
	}
	for _, radixBatch := range radixBatches {
		radixBatchStatus := GetBatchStatusFromRadixBatch(&radixBatch)
		setBatchJobEventMessages(radixBatchStatus, batchJobPodsMap, eventMessageForPods)
		allRadixBatchStatuses = append(allRadixBatchStatuses, *radixBatchStatus)
	}
	log.Debugf("Found %v batches for namespace %s", len(allRadixBatchStatuses), handler.common.Env.RadixDeploymentNamespace)
	return allRadixBatchStatuses, nil
}

// GetBatchJob Get status of a batch job
func (handler *batchHandler) GetBatchJob(ctx context.Context, batchName string, jobName string) (*modelsv1.JobStatus, error) {
	return apiv1.GetBatchJob(ctx, handler.common.HandlerApiV2, batchName, jobName)
}

// GetBatch Get status of a batch
func (handler *batchHandler) GetBatch(ctx context.Context, batchName string) (*modelsv1.BatchStatus, error) {
	log.Debugf("get batches for namespace: %s", handler.common.Env.RadixDeploymentNamespace)
	radixBatch, err := handler.common.HandlerApiV2.GetRadixBatch(ctx, batchName)
	if err != nil {
		return nil, err
	}
	radixBatchStatus := GetBatchStatusFromRadixBatch(radixBatch)
	labelSelectorForRadixBatchesPods := apiv1.GetLabelSelectorForRadixBatchesPods(handler.common.Env.RadixComponentName, batchName)
	eventMessageForPods, batchJobPodsMap, err := handler.common.GetRadixBatchJobMessagesAndPodMaps(ctx, labelSelectorForRadixBatchesPods)
	if err != nil {
		return nil, err
	}
	setBatchJobEventMessages(radixBatchStatus, batchJobPodsMap, eventMessageForPods)
	return radixBatchStatus, nil
}

// CreateBatch Create a batch with parameters
func (handler *batchHandler) CreateBatch(ctx context.Context, batchScheduleDescription *common.BatchScheduleDescription) (*modelsv1.BatchStatus, error) {
	radixBatch, err := handler.common.HandlerApiV2.CreateRadixBatch(ctx, batchScheduleDescription)
	if err != nil {
		return nil, err
	}
	return GetBatchStatusFromRadixBatch(radixBatch), nil
}

// CopyBatch Copy a batch with  deployment and optional parameters
func (handler *batchHandler) CopyBatch(ctx context.Context, batchName string, deploymentName string) (*modelsv1.BatchStatus, error) {
	radixBatch, err := handler.common.HandlerApiV2.CopyRadixBatch(ctx, batchName, deploymentName)
	if err != nil {
		return nil, err
	}
	return GetBatchStatusFromRadixBatch(radixBatch), nil
}

// DeleteBatch Delete a batch
func (handler *batchHandler) DeleteBatch(ctx context.Context, batchName string) error {
	log.Debugf("delete batch %s for namespace: %s", batchName, handler.common.Env.RadixDeploymentNamespace)
	err := handler.common.HandlerApiV2.DeleteRadixBatch(ctx, batchName)
	if err != nil {
		return err
	}
	return handler.common.HandlerApiV2.GarbageCollectPayloadSecrets(ctx)
}

// StopBatch Stop a batch
func (handler *batchHandler) StopBatch(ctx context.Context, batchName string) error {
	log.Debugf("delete batch %s for namespace: %s", batchName, handler.common.Env.RadixDeploymentNamespace)
	return handler.common.HandlerApiV2.StopRadixBatch(ctx, batchName)
}

// StopBatchJob Stop a batch job
func (handler *batchHandler) StopBatchJob(ctx context.Context, batchName string, jobName string) error {
	log.Debugf("delete the job %s in the batch %s for namespace: %s", jobName, batchName, handler.common.Env.RadixDeploymentNamespace)
	return apiv1.StopJob(ctx, handler.common.HandlerApiV2, jobName)
}

// MaintainHistoryLimit Delete outdated batches
func (handler *batchHandler) MaintainHistoryLimit(ctx context.Context) error {
	return handler.common.HandlerApiV2.MaintainHistoryLimit(ctx)
}

func setBatchJobEventMessages(radixBatchStatus *modelsv1.BatchStatus, batchJobPodsMap map[string]corev1.Pod, eventMessageForPods map[string]string) {
	for i := 0; i < len(radixBatchStatus.JobStatuses); i++ {
		apiv1.SetBatchJobEventMessageToBatchJobStatus(&radixBatchStatus.JobStatuses[i], batchJobPodsMap, eventMessageForPods)
	}
}
