package batchesv1

import (
	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-job-scheduler/api/v1/jobs"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// GetBatchStatusFromRadixBatch Gets batch status from RadixBatch
func GetBatchStatusFromRadixBatch(radixBatch *radixv1.RadixBatch) *modelsv1.BatchStatus {
	jobStatus := modelsv1.BatchStatus{
		JobStatus: modelsv1.JobStatus{
			BatchName: radixBatch.GetName(),
			Created:   utils.FormatTime(&radixBatch.ObjectMeta.CreationTimestamp),
			Started:   utils.FormatTime(radixBatch.Status.Condition.ActiveTime),
			Ended:     utils.FormatTime(radixBatch.Status.Condition.CompletionTime),
			Status:    radixBatch.Status.Condition.Reason, //TODO ?
			Message:   radixBatch.Status.Condition.Message,
		},
	}
	return &jobStatus
}

// GetBatchStatusFromRadixBatchStatus Gets batch status from RadixBatchStatus
func GetBatchStatusFromRadixBatchStatus(batchName string, radixBatchStatus *radixv1.RadixBatchStatus) *modelsv1.BatchStatus {
	jobStatus := modelsv1.BatchStatus{
		JobStatus: modelsv1.JobStatus{
			BatchName: batchName,
			//Created:   utils.FormatTime(&radixBatchStatus.Condition..CreationTimestamp),  TODO
			Started: utils.FormatTime(radixBatchStatus.Condition.ActiveTime),
			Ended:   utils.FormatTime(radixBatchStatus.Condition.CompletionTime),
			Status:  radixBatchStatus.Condition.Reason, //TODO ?
			Message: radixBatchStatus.Condition.Message,
		},
	}
	return &jobStatus
}

// GetBatchStatusFromRadixBatchStatusModel Gets batch status from RadixBatchStatus model
func GetBatchStatusFromRadixBatchStatusModel(radixBatchStatus *modelsv2.RadixBatchStatus) *modelsv1.BatchStatus {
	jobStatus := modelsv1.BatchStatus{
		JobStatus: modelsv1.JobStatus{
			BatchName: radixBatchStatus.Name,
			Created:   radixBatchStatus.CreationTime,
			Started:   utils.FormatTime(radixBatchStatus.Status.Condition.ActiveTime),
			Ended:     utils.FormatTime(radixBatchStatus.Status.Condition.CompletionTime),
			Status:    radixBatchStatus.Status.Condition.Reason, //TODO ?
			Message:   radixBatchStatus.Status.Condition.Message,
		},
	}
	return &jobStatus
}

// GetBatchStatusFromJob Gets job from a k8s jobs for the batch
func GetBatchStatusFromJob(kubeClient kubernetes.Interface, job *v1.Job, jobPods []corev1.Pod) (*modelsv1.BatchStatus, error) {
	batchJobStatus := jobs.GetJobStatusFromJob(kubeClient, job, jobPods)
	batchJobStatus.BatchName = job.GetName()
	batchStatus := modelsv1.BatchStatus{
		JobStatus: *batchJobStatus,
	}
	//TODO 		JobStatuses: nil,
	return &batchStatus, nil
}
