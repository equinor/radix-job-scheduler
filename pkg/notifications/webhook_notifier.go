package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-job-scheduler/internal"
	v1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-job-scheduler/models/v1/events"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/rs/zerolog/log"
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
	var startedTime, endedTime *time.Time
	if radixBatch.Status.Condition.ActiveTime != nil {
		startedTime = &radixBatch.Status.Condition.ActiveTime.Time
	}
	if radixBatch.Status.Condition.CompletionTime != nil {
		endedTime = &radixBatch.Status.Condition.CompletionTime.Time
	}

	jobStatuses := getRadixBatchJobStatusesFromRadixBatch(radixBatch, radixBatchJobStatuses)
	return events.BatchEvent{
		Event:   event,
		Updated: time.Now(),
		BatchStatus: v1.BatchStatus{
			Name:        radixBatch.GetName(),
			BatchId:     getBatchId(radixBatch),
			Created:     radixBatch.GetCreationTimestamp().Time,
			Started:     startedTime,
			Ended:       endedTime,
			Status:      string(internal.GetRadixBatchJobApiStatus(radixBatch, radixDeployJobComponent)),
			Message:     radixBatch.Status.Condition.Message,
			BatchType:   batchType,
			JobStatuses: jobStatuses,
		},
	}
}

func getRadixBatchJobStatusesFromRadixBatch(radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus) []v1.JobStatus {
	batchName := getBatchName(radixBatch)
	radixBatchJobsMap := getRadixBatchJobsMap(radixBatch.Spec.Jobs)
	jobStatuses := make([]v1.JobStatus, 0, len(radixBatchJobStatuses))
	for _, radixBatchJobStatus := range radixBatchJobStatuses {
		var started, ended, created *time.Time
		if radixBatchJobStatus.CreationTime != nil {
			created = &radixBatchJobStatus.CreationTime.Time
		}
		if radixBatchJobStatus.StartTime != nil {
			started = &radixBatchJobStatus.StartTime.Time
		}

		if radixBatchJobStatus.EndTime != nil {
			ended = &radixBatchJobStatus.EndTime.Time
		}

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
			Created:     created,
			Started:     started,
			Ended:       ended,
			Status:      string(internal.GetScheduledJobStatus(radixBatchJobStatus, stopJob)),
			Failed:      radixBatchJobStatus.Failed,
			Restart:     radixBatchJobStatus.Restart,
			Message:     radixBatchJobStatus.Message,
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
		var started, ended, created *time.Time
		if status.CreationTime != nil {
			created = &status.CreationTime.Time
		}
		if status.StartTime != nil {
			started = &status.StartTime.Time
		}

		if status.EndTime != nil {
			ended = &status.EndTime.Time
		}

		return v1.PodStatus{
			Name:             status.Name,
			Created:          created,
			StartTime:        started,
			EndTime:          ended,
			ContainerStarted: started,
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
