package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/equinor/radix-job-scheduler/models/v1/events"

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
		if !notifier.Enabled() || len(notifier.webhookURL) == 0 || radixBatch.Spec.RadixDeploymentJobRef.Job != notifier.jobComponentName {
			done <- struct{}{}
			close(done)
			return
		}
		// RadixBatch status and only changed job statuses
		batchStatus := notifier.getRadixBatchEventFromRadixBatch(event, radixBatch, updatedJobStatuses)
		statusesJson, err := json.Marshal(batchStatus)
		if err != nil {
			errChan <- fmt.Errorf("failed serialize updated JobStatuses %v", err)
			return
		}
		buf := bytes.NewReader(statusesJson)
		_, err = http.Post(notifier.webhookURL, "application/json", buf)
		if err != nil {
			errChan <- fmt.Errorf("failed to notify on RadixBatch object create or change %s: %v", radixBatch.GetName(), err)
			return
		}
		done <- struct{}{}
		close(done)
	}()
	return done
}

func webhookIsNotEmpty(webhook *string) bool {
	return webhook != nil && len(*webhook) > 0
}
