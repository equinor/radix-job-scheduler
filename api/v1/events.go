package v1

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	defaultsv1 "github.com/equinor/radix-job-scheduler/models/v1/defaults"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetLastEventMessageForPods returns the last event message for pods
func (handler *Handler) GetLastEventMessageForPods(ctx context.Context, pods []corev1.Pod) (map[string]string, error) {
	podNamesMap := slice.Reduce(pods, make(map[string]struct{}), func(acc map[string]struct{}, pod corev1.Pod) map[string]struct{} {
		acc[pod.Name] = struct{}{}
		return acc
	})
	eventMap := make(map[string]string)
	eventsList, err := handler.Kube.KubeClient().CoreV1().Events(handler.Env.RadixDeploymentNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return eventMap, err
	}
	events := sortEventsAsc(eventsList.Items)
	for _, event := range events {
		if _, ok := podNamesMap[event.InvolvedObject.Name]; !ok || event.InvolvedObject.Kind != "Pod" {
			continue
		}
		if strings.Contains(event.Message, "container init was OOM-killed (memory limit too low?)") {
			eventMap[event.InvolvedObject.Name] = fmt.Sprintf("Memory limit is probably too low. Error: %s", event.Message)
			continue
		}
		eventMap[event.InvolvedObject.Name] = event.Message
	}
	return eventMap, nil
}

func sortEventsAsc(events []corev1.Event) []corev1.Event {
	sort.Slice(events, func(i, j int) bool {
		if events[i].CreationTimestamp.IsZero() || events[j].CreationTimestamp.IsZero() {
			return false
		}
		return events[i].CreationTimestamp.Before(&events[j].CreationTimestamp)
	})
	return events
}

// GetRadixBatchJobMessagesAndPodMaps returns the event messages for the batch job statuses
func (handler *Handler) GetRadixBatchJobMessagesAndPodMaps(ctx context.Context, selectorForRadixBatchPods string) (map[string]string, map[string]corev1.Pod, error) {
	radixBatchesPods, err := handler.GetPodsForLabelSelector(ctx, selectorForRadixBatchPods)
	if err != nil {
		return nil, nil, err
	}
	eventMessageForPods, err := handler.GetLastEventMessageForPods(ctx, radixBatchesPods)
	if err != nil {
		return nil, nil, err
	}
	batchJobPodsMap := slice.Reduce(radixBatchesPods, make(map[string]corev1.Pod), func(acc map[string]corev1.Pod, pod corev1.Pod) map[string]corev1.Pod {
		if batchJobName, ok := pod.GetLabels()[defaultsv1.K8sJobNameLabel]; ok {
			acc[batchJobName] = pod
		}
		return acc
	})
	return eventMessageForPods, batchJobPodsMap, nil
}

// SetBatchJobEventMessageToBatchJobStatus sets the event message for the batch job status
func SetBatchJobEventMessageToBatchJobStatus(jobStatus *modelsv1.JobStatus, batchJobPodsMap map[string]corev1.Pod, eventMessageForPods map[string]string) {
	if jobStatus == nil || len(jobStatus.Message) > 0 {
		return
	}
	if batchJobPod, ok := batchJobPodsMap[jobStatus.Name]; ok {
		if eventMessage, ok := eventMessageForPods[batchJobPod.Name]; ok {
			jobStatus.Message = eventMessage
		}
	}
}
