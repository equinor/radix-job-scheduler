package v1

import (
	"context"
	"fmt"
	"github.com/equinor/radix-common/utils/slice"
	defaultsv1 "github.com/equinor/radix-job-scheduler/models/v1/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetLastEventMessageForPods returns the last event message for pods
func (handler *Handler) GetLastEventMessageForPods(pods []corev1.Pod) (map[string]string, error) {
	podNamesMap := slice.Reduce(pods, make(map[string]struct{}), func(acc map[string]struct{}, pod corev1.Pod) map[string]struct{} {
		acc[pod.Name] = struct{}{}
		return acc
	})
	eventMap := make(map[string]string)
	eventsList, err := handler.Kube.KubeClient().CoreV1().Events(handler.Env.RadixDeploymentNamespace).List(context.Background(), metav1.ListOptions{})
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

func getLabelSelectorForJobComponentForJobPods(componentName string) string {
	reqNoBatchJobName, _ := labels.NewRequirement(kube.RadixBatchJobNameLabel, selection.Exists, []string{})
	reqComponentName, _ := labels.NewRequirement(kube.RadixComponentLabel, selection.Equals, []string{componentName})
	reqJobTypeJobScheduler, _ := labels.NewRequirement(kube.RadixJobTypeLabel, selection.Equals, []string{kube.RadixJobTypeJobSchedule})
	reqJobPodScheduler, _ := labels.NewRequirement(defaultsv1.K8sJobNameLabel, selection.Exists, []string{})
	return labels.NewSelector().
		Add(*reqNoBatchJobName).
		Add(*reqComponentName).
		Add(*reqJobTypeJobScheduler).
		Add(*reqJobPodScheduler).
		String()
}
