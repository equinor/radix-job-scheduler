package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-job-scheduler/models/v1/events"
	"github.com/equinor/radix-job-scheduler/pkg/internal"
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

func (notifier *webhookNotifier) Notify(event events.Event, radixBatch *radixv1.RadixBatch, updatedJobStatuses []radixv1.RadixBatchJobStatus) error {
	if !notifier.Enabled() || len(notifier.webhookURL) == 0 || radixBatch.Spec.RadixDeploymentJobRef.Job != notifier.jobComponentName {
		return nil
	}
	// BatchStatus status and only changed job statuses
	batchStatus := getRadixBatchEventFromRadixBatch(event, radixBatch, updatedJobStatuses, notifier.radixDeployJobComponent)
	statusesJson, err := json.Marshal(batchStatus)
	if err != nil {
		return fmt.Errorf("failed serialize updated JobStatuses %v", err)
	}
	log.Trace().Msg(string(statusesJson))
	buf := bytes.NewReader(statusesJson)
	if _, err = http.Post(notifier.webhookURL, "application/json", buf); err != nil {
		return fmt.Errorf("failed to notify on BatchStatus object create or change %s: %v", radixBatch.GetName(), err)
	}
	return nil
}

func webhookIsNotEmpty(webhook *string) bool {
	return webhook != nil && len(*webhook) > 0
}

func getRadixBatchEventFromRadixBatch(event events.Event, radixBatch *radixv1.RadixBatch, radixBatchJobStatuses []radixv1.RadixBatchJobStatus, radixDeployJobComponent *radixv1.RadixDeployJobComponent) events.BatchEvent {
	batchStatus, jobStatuses := internal.GetBatchAndJobStatuses(radixBatch, radixDeployJobComponent, radixBatchJobStatuses)
	return events.BatchEvent{
		Event: event,
		BatchStatus: modelsv1.BatchStatus{
			JobStatus:   batchStatus,
			JobStatuses: jobStatuses,
			BatchType:   radixBatch.Labels[kube.RadixBatchTypeLabel],
		},
	}
}
