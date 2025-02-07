package history

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-job-scheduler/internal"
	"github.com/equinor/radix-job-scheduler/internal/config"
	"github.com/equinor/radix-job-scheduler/internal/query"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// completedRadixBatches Completed RadixBatch lists
type completedRadixBatches struct {
	SucceededRadixBatches    []*modelsv2.Batch
	NotSucceededRadixBatches []*modelsv2.Batch
	SucceededSingleJobs      []*modelsv2.Batch
	NotSucceededSingleJobs   []*modelsv2.Batch
}

// History Interface for job History
type History interface {
	// Cleanup the pipeline job history for the Radix application
	Cleanup(ctx context.Context) error
}

type history struct {
	kubeUtil                *kube.Kube
	cfg                     *config.Config
	radixDeployJobComponent *radixv1.RadixDeployJobComponent
}

// NewHistory Constructor for job History
func NewHistory(kubeUtil *kube.Kube, cfg *config.Config, radixDeployJobComponent *radixv1.RadixDeployJobComponent) History {
	return &history{
		kubeUtil:                kubeUtil,
		cfg:                     cfg,
		radixDeployJobComponent: radixDeployJobComponent,
	}
}

// Cleanup the pipeline job history
func (h *history) Cleanup(ctx context.Context) error {
	logger := log.Ctx(ctx)
	const minimumAge = 3600 // TODO add as default env-var and/or job-component property
	completedBefore := time.Now().Add(-time.Second * minimumAge)
	completedRadixBatches, err := h.getCompletedRadixBatchesSortedByCompletionTimeAsc(ctx, completedBefore)
	if err != nil {
		return err
	}

	logger.Debug().Msg("cleanup RadixBatch history for succeeded batches")
	var errs []error
	historyLimit := h.cfg.RadixJobSchedulersPerEnvironmentHistoryLimit
	if err := h.cleanupRadixBatchHistory(ctx, completedRadixBatches.SucceededRadixBatches, historyLimit); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("cleanup RadixBatch history for not succeeded batches")
	if err := h.cleanupRadixBatchHistory(ctx, completedRadixBatches.NotSucceededRadixBatches, historyLimit); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("cleanup RadixBatch history for succeeded single jobs")
	if err := h.cleanupRadixBatchHistory(ctx, completedRadixBatches.SucceededSingleJobs, historyLimit); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("cleanup RadixBatch history for not succeeded single jobs")
	if err := h.cleanupRadixBatchHistory(ctx, completedRadixBatches.NotSucceededSingleJobs, historyLimit); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("delete orphaned payload secrets")
	if err = garbageCollectPayloadSecrets(ctx, h.kubeUtil, h.cfg.RadixDeploymentNamespace, h.cfg.RadixComponentName); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (h *history) getCompletedRadixBatchesSortedByCompletionTimeAsc(ctx context.Context, completedBefore time.Time) (*completedRadixBatches, error) {
	radixBatches, err := query.ListRadixBatches(ctx, h.cfg.RadixDeploymentNamespace, h.kubeUtil.RadixClient(), radixlabels.ForComponentName(h.cfg.RadixComponentName).AsSelector())
	if err != nil {
		return nil, err
	}
	radixBatches = sortRJSchByCompletionTimeAsc(radixBatches)
	return &completedRadixBatches{
		SucceededRadixBatches:    h.getSucceededRadixBatches(radixBatches, completedBefore),
		NotSucceededRadixBatches: h.getNotSucceededRadixBatches(radixBatches, completedBefore),
		SucceededSingleJobs:      h.getSucceededSingleJobs(radixBatches, completedBefore),
		NotSucceededSingleJobs:   h.getNotSucceededSingleJobs(radixBatches, completedBefore),
	}, nil
}

func (h *history) getNotSucceededRadixBatches(radixBatches []radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.Batch {
	return convertToRadixBatchStatuses(slice.FindAll(radixBatches, func(radixBatch radixv1.RadixBatch) bool {
		return radixBatchHasType(&radixBatch, kube.RadixBatchTypeBatch) && internal.IsRadixBatchNotSucceeded(&radixBatch) && radixBatchIsCompletedBefore(completedBefore, &radixBatch)
	}), h.radixDeployJobComponent)
}

func (h *history) getSucceededRadixBatches(radixBatches []radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.Batch {
	radixBatches = slice.FindAll(radixBatches, func(radixBatch radixv1.RadixBatch) bool {
		return radixBatchHasType(&radixBatch, kube.RadixBatchTypeBatch) && internal.IsRadixBatchSucceeded(&radixBatch) && radixBatchIsCompletedBefore(completedBefore, &radixBatch)
	})
	return convertToRadixBatchStatuses(radixBatches, h.radixDeployJobComponent)
}

func radixBatchIsCompletedBefore(completedBefore time.Time, radixBatch *radixv1.RadixBatch) bool {
	return radixBatch.Status.Condition.CompletionTime != nil && (*radixBatch.Status.Condition.CompletionTime).Before(&metav1.Time{Time: completedBefore})
}

func (h *history) getNotSucceededSingleJobs(radixBatches []radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.Batch {
	return convertToRadixBatchStatuses(slice.FindAll(radixBatches, func(radixBatch radixv1.RadixBatch) bool {
		return radixBatchHasType(&radixBatch, kube.RadixBatchTypeJob) && internal.IsRadixBatchNotSucceeded(&radixBatch) && radixBatchIsCompletedBefore(completedBefore, &radixBatch)
	}), h.radixDeployJobComponent)
}

func (h *history) getSucceededSingleJobs(radixBatches []radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.Batch {
	return convertToRadixBatchStatuses(slice.FindAll(radixBatches, func(radixBatch radixv1.RadixBatch) bool {
		return radixBatchHasType(&radixBatch, kube.RadixBatchTypeJob) && internal.IsRadixBatchSucceeded(&radixBatch) && radixBatchIsCompletedBefore(completedBefore, &radixBatch)
	}), h.radixDeployJobComponent)
}

func radixBatchHasType(radixBatch *radixv1.RadixBatch, radixBatchType kube.RadixBatchType) bool {
	return radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(radixBatchType)
}

func (h *history) cleanupRadixBatchHistory(ctx context.Context, radixBatchesSortedByCompletionTimeAsc []*modelsv2.Batch, historyLimit int) error {
	logger := log.Ctx(ctx)
	numToDelete := len(radixBatchesSortedByCompletionTimeAsc) - historyLimit
	if numToDelete <= 0 {
		logger.Debug().Msgf("no history batches to delete: %d batches, %d history limit", len(radixBatchesSortedByCompletionTimeAsc), historyLimit)
		return nil
	}
	logger.Debug().Msgf("history batches to delete: %v", numToDelete)

	for i := 0; i < numToDelete; i++ {
		radixBatch := radixBatchesSortedByCompletionTimeAsc[i]
		logger.Debug().Msgf("deleting batch %s", radixBatch.Name)
		if err := h.kubeUtil.RadixClient().RadixV1().RadixBatches(h.cfg.RadixDeploymentNamespace).Delete(ctx, radixBatch.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func sortRJSchByCompletionTimeAsc(batches []radixv1.RadixBatch) []radixv1.RadixBatch {
	sort.Slice(batches, func(i, j int) bool {
		batch1 := &(batches)[i]
		batch2 := &(batches)[j]
		return isRJS1CompletedBeforeRJS2(batch1, batch2)
	})
	return batches
}

func isRJS1CompletedBeforeRJS2(batch1 *radixv1.RadixBatch, batch2 *radixv1.RadixBatch) bool {
	rd1ActiveFrom := getCompletionTimeFrom(batch1)
	rd2ActiveFrom := getCompletionTimeFrom(batch2)

	return rd1ActiveFrom.Before(rd2ActiveFrom)
}

func getCompletionTimeFrom(radixBatch *radixv1.RadixBatch) *metav1.Time {
	if radixBatch.Status.Condition.CompletionTime.IsZero() {
		return pointers.Ptr(radixBatch.GetCreationTimestamp())
	}
	return radixBatch.Status.Condition.CompletionTime
}

func convertToRadixBatchStatuses(radixBatches []radixv1.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) []*modelsv2.Batch {
	batches := make([]*modelsv2.Batch, 0, len(radixBatches))
	for _, radixBatch := range radixBatches {
		batches = append(batches, pointers.Ptr(batch.GetRadixBatchStatus(&radixBatch, radixDeployJobComponent)))
	}
	return batches
}

// GetJobComponentPayloadSecretRefNames Get the payload secret ref names for the job components
func getJobComponentPayloadSecretRefNames(ctx context.Context, radixClient radixclient.Interface, namespace, radixComponentName string) (map[string]bool, error) {
	radixBatches, err := query.ListRadixBatches(ctx, namespace, radixClient, radixlabels.ForComponentName(radixComponentName).AsSelector())
	if err != nil {
		return nil, err
	}
	payloadSecretRefNames := make(map[string]bool)
	for _, radixBatch := range radixBatches {
		for _, job := range radixBatch.Spec.Jobs {
			if job.PayloadSecretRef != nil {
				payloadSecretRefNames[job.PayloadSecretRef.Name] = true
			}
		}
	}
	return payloadSecretRefNames, nil
}

// GarbageCollectPayloadSecrets Delete orphaned payload secrets
func garbageCollectPayloadSecrets(ctx context.Context, kubeUtil *kube.Kube, namespace, radixComponentName string) error {
	logger := log.Ctx(ctx)
	logger.Debug().Msgf("Garbage collecting payload secrets")
	payloadSecretRefNames, err := getJobComponentPayloadSecretRefNames(ctx, kubeUtil.RadixClient(), namespace, radixComponentName)
	if err != nil {
		return err
	}
	payloadSecrets, err := kubeUtil.ListSecretsWithSelector(ctx, namespace, radixlabels.GetRadixBatchDescendantsSelector(radixComponentName).String())
	if err != nil {
		return err
	}
	logger.Debug().Msgf("%d payload secrets, %d secret reference unique names", len(payloadSecrets), len(payloadSecretRefNames))
	yesterday := time.Now().Add(time.Hour * -24)
	for _, payloadSecret := range payloadSecrets {
		if _, ok := payloadSecretRefNames[payloadSecret.GetName()]; !ok {
			if payloadSecret.GetCreationTimestamp().After(yesterday) {
				logger.Debug().Msgf("skipping deletion of an orphaned payload secret %s, created within 24 hours", payloadSecret.GetName())
				continue
			}
			if err := kubeUtil.DeleteSecret(ctx, payloadSecret.GetNamespace(), payloadSecret.GetName()); err != nil && !kubeerrors.IsNotFound(err) {
				logger.Error().Err(err).Msgf("failed deleting of an orphaned payload secret %s", payloadSecret.GetName())
			} else {
				logger.Debug().Msgf("deleted an orphaned payload secret %s", payloadSecret.GetName())
			}
		}
	}
	return nil
}
