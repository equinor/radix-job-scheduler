package jobs

import (
	"context"
	"fmt"
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"sort"
	"strings"

	"github.com/equinor/radix-common/utils"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetJobStatusFromJob Gets job from a k8s job
func GetJobStatusFromJob(kubeClient kubernetes.Interface, job *v1.Job, jobPods []corev1.Pod) *models.JobStatus {
	jobStatus := models.JobStatus{
		Name:    job.GetName(),
		Created: utils.FormatTime(&job.ObjectMeta.CreationTimestamp),
		Started: utils.FormatTime(job.Status.StartTime),
		Ended:   utils.FormatTime(job.Status.CompletionTime),
	}
	status := models.GetStatusFromJobStatus(job.Status)
	jobStatus.Status = status.String()
	jobStatus.JobId = job.ObjectMeta.Labels[kube.RadixJobIdLabel]         //Not empty, if JobId exists
	jobStatus.BatchName = job.ObjectMeta.Labels[kube.RadixBatchNameLabel] //Not empty, if BatchName exists
	if status != models.Running {
		return &jobStatus
	}
	for _, pod := range jobPods {
		if len(pod.Status.ContainerStatuses) > 0 {
			cs := pod.Status.ContainerStatuses[0]
			if cs.Ready {
				continue
			}
			switch {
			case cs.State.Terminated != nil:
				jobStatus.Status = models.Stopped.String()
				jobStatus.Message = cs.State.Terminated.Message
				return &jobStatus
			case cs.State.Waiting != nil:
				jobStatus.Status = models.Waiting.String()
				jobStatus.Started = ""
				message := cs.State.Waiting.Message
				if len(message) > 0 {
					jobStatus.Message = message
					return &jobStatus
				}
				jobStatus.Message = getLastEventMessageForPod(kubeClient, pod)
				if len(jobStatus.Message) == 0 {
					jobStatus.Message = "Job has not been started. If it takes long time to start, please check an events list for a reason."
				}
				return &jobStatus
			}
			continue
		}
		if len(pod.Status.Conditions) > 0 {

			lastCondition := sortPodStatusConditionsDesc(pod.Status.Conditions)[0]
			if lastCondition.Status == corev1.ConditionTrue {
				continue
			}
			jobStatus.Status = models.Waiting.String()
			jobStatus.Message = fmt.Sprintf("%s %s", lastCondition.Reason, lastCondition.Message)
		}
	}
	return &jobStatus
}

func getLastEventMessageForPod(kubeClient kubernetes.Interface, pod corev1.Pod) string {
	eventsList, err := kubeClient.CoreV1().Events(pod.Namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return ""
	}
	events := sortEventsDesc(eventsList.Items)
	for _, event := range events {
		if event.InvolvedObject.Name == pod.Name {
			if strings.Contains(event.Message, "container init was OOM-killed (memory limit too low?)") {
				return fmt.Sprintf("Memory limit is probably too low. Error: %s", event.Message)
			}
			return event.Message
		}
	}
	return ""
}

func sortEventsDesc(events []corev1.Event) []corev1.Event {
	sort.Slice(events, func(i, j int) bool {
		if events[i].CreationTimestamp.IsZero() || events[j].CreationTimestamp.IsZero() {
			return false
		}
		return events[j].CreationTimestamp.Before(&events[i].CreationTimestamp)
	})
	return events
}

func sortPodStatusConditionsDesc(podConditions []corev1.PodCondition) []corev1.PodCondition {
	sort.Slice(podConditions, func(i, j int) bool {
		if podConditions[i].LastTransitionTime.IsZero() || podConditions[j].LastTransitionTime.IsZero() {
			return false
		}
		return podConditions[j].LastTransitionTime.Before(&podConditions[i].LastTransitionTime)
	})
	return podConditions
}
