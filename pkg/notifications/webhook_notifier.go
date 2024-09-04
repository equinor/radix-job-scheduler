package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-job-scheduler/models/v1/events"
	"github.com/equinor/radix-job-scheduler/utils/radix/jobs"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/rs/zerolog/log"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type webhookNotifier struct {
	webhookURL              string
	jobComponentName        string
	radixDeployJobComponent *radixv1.RadixDeployJobComponent
}

func NewWebhookNotifier(radixDeployJobComponent *radixv1.RadixDeployJobComponent) Notifier {
	notifier := webhookNotifier{
		jobComponentName:        radixDeployJobComponent.Name,
		radixDeployJobComponent: radixDeployJobComponent,
	}
	if radixDeployJobComponent.Notifications != nil && webhookIsNotEmpty(radixDeployJobComponent.Notifications.Webhook) {
		notifier.webhookURL = *radixDeployJobComponent.Notifications.Webhook
	}
	return &notifier
}

func (notifier *webhookNotifier) Enabled() bool {
	return len(notifier.webhookURL) > 0
}

func (notifier *webhookNotifier) String() string {
	if notifier.Enabled() {
		return fmt.Sprintf("Webhook notifier is enabled. Webhook: %s", notifier.webhookURL)
	}
	return "Webhook notifier is disabled"
}

func (notifier *webhookNotifier) Notify(event events.Event, radixBatch *radixv1.RadixBatch, updatedJobStatuses []radixv1.RadixBatchJobStatus, errChan chan error) (done chan struct{}) {
	done = make(chan struct{})
	go func() {
		defer close(done)
		if !notifier.Enabled() || len(notifier.webhookURL) == 0 || radixBatch.Spec.RadixDeploymentJobRef.Job != notifier.jobComponentName {
			done <- struct{}{}
			return
		}
		// RadixBatch status and only changed job statuses
		batchStatus := getRadixBatchEventFromRadixBatch(event, radixBatch, updatedJobStatuses, notifier.radixDeployJobComponent)
		statusesJson, err := json.Marshal(batchStatus)
		if err != nil {
			errChan <- fmt.Errorf("failed serialize updated JobStatuses %v", err)
			return
		}
		log.Trace().Msg(string(statusesJson))
		buf := bytes.NewReader(statusesJson)
		_, err = http.Post(notifier.webhookURL, "application/json", buf)
		if err != nil {
			errChan <- fmt.Errorf("failed to notify on RadixBatch object create or change %s: %v", radixBatch.GetName(), err)
			return
		}
		done <- struct{}{}
	}()
	return done
}

func webhookIsNotEmpty(webhook *string) bool {
	return webhook != nil && len(*webhook) > 0
}

func getRadixBatchEventFromRadixBatch(event events.Event, radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus, radixDeployJobComponent *radixv1.RadixDeployJobComponent) events.BatchEvent {
	batchType := radixBatch.Labels[kube.RadixBatchTypeLabel]
	startedTime := utils.FormatTime(radixBatch.Status.Condition.ActiveTime)
	endedTime := utils.FormatTime(radixBatch.Status.Condition.CompletionTime)
	batchStatus := v1.JobStatus{
		Name:    radixBatch.GetName(),
		BatchId: getBatchId(radixBatch),
		Created: utils.FormatTime(pointers.Ptr(radixBatch.GetCreationTimestamp())),
		Started: startedTime,
		Ended:   endedTime,
		Status:  string(jobs.GetRadixBatchStatus(radixBatch, radixDeployJobComponent)),
		Message: radixBatch.Status.Condition.Message,
		Updated: utils.FormatTime(pointers.Ptr(v2.Now())),
	}
	jobStatuses := getRadixBatchJobStatusesFromRadixBatch(radixBatch, radixBatchJobStatuses)
	return events.BatchEvent{
		Event: event,
		BatchStatus: v1.BatchStatus{
			JobStatus:   batchStatus,
			JobStatuses: jobStatuses,
			BatchType:   batchType,
		},
	}
}

func getRadixBatchJobStatusesFromRadixBatch(radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus) []v1.JobStatus {
	batchName := getBatchName(radixBatch)
	radixBatchJobsMap := getRadixBatchJobsMap(radixBatch.Spec.Jobs)
	jobStatuses := make([]v1.JobStatus, 0, len(radixBatchJobStatuses))
	for _, radixBatchJobStatus := range radixBatchJobStatuses {
		radixBatchJob, ok := radixBatchJobsMap[radixBatchJobStatus.Name]
		if !ok {
			continue
		}
		stopJob := radixBatchJob.Stop != nil && *radixBatchJob.Stop
		jobName := fmt.Sprintf("%s-%s", radixBatch.Name, radixBatchJobStatus.Name) // composed name in models are always consist of a batchName and original jobName
		jobStatus := v1.JobStatus{
			BatchName:   batchName,
			Name:        jobName,
			JobId:       radixBatchJob.JobId,
			Created:     utils.FormatTime(radixBatchJobStatus.CreationTime),
			Started:     utils.FormatTime(radixBatchJobStatus.StartTime),
			Ended:       utils.FormatTime(radixBatchJobStatus.EndTime),
			Status:      string(jobs.GetScheduledJobStatus(radixBatchJobStatus, stopJob)),
			Failed:      radixBatchJobStatus.Failed,
			Restart:     radixBatchJobStatus.Restart,
			Message:     radixBatchJobStatus.Message,
			Updated:     utils.FormatTime(pointers.Ptr(v2.Now())),
			PodStatuses: getPodStatusByRadixBatchJobPodStatus(radixBatchJobStatus.RadixBatchJobPodStatuses),
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

func getPodStatusByRadixBatchJobPodStatus(podStatuses []radixv1.RadixBatchJobPodStatus) []v1.PodStatus {
	return slice.Map(podStatuses, func(status radixv1.RadixBatchJobPodStatus) v1.PodStatus {
		return v1.PodStatus{
			Name:             status.Name,
			Created:          utils.FormatTime(status.CreationTime),
			StartTime:        utils.FormatTime(status.StartTime),
			EndTime:          utils.FormatTime(status.EndTime),
			ContainerStarted: utils.FormatTime(status.StartTime),
			Status:           v1.ReplicaStatus{Status: string(status.Phase)},
			StatusMessage:    status.Message,
			RestartCount:     status.RestartCount,
			Image:            status.Image,
			ImageId:          status.ImageID,
			PodIndex:         status.PodIndex,
			ExitCode:         status.ExitCode,
			Reason:           status.Reason,
		}
	})
}

func getBatchName(radixBatch *radixv1.RadixBatch) string {
	return utils.TernaryString(radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeJob), "", radixBatch.GetName())
}

func getBatchId(radixBatch *radixv1.RadixBatch) string {
	return utils.TernaryString(radixBatch.GetLabels()[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeJob), "", radixBatch.Spec.BatchId)
}
