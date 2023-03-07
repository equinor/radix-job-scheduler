package radix

import (
	"context"
	"encoding/json"
	"fmt"
	commonUtils "github.com/equinor/radix-common/utils"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-job-scheduler/utils/radix/test"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-job-scheduler/models"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclientfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewWebhookNotifier(t *testing.T) {
	tests := []struct {
		name            string
		notifications   *radixv1.Notifications
		expectedEnabled bool
		expectedWebhook string
		expectedErr     bool
	}{
		{
			name:            "No notification",
			notifications:   nil,
			expectedEnabled: false, expectedWebhook: "", expectedErr: false,
		},
		{
			name:            "Empty notification",
			notifications:   &radixv1.Notifications{},
			expectedEnabled: false, expectedWebhook: "", expectedErr: false,
		},
		{
			name:            "Empty webhook in the notification",
			notifications:   &radixv1.Notifications{Webhook: pointers.Ptr("")},
			expectedEnabled: false, expectedWebhook: "", expectedErr: false,
		},
		{
			name:            "Set webhook in the notification",
			notifications:   &radixv1.Notifications{Webhook: pointers.Ptr("http://job1:8080")},
			expectedEnabled: true, expectedWebhook: "http://job1:8080", expectedErr: false,
		},
		{
			name:            "Invalid webhook to non existing job component name",
			notifications:   &radixv1.Notifications{Webhook: pointers.Ptr("http://not-existing-job:8080")},
			expectedEnabled: false, expectedWebhook: "", expectedErr: true,
		},
		{
			name:            "Invalid webhook to non existing job component port",
			notifications:   &radixv1.Notifications{Webhook: pointers.Ptr("http://job1:8081")},
			expectedEnabled: false, expectedWebhook: "", expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appName := "app"
			environment := "qa"
			envBranch := "main"
			radixClient := radixclientfake.NewSimpleClientset()
			env := models.NewEnv()
			ra := test.GetRadixApplicationWithRadixJobComponent(appName, environment, envBranch, "job1", 8080, tt.notifications)
			rd := test.GetRadixDeploymentWithRadixJobComponent(appName, environment, "job1")
			_, err := radixClient.RadixV1().RadixApplications(utils.GetAppNamespace(appName)).Create(context.Background(), ra, metav1.CreateOptions{})
			if err != nil {
				panic(err)
			}
			_, err = radixClient.RadixV1().RadixDeployments(utils.GetEnvironmentNamespace(appName, environment)).Create(context.Background(), rd, metav1.CreateOptions{})
			if err != nil {
				panic(err)
			}

			gotNotifier, err := NewWebhookNotifier(ra, tt.notifications, env)
			if err != nil && !tt.expectedErr {
				t.Errorf("not expected error %v", err)
				return
			} else if err == nil && tt.expectedErr {
				t.Errorf("missing expected error")
				return
			}
			if err != nil {
				return
			}
			notifier := gotNotifier.(*webhookNotifier)
			assert.Equal(t, tt.expectedEnabled, notifier.enabled)
			assert.Equal(t, tt.expectedWebhook, notifier.webhook)
		})
	}
}

type testTransport struct {
	requestReceived func(request *http.Request)
}

func (t *testTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.requestReceived(request)
	return &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
	}, nil
}

func Test_webhookNotifier_Notify(t *testing.T) {
	type fields struct {
		enabled                 bool
		webhook                 string
		expectedRequest         bool
		expectedError           bool
		expectedBatchNameInJobs string
	}
	type args struct {
		newRadixBatch      *radixv1.RadixBatch
		updatedJobStatuses []radixv1.RadixBatchJobStatus
	}
	activeTime := metav1.NewTime(time.Date(2020, 10, 30, 1, 1, 1, 1, &time.Location{}))
	completedTime := metav1.NewTime(activeTime.Add(30 * time.Minute))
	startJobTime1 := metav1.NewTime(activeTime.Add(1 * time.Minute))
	startJobTime3 := metav1.NewTime(activeTime.Add(2 * time.Minute))
	endJobTime1 := metav1.NewTime(activeTime.Add(10 * time.Minute))
	endJobTime3 := metav1.NewTime(activeTime.Add(20 * time.Minute))

	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{name: "No request for not enabled notifier",
			fields: fields{enabled: false, webhook: "http://job1:8080", expectedRequest: false, expectedError: false},
			args: args{
				newRadixBatch: &radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Status: radixv1.RadixBatchStatus{Condition: radixv1.RadixBatchCondition{Type: radixv1.BatchConditionTypeWaiting}}},
				updatedJobStatuses: []radixv1.RadixBatchJobStatus{{Name: "job1"}}},
		},
		{name: "No request for enabled notifier with empty webhook",
			fields: fields{enabled: true, webhook: "", expectedRequest: false, expectedError: false},
			args: args{
				newRadixBatch: &radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Status: radixv1.RadixBatchStatus{Condition: radixv1.RadixBatchCondition{Type: radixv1.BatchConditionTypeWaiting}}},
				updatedJobStatuses: []radixv1.RadixBatchJobStatus{{Name: "job1"}}},
		},
		{name: "Waiting batch, no jobs",
			fields: fields{enabled: true, webhook: "http://job1:8080", expectedRequest: true, expectedError: false, expectedBatchNameInJobs: "batch1"},
			args: args{
				newRadixBatch: &radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Status: radixv1.RadixBatchStatus{
						Condition: radixv1.RadixBatchCondition{
							Type:    radixv1.BatchConditionTypeWaiting,
							Reason:  "some reason",
							Message: "some message",
						},
					},
				},
				updatedJobStatuses: []radixv1.RadixBatchJobStatus{}},
		},
		{name: "Active batch",
			fields: fields{enabled: true, webhook: "http://job1:8080", expectedRequest: true, expectedError: false, expectedBatchNameInJobs: "batch1"},
			args: args{
				newRadixBatch: &radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Status: radixv1.RadixBatchStatus{
						Condition: radixv1.RadixBatchCondition{
							Type:           radixv1.BatchConditionTypeActive,
							Reason:         "some reason",
							Message:        "some message",
							CompletionTime: &completedTime,
							ActiveTime:     &activeTime,
						},
					},
				},
				updatedJobStatuses: []radixv1.RadixBatchJobStatus{}},
		},
		{name: "Completed Batch with multiple jobs",
			fields: fields{enabled: true, webhook: "http://job1:8080", expectedRequest: true, expectedError: false, expectedBatchNameInJobs: "batch1"},
			args: args{
				newRadixBatch: &radixv1.RadixBatch{
					ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeBatch)},
					Spec: radixv1.RadixBatchSpec{
						Jobs: []radixv1.RadixBatchJob{{Name: "job1"}, {Name: "job2"}, {Name: "job3"}},
					},
					Status: radixv1.RadixBatchStatus{
						Condition: radixv1.RadixBatchCondition{
							Type:           radixv1.BatchConditionTypeCompleted,
							Reason:         "some reason",
							Message:        "some message",
							CompletionTime: &completedTime,
							ActiveTime:     &activeTime,
						},
					},
				},
				updatedJobStatuses: []radixv1.RadixBatchJobStatus{
					{
						Name: "job1",
					},
					{
						Name:      "job2",
						Phase:     "some job phase",
						StartTime: &startJobTime1,
					},
					{
						Name:      "job3",
						Phase:     "some job phase 753",
						Reason:    "some reason 123",
						Message:   "some message 456",
						StartTime: &startJobTime3,
						EndTime:   &endJobTime3,
					},
				}},
		},
		{name: "Completed single job batch",
			fields: fields{enabled: true, webhook: "http://job1:8080", expectedRequest: true, expectedError: false, expectedBatchNameInJobs: ""},
			args: args{
				newRadixBatch: &radixv1.RadixBatch{
					ObjectMeta: metav1.ObjectMeta{Name: "batch1", Labels: labels.ForBatchType(kube.RadixBatchTypeJob)},
					Spec: radixv1.RadixBatchSpec{
						Jobs: []radixv1.RadixBatchJob{{Name: "job1"}},
					},
					Status: radixv1.RadixBatchStatus{
						Condition: radixv1.RadixBatchCondition{
							Type:           radixv1.BatchConditionTypeCompleted,
							Reason:         "some reason",
							Message:        "some message",
							CompletionTime: &completedTime,
							ActiveTime:     &activeTime,
						},
					},
				},
				updatedJobStatuses: []radixv1.RadixBatchJobStatus{
					{
						Name:      "job1",
						Phase:     "some job phase 753",
						Reason:    "some reason 123",
						Message:   "some message 456",
						StartTime: &startJobTime1,
						EndTime:   &endJobTime1,
					},
				}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := &webhookNotifier{
				enabled: tt.fields.enabled,
				webhook: tt.fields.webhook,
			}

			var receivedRequest *http.Request
			http.DefaultClient = &http.Client{
				Transport: &testTransport{
					requestReceived: func(request *http.Request) {
						receivedRequest = request
					},
				},
			}

			errChan := make(chan error)
			doneChan := notifier.Notify(tt.args.newRadixBatch, tt.args.updatedJobStatuses, errChan)

			var notificationErr error
			select {
			case <-doneChan:
				break
			case notificationErr = <-errChan:
				break
			case <-time.After(1 * time.Minute):
				assert.Fail(t, "unexpected long request timeout")
				break
			}

			if tt.fields.expectedRequest && receivedRequest == nil {
				assert.Fail(t, "missing an expected http request")
				return
			} else if !tt.fields.expectedRequest && receivedRequest != nil {
				assert.Fail(t, "received a not expected http request")
				return
			}
			if tt.fields.expectedError && notificationErr == nil {
				assert.Fail(t, "missing an expected notification error")
				return
			} else if !tt.fields.expectedError && notificationErr != nil {
				assert.Fail(t, fmt.Sprintf("received a not expected notification error %v", notificationErr))
				return
			}
			if receivedRequest != nil {
				assert.Equal(t, tt.fields.webhook, fmt.Sprintf("%s://%s", receivedRequest.URL.Scheme, receivedRequest.Host))
				var batchStatus modelsv1.BatchStatus
				if body, _ := io.ReadAll(receivedRequest.Body); len(body) > 0 {
					if err := json.Unmarshal(body, &batchStatus); err != nil {
						assert.Fail(t, fmt.Sprintf("failed to decerialize the request body: %v", err))
						return
					}
				}
				assert.Equal(t, tt.args.newRadixBatch.Name, batchStatus.Name, "Not matching batch name")
				assertTimesEqual(t, tt.args.newRadixBatch.Status.Condition.ActiveTime, batchStatus.Started, "batchStatus.Started")
				assertTimesEqual(t, tt.args.newRadixBatch.Status.Condition.CompletionTime, batchStatus.Ended, "batchStatus.Ended")
				if len(tt.args.updatedJobStatuses) != len(batchStatus.JobStatuses) {
					assert.Fail(t, fmt.Sprintf("Not matching amount of updatedJobStatuses %d and JobStatuses %d", len(tt.args.updatedJobStatuses), len(batchStatus.JobStatuses)))
					return
				}
				for index, updateJobsStatus := range tt.args.updatedJobStatuses {
					jobStatus := batchStatus.JobStatuses[index]
					assert.Equal(t, tt.fields.expectedBatchNameInJobs, jobStatus.BatchName)
					assert.Equal(t, fmt.Sprintf("%s-%s", batchStatus.Name, updateJobsStatus.Name), jobStatus.Name)
					assertTimesEqual(t, updateJobsStatus.StartTime, jobStatus.Started, "job.Started")
					assertTimesEqual(t, updateJobsStatus.EndTime, jobStatus.Ended, "job.EndTime")
				}
			}
		})
	}
}

func assertTimesEqual(t *testing.T, expectedTime *metav1.Time, resultTime string, arg string) {
	if expectedTime != nil && len(resultTime) == 0 {
		assert.Fail(t, fmt.Sprintf("missing an expected %s", arg))
		return
	} else if expectedTime == nil && len(resultTime) > 0 {
		assert.Fail(t, fmt.Sprintf("got a not expected %s", arg))
		return
	}
	if expectedTime != nil {
		assert.Equal(t, commonUtils.FormatTime(expectedTime), resultTime)
	}
}
