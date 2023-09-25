package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/equinor/radix-job-scheduler/models/v1/events"
	"net/http"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

type webhookNotifier struct {
	enabled bool
	webhook string
}

func NewWebhookNotifier(notifications *radixv1.Notifications) (Notifier, error) {
	notifier := webhookNotifier{}
	if notifications != nil && webhookIsNotEmpty(notifications.Webhook) {
		notifier.enabled = true
		notifier.webhook = *notifications.Webhook
	}
	return &notifier, nil
}

func (notifier *webhookNotifier) Enabled() bool {
	return notifier.enabled
}

func (notifier *webhookNotifier) String() string {
	if notifier.enabled {
		return fmt.Sprintf("Webhook notifier is enabled. Webhook: %s", notifier.webhook)
	}
	return "Webhook notifier is disabled"
}

func (notifier *webhookNotifier) Notify(event events.Event, newRadixBatch *radixv1.RadixBatch, updatedJobStatuses []radixv1.RadixBatchJobStatus, errChan chan error) (done chan struct{}) {
	done = make(chan struct{})
	go func() {
		if !notifier.Enabled() || len(notifier.webhook) == 0 {
			done <- struct{}{}
			close(done)
			return
		}
		// RadixBatch status and only changed job statuses
		batchStatus := getRadixBatchEventFromRadixBatch(event, newRadixBatch, updatedJobStatuses)
		statusesJson, err := json.Marshal(batchStatus)
		if err != nil {
			errChan <- fmt.Errorf("failed serialize updated JobStatuses %v", err)
			return
		}
		buf := bytes.NewReader(statusesJson)
		_, err = http.Post(notifier.webhook, "application/json", buf)
		if err != nil {
			errChan <- fmt.Errorf("failed to notify on RadixBatch object create or change %s: %v", newRadixBatch.GetName(), err)
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
