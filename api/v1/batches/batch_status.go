package batchesv1

import (
	"github.com/equinor/radix-job-scheduler/api/v1/jobs"
	v12 "github.com/equinor/radix-job-scheduler/models/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// GetBatchStatusFromJob Gets job from a k8s jobs for the batch
func GetBatchStatusFromJob(kubeClient kubernetes.Interface, job *v1.Job, jobPods []corev1.Pod) (*v12.BatchStatus, error) {
	batchJobStatus := jobs.GetJobStatusFromJob(kubeClient, job, jobPods)
	batchJobStatus.BatchName = job.GetName()
	batchStatus := v12.BatchStatus{
		JobStatus: *batchJobStatus,
	}
	//TODO 		JobStatuses: nil,
	return &batchStatus, nil
}
