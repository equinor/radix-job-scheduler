package jobs

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-job-scheduler/models/common"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	defaultsv1 "github.com/equinor/radix-job-scheduler/models/v1/defaults"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var imageErrors = map[string]bool{"ImagePullBackOff": true, "ImageInspectError": true, "ErrImagePull": true,
	"ErrImageNeverPull": true, "RegistryUnavailable": true, "InvalidImageName": true}

// GetJobStatusFromJob Gets job from a k8s job
func GetJobStatusFromJob(kubeClient kubernetes.Interface, job *v1.Job, jobPods []corev1.Pod) *modelsv1.JobStatus {
	jobStatus := modelsv1.JobStatus{
		Name:    job.GetName(),
		Created: utils.FormatTime(&job.ObjectMeta.CreationTimestamp),
		Started: utils.FormatTime(job.Status.StartTime),
		Ended:   getJobEndTimestamp(job),
	}
	status := common.GetStatusFromJobStatus(job.Status)

	jobStatus.Status = status.String()
	jobStatus.JobId = job.ObjectMeta.Labels[defaultsv1.RadixJobIdLabel]   //Not empty, if JobId exists
	jobStatus.BatchName = job.ObjectMeta.Labels[kube.RadixBatchNameLabel] //Not empty, if BatchName exists
	if status != common.Running {
		// if the job is not in state 'Running', we check that job's pod status reason
		for _, pod := range jobPods {
			if pod.Status.Reason == "DeadlineExceeded" {
				// if the pod's status reason is 'DeadlineExceeded', the entire job also gets that status
				jobStatus.Status = common.DeadlineExceeded.String()
				jobStatus.Message = pod.Status.Message
				return &jobStatus
			}
		}
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
				//  job with one or more 'terminated' containers gets status 'Stopped'
				jobStatus.Status = common.Stopped.String()
				jobStatus.Message = cs.State.Terminated.Message

				return &jobStatus
			case cs.State.Waiting != nil:
				if _, ok := imageErrors[cs.State.Waiting.Reason]; ok {
					// if container waits because of inaccessible image, the job is 'Failed'
					jobStatus.Status = common.Failed.String()
				} else {
					// if container waits for any other reason, job is 'Waiting'
					jobStatus.Status = common.Waiting.String()
				}
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
			jobStatus.Status = common.Waiting.String()
			jobStatus.Message = fmt.Sprintf("%s %s", lastCondition.Reason, lastCondition.Message)
		}
	}
	return &jobStatus
}

// GetSingleJobStatusFromRadixBatchJob Gets job status from RadixBatch
func GetSingleJobStatusFromRadixBatchJob(radixBatch *modelsv2.RadixBatch) (*modelsv1.JobStatus, error) {
	if len(radixBatch.JobStatuses) != 1 {
		return nil, fmt.Errorf("batch should have only one job")
	}
	radixBatchJobStatus := radixBatch.JobStatuses[0]
	jobStatus := modelsv1.JobStatus{
		BatchName: radixBatch.Name,
		JobId:     radixBatchJobStatus.JobId,
		Name:      radixBatchJobStatus.Name,
		Created:   radixBatchJobStatus.CreationTime,
		Started:   radixBatchJobStatus.Started,
		Ended:     radixBatchJobStatus.Ended,
		Status:    radixBatchJobStatus.Status,
		Message:   radixBatchJobStatus.Message,
	}
	return &jobStatus, nil
}

func getJobEndTimestamp(job *v1.Job) string {
	if job.Status.CompletionTime != nil {
		// if the k8s job succeeds, we simply return the CompletionTime
		return utils.FormatTime(job.Status.CompletionTime)
	}
	// if a k8s job fails, there is no timestamp for the failure. We set the job's failure time to be
	// the timestamp for the job's last status condition.
	if job.Status.Conditions != nil {
		lastCondition := sortJobStatusConditionsDesc(job.Status.Conditions)[0].LastTransitionTime
		return utils.FormatTime(&lastCondition)
	}
	return ""
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

func sortJobStatusConditionsDesc(jobConditions []v1.JobCondition) []v1.JobCondition {
	sort.Slice(jobConditions, func(i, j int) bool {
		if jobConditions[i].LastTransitionTime.IsZero() || jobConditions[j].LastTransitionTime.IsZero() {
			return false
		}
		return jobConditions[j].LastTransitionTime.Before(&jobConditions[i].LastTransitionTime)
	})
	return jobConditions
}
