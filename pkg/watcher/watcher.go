package watcher

import (
	"context"
	"fmt"
	"time"

	"github.com/equinor/radix-job-scheduler/models/v1/events"
	"github.com/equinor/radix-job-scheduler/pkg/batch"
	"github.com/equinor/radix-job-scheduler/pkg/notifications"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixinformers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	v1 "github.com/equinor/radix-operator/pkg/client/informers/externalversions/radix/v1"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	resyncPeriod = 0
)

// Watcher Watcher interface
type Watcher interface {
	Stop()
}

type watcher struct {
	radixInformerFactory radixinformers.SharedInformerFactory
	batchInformer        v1.RadixBatchInformer
	stop                 chan struct{}
	logger               zerolog.Logger
	jobHistory           batch.History
}

// Stop Stops the watcher
func (w *watcher) Stop() {
	w.stop <- struct{}{}
}

// NewRadixBatchWatcher New RadixBatch watcher, notifying on adding and changing of RadixBatches and their jobs
func NewRadixBatchWatcher(ctx context.Context, radixClient radixclient.Interface, namespace string, jobHistory batch.History, notifier notifications.Notifier) (Watcher, error) {
	watcher := watcher{
		stop:                 make(chan struct{}),
		radixInformerFactory: radixinformers.NewSharedInformerFactoryWithOptions(radixClient, resyncPeriod, radixinformers.WithNamespace(namespace)),
		logger:               log.Logger.With().Str("pkg", "radix-batch-watcher").Logger(),
		jobHistory:           jobHistory,
	}

	existingRadixBatchMap, err := getRadixBatchMap(radixClient, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get list of RadixBatches %w", err)
	}

	watcher.batchInformer = watcher.radixInformerFactory.Radix().V1().RadixBatches()

	watcher.logger.Info().Msg("Setting up event handlers")

	_, err = watcher.batchInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			radixBatch, converted := cur.(*radixv1.RadixBatch)
			if !converted {
				log.Error().Msg("Failed to cast RadixBatch object")
				return
			}
			if radixBatch.Status.Condition.Type != "" {
				return // skip existing batch added to the cache
			}
			if _, ok := existingRadixBatchMap[radixBatch.GetName()]; ok {
				watcher.logger.Debug().Msgf("skip existing RadixBatch object %s", radixBatch.GetName())
				return
			}
			watcher.logger.Debug().Msgf("RadixBatch object was added %s", radixBatch.GetName())
			jobStatuses := radixBatch.Status.JobStatuses
			if len(jobStatuses) == 0 {
				jobStatuses = make([]radixv1.RadixBatchJobStatus, 0)
			}
			notify(ctx, notifier, events.Create, radixBatch, jobStatuses)
			watcher.cleanupJobHistory(ctx)
		},
		UpdateFunc: func(old, cur interface{}) {
			oldRadixBatch := old.(*radixv1.RadixBatch)
			newRadixBatch := cur.(*radixv1.RadixBatch)
			updatedJobStatuses := getUpdatedJobStatuses(oldRadixBatch, newRadixBatch)
			if len(updatedJobStatuses) == 0 && equalBatchStatuses(&oldRadixBatch.Status, &newRadixBatch.Status) {
				watcher.logger.Debug().Msgf("RadixBatch status and job statuses have no changes in the batch %s. Do nothing", newRadixBatch.GetName())
				return
			}
			watcher.logger.Debug().Msgf("RadixBatch object was changed %s", newRadixBatch.GetName())
			notify(nil, notifier, events.Update, newRadixBatch, updatedJobStatuses)
		},
		DeleteFunc: func(obj interface{}) {
			radixBatch, _ := obj.(*radixv1.RadixBatch)
			key, err := cache.MetaNamespaceKeyFunc(radixBatch)
			if err != nil {
				watcher.logger.Error().Err(err).Msgf("fail on received event deleted RadixBatch object %s", key)
				return
			}
			watcher.logger.Debug().Msgf("RadixBatch object was deleted %s", radixBatch.GetName())
			jobStatuses := radixBatch.Status.JobStatuses
			if len(jobStatuses) == 0 {
				jobStatuses = make([]radixv1.RadixBatchJobStatus, 0)
			}
			notify(nil, notifier, events.Delete, radixBatch, jobStatuses)
			delete(existingRadixBatchMap, radixBatch.GetName())
		},
	})
	if err != nil {
		watcher.logger.Error().Err(err).Msg("Failed to setup job informer")
		return nil, err
	}

	watcher.radixInformerFactory.Start(watcher.stop)
	log.Info().Msg("Waiting for Radix objects caches to sync")
	watcher.radixInformerFactory.WaitForCacheSync(ctx.Done())
	log.Info().Msg("Completed syncing informer caches")

	go func() {
		<-watcher.stop
		return
	}()
	return &watcher, nil
}

func notify(ctx context.Context, notifier notifications.Notifier, ev events.Event, newRadixBatch *radixv1.RadixBatch, updatedJobStatuses []radixv1.RadixBatchJobStatus) {
	go func() {
		if err := notifier.Notify(ev, newRadixBatch, updatedJobStatuses); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("failed to notify")
		}
	}()
}

func (w *watcher) cleanupJobHistory(ctx context.Context) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Minute*5)
	go func() {
		defer cancel()
		if err := w.jobHistory.Cleanup(ctxWithTimeout); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("failed to cleanup job history")
		}
	}()
}

func getUpdatedJobStatuses(oldRadixBatch *radixv1.RadixBatch, newRadixBatch *radixv1.RadixBatch) []radixv1.RadixBatchJobStatus {
	oldJobStatuses := make(map[string]radixv1.RadixBatchJobStatus)
	for _, jobStatus := range oldRadixBatch.Status.JobStatuses {
		batchJobStatus := jobStatus
		oldJobStatuses[batchJobStatus.Name] = batchJobStatus
	}
	updatedJobStatuses := make([]radixv1.RadixBatchJobStatus, 0, len(newRadixBatch.Status.JobStatuses))
	for _, newJobStatus := range newRadixBatch.Status.JobStatuses {
		if oldJobStatus, ok := oldJobStatuses[newJobStatus.Name]; !ok || !equalJobStatuses(&oldJobStatus, &newJobStatus) {
			updatedJobStatuses = append(updatedJobStatuses, newJobStatus)
		}
	}
	return updatedJobStatuses
}

func equalJobStatuses(status1, status2 *radixv1.RadixBatchJobStatus) bool {
	return status1.Phase == status2.Phase &&
		status1.Reason == status2.Reason &&
		status1.Message == status2.Message
}

func equalBatchStatuses(status1, status2 *radixv1.RadixBatchStatus) bool {
	return status1.Condition.ActiveTime == status2.Condition.ActiveTime &&
		status1.Condition.CompletionTime == status2.Condition.CompletionTime &&
		status1.Condition.Reason == status2.Condition.Reason &&
		status1.Condition.Type == status2.Condition.Type &&
		status1.Condition.Message == status2.Condition.Message
}

func getRadixBatchMap(radixClient radixclient.Interface, namespace string) (map[string]*radixv1.RadixBatch, error) {
	radixBatchList, err := radixClient.RadixV1().RadixBatches(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	radixBatchMap := make(map[string]*radixv1.RadixBatch, len(radixBatchList.Items))
	for _, radixBatch := range radixBatchList.Items {
		radixBatch := radixBatch
		radixBatchMap[radixBatch.GetName()] = &radixBatch
	}
	return radixBatchMap, nil
}
