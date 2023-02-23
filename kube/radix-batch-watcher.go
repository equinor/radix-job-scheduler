package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	apiModels "github.com/equinor/radix-job-scheduler/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixinformers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/pkg/client/informers/externalversions/radix/v1"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"net/http"
	"strings"
)

const (
	resyncPeriod = 0
)

var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{"radixJobSchedulerServer": "kube-radix-batch-Watcher"})
}

type Watcher struct {
	radixInformerFactory radixinformers.SharedInformerFactory
	batchInformer        v1.RadixBatchInformer
	Stop                 chan struct{}
}

func New(radixClient radixclient.Interface, env *apiModels.Env) (*Watcher, error) {
	namespace := env.RadixDeploymentNamespace
	existingRadixBatchMap, err := getRadixBatchMap(radixClient, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get list of RadixBatches %w", err)
	}

	w := Watcher{
		radixInformerFactory: radixinformers.NewSharedInformerFactory(radixClient, resyncPeriod),
		Stop:                 make(chan struct{}),
	}
	w.batchInformer = w.radixInformerFactory.Radix().V1().RadixBatches()

	logger.Info("Setting up event handlers")
	w.batchInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			newRadixBatch := cur.(*radixv1.RadixBatch)
			if !strings.EqualFold(newRadixBatch.GetNamespace(), namespace) {
				return
			}
			if _, ok := existingRadixBatchMap[newRadixBatch.GetName()]; ok {
				logger.Debugf("skip existing RadixBatch object %s", newRadixBatch.GetName())
				return
			}
			logger.Debugf("RadixBatch object was added %s", newRadixBatch.GetName())
		},
		UpdateFunc: func(old, cur interface{}) {
			oldRadixBatch := old.(*radixv1.RadixBatch)
			newRadixBatch := cur.(*radixv1.RadixBatch)
			if !strings.EqualFold(newRadixBatch.GetNamespace(), namespace) {
				return
			}
			oldJobStatuses := make(map[string]radixv1.RadixBatchJobStatus)
			for _, jobStatus := range oldRadixBatch.Status.JobStatuses {
				batchJobStatus := jobStatus
				oldJobStatuses[batchJobStatus.Name] = batchJobStatus
			}
			var updatedJobStatuses []radixv1.RadixBatchJobStatus
			for _, newJobStatus := range newRadixBatch.Status.JobStatuses {
				if oldJobStatus, ok := oldJobStatuses[newJobStatus.Name]; !ok || notEqualJobStatuses(&oldJobStatus, &newJobStatus) {
					updatedJobStatuses = append(updatedJobStatuses, newJobStatus)
				}
			}
			//TODO convert to modelsv1.BatchStatus
			if len(updatedJobStatuses) == 0 {
				logger.Debugf("RadixBatch job statuses have no changes in the batch %s. Do nothing", newRadixBatch.GetName())
				return
			}
			statusesJson, err := json.Marshal(updatedJobStatuses)
			if err != nil {
				logger.Errorf("failed serialise updatedJobStatuses %v", err)
				return
			}
			buf := bytes.NewReader(statusesJson)
			resp, err := http.Post("http://localhost:8082", "application/json", buf)
			if err != nil {
				logger.Errorf("fail sending callback %v", err)
			} else {
				logger.Infof("sent update callback %s. Respond: %s", newRadixBatch.Name, resp.Status)
			}
			logger.Debugf("RadixBatch object was changed %s", newRadixBatch.GetName())
		},
		DeleteFunc: func(obj interface{}) {
			radixBatch, _ := obj.(*radixv1.RadixBatch)
			if !strings.EqualFold(radixBatch.GetNamespace(), namespace) {
				return
			}
			key, err := cache.MetaNamespaceKeyFunc(radixBatch)
			if err != nil {
				logger.Errorf("fail on received event deleted RadixBatch object %s: %v", key, err)
				return
			}
			logger.Debugf("RadixBatch object was deleted %s", radixBatch.GetName())
			delete(existingRadixBatchMap, radixBatch.GetName())
		},
	})
	w.radixInformerFactory.Start(w.Stop)
	return &w, nil
}

func notEqualJobStatuses(status1, status2 *radixv1.RadixBatchJobStatus) bool {
	return status1.Phase != status2.Phase ||
		status1.Reason != status2.Reason ||
		status1.Message != status2.Message
}

func getRadixBatchMap(radixClient radixclient.Interface, namespace string) (map[string]*radixv1.RadixBatch, error) {
	radixBatchList, err := radixClient.RadixV1().RadixBatches(namespace).List(context.Background(), metav1.ListOptions{})
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
