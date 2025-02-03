package v1

import (
	"fmt"
	"slices"
	"time"

	commonutils "github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BuildBatchStatus(rb radixv1.RadixBatch, batchStatusRules []radixv1.BatchStatusRule, events []corev1.Event) BatchStatus {
	jobStatusMapper := func(job radixv1.RadixBatchJob) JobStatus {
		return buildJobStatus(rb, job, events)
	}

	status := getBatchStatusEnumFromRadixBatchStatusCondition(rb.Status.Condition)
	if matchedRule, ok := findMatchingBatchStatusRule(rb, batchStatusRules); ok {
		status = getBatchStatusEnumFromRadixBatchJobApiStatus(matchedRule.BatchStatus)
	}

	batchStatus := BatchStatus{
		Name:           rb.Name,
		BatchType:      rb.Labels[kube.RadixBatchTypeLabel],
		BatchId:        rb.Spec.BatchId,
		Created:        rb.GetCreationTimestamp().Time,
		Started:        timeOrNilFromKubeTime(rb.Status.Condition.ActiveTime),
		Ended:          timeOrNilFromKubeTime(rb.Status.Condition.CompletionTime),
		Status:         status,
		JobStatuses:    slice.Map(rb.Spec.Jobs, jobStatusMapper),
		Message:        rb.Status.Condition.Message,
		DeploymentName: rb.Spec.RadixDeploymentJobRef.Name,
	}

	return batchStatus
}

func buildJobStatus(rb radixv1.RadixBatch, job radixv1.RadixBatchJob, events []corev1.Event) JobStatus {
	jobStatus := JobStatus{
		Name:      fmt.Sprintf("%s-%s", rb.Name, job.Name),
		JobId:     job.JobId,
		BatchName: commonutils.TernaryString(rb.Labels[kube.RadixBatchTypeLabel] == string(kube.RadixBatchTypeJob), "", rb.Name),
		Status:    JobStatusEnumWaiting,
	}

	batchJobStatus, batchJobStatusExist := slice.FindFirst(rb.Status.JobStatuses, func(s radixv1.RadixBatchJobStatus) bool { return s.Name == job.Name })
	if batchJobStatusExist {
		jobStatus.Created = timeOrNilFromKubeTime(batchJobStatus.CreationTime)
		jobStatus.Started = timeOrNilFromKubeTime(batchJobStatus.StartTime)
		jobStatus.Ended = timeOrNilFromKubeTime(batchJobStatus.EndTime)
		jobStatus.Message = batchJobStatus.Message
		jobStatus.Failed = batchJobStatus.Failed
		jobStatus.Restart = batchJobStatus.Restart
		jobStatus.Status = getJobStatusEnum(batchJobStatus, pointers.Val(job.Stop))
		jobStatus.PodStatuses = slice.Map(batchJobStatus.RadixBatchJobPodStatuses, buildPodStatus)

		if len(jobStatus.Message) == 0 {
			if lastEvent, ok := getLastPodEventForBatchJobStatus(batchJobStatus, events, rb.GetNamespace()); ok {
				jobStatus.Message = lastEvent.Message
			}
		}
	}

	return jobStatus
}

func getLastPodEventForBatchJobStatus(batchJobStatus radixv1.RadixBatchJobStatus, events []corev1.Event, namespace string) (foundEvent corev1.Event, ok bool) {
	podNames := slice.Map(batchJobStatus.RadixBatchJobPodStatuses, func(p radixv1.RadixBatchJobPodStatus) string { return p.Name })

	predicate := func(e corev1.Event) bool {
		return e.InvolvedObject.Kind == "Pod" && e.InvolvedObject.Namespace == namespace && slices.Contains(podNames, e.InvolvedObject.Name)
	}

	for _, event := range slice.FindAll(events, predicate) {
		if !event.LastTimestamp.Before(&foundEvent.LastTimestamp) {
			foundEvent = event
			ok = true
		}
	}

	return
}

func buildPodStatus(podStatus radixv1.RadixBatchJobPodStatus) PodStatus {
	return PodStatus{
		Name:             podStatus.Name,
		Created:          timeOrNilFromKubeTime(podStatus.CreationTime),
		StartTime:        timeOrNilFromKubeTime(podStatus.StartTime),
		ContainerStarted: timeOrNilFromKubeTime(podStatus.StartTime),
		EndTime:          timeOrNilFromKubeTime(podStatus.EndTime),
		Status:           ReplicaStatus{Status: string(podStatus.Phase)},
		StatusMessage:    podStatus.Message,
		RestartCount:     podStatus.RestartCount,
		Image:            podStatus.Image,
		ImageId:          podStatus.ImageID,
		PodIndex:         podStatus.PodIndex,
		ExitCode:         podStatus.ExitCode,
		Reason:           podStatus.Reason,
	}
}

func getBatchStatusEnumFromRadixBatchStatusCondition(condition radixv1.RadixBatchCondition) BatchStatusEnum {
	switch condition.Type {
	case radixv1.BatchConditionTypeWaiting:
		return BatchStatusEnumWaiting
	case radixv1.BatchConditionTypeActive:
		return BatchStatusEnumActive
	case radixv1.BatchConditionTypeCompleted:
		return BatchStatusEnumCompleted
	default:
		return BatchStatusEnumWaiting
	}
}

func getBatchStatusEnumFromRadixBatchJobApiStatus(apiStatus radixv1.RadixBatchJobApiStatus) BatchStatusEnum {
	switch apiStatus {
	case radixv1.RadixBatchJobApiStatusWaiting:
		return BatchStatusEnumWaiting
	case radixv1.RadixBatchJobApiStatusActive:
		return BatchStatusEnumActive
	case radixv1.RadixBatchJobApiStatusCompleted:
		return BatchStatusEnumCompleted
	case radixv1.RadixBatchJobApiStatusRunning:
		return BatchStatusEnumRunning
	case radixv1.RadixBatchJobApiStatusSucceeded:
		return BatchStatusEnumSucceeded
	case radixv1.RadixBatchJobApiStatusFailed:
		return BatchStatusEnumFailed
	case radixv1.RadixBatchJobApiStatusStopping:
		return BatchStatusEnumStopping
	case radixv1.RadixBatchJobApiStatusStopped:
		return BatchStatusEnumStopped
	default:
		return BatchStatusEnumWaiting
	}
}

func getJobStatusEnum(jobStatus radixv1.RadixBatchJobStatus, stopJob bool) JobStatusEnum {
	status := JobStatusEnumWaiting
	switch jobStatus.Phase {
	case radixv1.BatchJobPhaseActive:
		status = JobStatusEnumActive
	case radixv1.BatchJobPhaseRunning:
		status = JobStatusEnumRunning
	case radixv1.BatchJobPhaseSucceeded:
		status = JobStatusEnumSucceeded
	case radixv1.BatchJobPhaseFailed:
		status = JobStatusEnumFailed
	case radixv1.BatchJobPhaseStopped:
		status = JobStatusEnumStopped
	case radixv1.BatchJobPhaseWaiting:
		status = JobStatusEnumWaiting
	}

	if stopJob && !status.IsCompleted() {
		status = JobStatusEnumStopping
	}

	return status
}

func findMatchingBatchStatusRule(rb radixv1.RadixBatch, rules []radixv1.BatchStatusRule) (radixv1.BatchStatusRule, bool) {
	jobPhases := slice.Map(rb.Status.JobStatuses, func(js radixv1.RadixBatchJobStatus) radixv1.RadixBatchJobPhase { return js.Phase })

	for _, r := range rules {
		if matchBatchStatusRule(r, jobPhases) {
			return r, true
		}
	}

	return radixv1.BatchStatusRule{}, false
}

func matchBatchStatusRule(rule radixv1.BatchStatusRule, phases []radixv1.RadixBatchJobPhase) bool {
	operatorPredicate := func(phase radixv1.RadixBatchJobPhase) bool {
		if rule.Operator == radixv1.OperatorIn {
			return slices.Contains(rule.JobStatuses, phase)
		}
		return !slices.Contains(rule.JobStatuses, phase)
	}

	if rule.Condition == radixv1.ConditionAll {
		return slice.All(phases, operatorPredicate)
	}
	return slice.Any(phases, operatorPredicate)
}

func timeOrNilFromKubeTime(t *v1.Time) *time.Time {
	if t == nil {
		return nil
	}
	return &t.Time
}
