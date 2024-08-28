package apiv2

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-job-scheduler/internal"
	"github.com/equinor/radix-job-scheduler/models"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-job-scheduler/pkg/batch"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// History Interface for job History
type History interface {
	// Cleanup the pipeline job history for the Radix application
	Cleanup(ctx context.Context) error
}

type history struct {
	namespacesRequestsToCleanup sync.Map
	radixClient                 radixclient.Interface
	historyLimit                int
	kubeUtil                    *kube.Kube
	historyPeriodLimit          time.Duration
	namespace                   string
	env                         *models.Env
	radixDeployJobComponent     *radixv1.RadixDeployJobComponent
}

// NewHistory Constructor for job History
func NewHistory(kubeUtil *kube.Kube, env *models.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) History {
	return &history{
		kubeUtil:                kubeUtil,
		env:                     env,
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

	historyLimit := h.historyLimit
	logger.Debug().Msg("maintain history limit for succeeded batches")
	var errs []error
	if err := h.maintainHistoryLimitForBatches(ctx, completedRadixBatches.SucceededRadixBatches, historyLimit); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("maintain history limit for not succeeded batches")
	if err := h.maintainHistoryLimitForBatches(ctx, completedRadixBatches.NotSucceededRadixBatches, historyLimit); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("maintain history limit for succeeded single jobs")
	if err := h.maintainHistoryLimitForBatches(ctx, completedRadixBatches.SucceededSingleJobs, historyLimit); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("maintain history limit for not succeeded single jobs")
	if err := h.maintainHistoryLimitForBatches(ctx, completedRadixBatches.NotSucceededSingleJobs, historyLimit); err != nil {
		errs = append(errs, err)
	}
	logger.Debug().Msg("delete orphaned payload secrets")
	if err = internal.GarbageCollectPayloadSecrets(ctx, h.kubeUtil, h.env.RadixDeploymentNamespace, h.env.RadixComponentName); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (h *history) getCompletedRadixBatchesSortedByCompletionTimeAsc(ctx context.Context, completedBefore time.Time) (*CompletedRadixBatches, error) {
	radixBatches, err := internal.GetRadixBatches(ctx, h.namespace, h.kubeUtil.RadixClient(), radixLabels.ForComponentName(h.env.RadixComponentName))
	if err != nil {
		return nil, err
	}
	radixBatches = sortRJSchByCompletionTimeAsc(radixBatches)
	return &CompletedRadixBatches{
		SucceededRadixBatches:    h.getSucceededRadixBatches(radixBatches, completedBefore),
		NotSucceededRadixBatches: h.getNotSucceededRadixBatches(radixBatches, completedBefore),
		SucceededSingleJobs:      h.getSucceededSingleJobs(radixBatches, completedBefore),
		NotSucceededSingleJobs:   h.getNotSucceededSingleJobs(radixBatches, completedBefore),
	}, nil
}

func (h *history) getNotSucceededRadixBatches(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.RadixBatch {
	return convertToRadixBatchStatuses(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeBatch) && internal.IsRadixBatchNotSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch)
	}), h.radixDeployJobComponent)
}

func (h *history) getSucceededRadixBatches(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.RadixBatch {
	radixBatches = slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeBatch) && internal.IsRadixBatchSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch)
	})
	return convertToRadixBatchStatuses(radixBatches, h.radixDeployJobComponent)
}

func radixBatchIsCompletedBefore(completedBefore time.Time, radixBatch *radixv1.RadixBatch) bool {
	return radixBatch.Status.Condition.CompletionTime != nil && (*radixBatch.Status.Condition.CompletionTime).Before(&metav1.Time{Time: completedBefore})
}

func (h *history) getNotSucceededSingleJobs(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.RadixBatch {
	return convertToRadixBatchStatuses(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeJob) && internal.IsRadixBatchNotSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch)
	}), h.radixDeployJobComponent)
}

func (h *history) getSucceededSingleJobs(radixBatches []*radixv1.RadixBatch, completedBefore time.Time) []*modelsv2.RadixBatch {
	return convertToRadixBatchStatuses(slice.FindAll(radixBatches, func(radixBatch *radixv1.RadixBatch) bool {
		return radixBatchHasType(radixBatch, kube.RadixBatchTypeJob) && internal.IsRadixBatchSucceeded(radixBatch) && radixBatchIsCompletedBefore(completedBefore, radixBatch)
	}), h.radixDeployJobComponent)
}

func radixBatchHasType(radixBatch *radixv1.RadixBatch, radixBatchType kube.RadixBatchType) bool {
	return radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(radixBatchType)
}

func (h *history) maintainHistoryLimitForBatches(ctx context.Context, radixBatchesSortedByCompletionTimeAsc []*modelsv2.RadixBatch, historyLimit int) error {
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
		if err := batch.DeleteRadixBatchByName(ctx, h.radixClient, h.env.RadixDeploymentNamespace, radixBatch.Name); err != nil {
			return err
		}
	}
	return nil
}

func sortRJSchByCompletionTimeAsc(batches []*radixv1.RadixBatch) []*radixv1.RadixBatch {
	sort.Slice(batches, func(i, j int) bool {
		batch1 := (batches)[i]
		batch2 := (batches)[j]
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
