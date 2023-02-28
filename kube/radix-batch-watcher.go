package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	apimodelsv2 "github.com/equinor/radix-job-scheduler/api/v2"
	apiModels "github.com/equinor/radix-job-scheduler/models"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixinformers "github.com/equinor/radix-operator/pkg/client/informers/externalversions"
	"github.com/equinor/radix-operator/pkg/client/informers/externalversions/radix/v1"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	resyncPeriod = 0
)

var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{"radixJobScheduler": "radix-batch-watcher"})
}

type Watcher struct {
	radixInformerFactory radixinformers.SharedInformerFactory
	batchInformer        v1.RadixBatchInformer
	Stop                 chan struct{}
	webhook              string
}

func New(radixClient radixclient.Interface, env *apiModels.Env, radixDeployJobComponent *radixv1.RadixDeployJobComponent) (*Watcher, error) {
	w := Watcher{Stop: make(chan struct{})}

	if radixDeployJobComponent.Notifications == nil || !webhookIsValid(radixDeployJobComponent.Notifications.Webhook) {
		return &w, nil
	}

	w.radixInformerFactory = radixinformers.NewSharedInformerFactory(radixClient, resyncPeriod)
	w.webhook = radixDeployJobComponent.Notifications.Webhook

	namespace := env.RadixDeploymentNamespace
	existingRadixBatchMap, err := getRadixBatchMap(radixClient, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get list of RadixBatches %w", err)
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
			err := w.notifyWebhook(newRadixBatch, newRadixBatch.Status.JobStatuses)
			if err != nil {
				logger.Errorf("failed notify on RadixBatch object was added %s: %v", newRadixBatch.GetName(), err)
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
				if oldJobStatus, ok := oldJobStatuses[newJobStatus.Name]; !ok || !equalJobStatuses(&oldJobStatus, &newJobStatus) {
					updatedJobStatuses = append(updatedJobStatuses, newJobStatus)
				}
			}
			if len(updatedJobStatuses) == 0 && equalBatchStatuses(&oldRadixBatch.Status, &newRadixBatch.Status) {
				logger.Debugf("RadixBatch job statuses have no changes in the batch %s. Do nothing", newRadixBatch.GetName())
				return
			}
			err := w.notifyWebhook(newRadixBatch, updatedJobStatuses)
			if err != nil {
				logger.Errorf("failed notify on RadixBatch object was changed %s: %v", newRadixBatch.GetName(), err)
				return
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

func (w *Watcher) notifyWebhook(newRadixBatch *radixv1.RadixBatch, updatedJobStatuses []radixv1.RadixBatchJobStatus) error {
	// RadixBatch status and only changed job statuses
	batchStatus := getRadixBatchModelFromRadixBatch(newRadixBatch, updatedJobStatuses)
	statusesJson, err := json.Marshal(batchStatus)
	if err != nil {
		return fmt.Errorf("failed serialise updatedJobStatuses %v", err)
	}
	buf := bytes.NewReader(statusesJson)
	resp, err := http.Post(w.webhook, "application/json", buf)
	if err != nil {
		return fmt.Errorf("fail sending callback %v", err)
	}
	logger.Infof("sent update callback %s. Respond: %s", newRadixBatch.Name, resp.Status)
	return nil
}

func webhookIsValid(webhook string) bool {
	return len(webhook) > 0
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

func getRadixBatchModelFromRadixBatch(radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus) modelsv1.BatchStatus {
	batchType := radixBatch.Labels[kube.RadixBatchTypeLabel]
	jobStatusBatchName := utils.TernaryString(batchType == string(kube.RadixBatchTypeJob), "", radixBatch.GetName())
	return modelsv1.BatchStatus{
		JobStatus: modelsv1.JobStatus{
			Name:    radixBatch.GetName(),
			Created: utils.FormatTime(pointers.Ptr(radixBatch.GetCreationTimestamp())),
			Started: utils.FormatTime(radixBatch.Status.Condition.ActiveTime),
			Ended:   utils.FormatTime(radixBatch.Status.Condition.CompletionTime),
			Status:  apimodelsv2.GetRadixBatchStatus(radixBatch).String(),
			Message: radixBatch.Status.Condition.Message,
		},
		JobStatuses: getRadixBatchJobStatusesFromRadixBatch(radixBatch, radixBatchJobStatuses, jobStatusBatchName),
		BatchType:   batchType,
	}
}

func getRadixBatchJobStatusesFromRadixBatch(radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus, jobStatusBatchName string) []modelsv1.JobStatus {
	radixBatchJobsMap := getRadixBatchJobsMap(radixBatch.Spec.Jobs)
	var jobStatuses []modelsv1.JobStatus
	for _, radixBatchJobStatus := range radixBatchJobStatuses {
		radixBatchJob, ok := radixBatchJobsMap[radixBatchJobStatus.Name]
		if !ok {
			continue
		}
		jobName := fmt.Sprintf("%s-%s", radixBatch.Name, radixBatchJobStatus.Name) //composed name in models are always consist of a batchName and original jobName
		jobStatus := modelsv1.JobStatus{
			BatchName: jobStatusBatchName,
			Name:      jobName,
			JobId:     radixBatchJob.JobId,
			Created:   utils.FormatTime(radixBatchJobStatus.CreationTime),
			Started:   utils.FormatTime(radixBatchJobStatus.StartTime),
			Ended:     utils.FormatTime(radixBatchJobStatus.EndTime),
			Status:    apimodelsv2.GetRadixBatchJobStatusFromPhase(radixBatchJob, radixBatchJobStatus.Phase).String(),
			Message:   radixBatchJobStatus.Message,
		}
		jobStatuses = append(jobStatuses, jobStatus)
	}
	return jobStatuses
}

func getRadixBatchJobsMap(radixBatchJobs []radixv1.RadixBatchJob) map[string]radixv1.RadixBatchJob {
	jobMap := make(map[string]radixv1.RadixBatchJob, len(radixBatchJobs))
	for _, radixBatchJob := range radixBatchJobs {
		jobMap[radixBatchJob.Name] = radixBatchJob
	}
	return jobMap
}
