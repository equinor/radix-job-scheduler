package batches

import (
	jobs "github.com/equinor/radix-job-scheduler/api/jobs"
	"github.com/equinor/radix-job-scheduler/models"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// GetBatchStatusFromJob Gets job from a k8s jobs for the batch
func GetBatchStatusFromJob(kubeClient kubernetes.Interface, job *v1.Job, jobPods []corev1.Pod) (*models.BatchStatus, error) {
	jobStatus := jobs.GetJobStatusFromJob(kubeClient, job, jobPods)
	batchStatus := models.BatchStatus{
		JobStatus: *jobStatus,
	}
	//TODO 		JobStatuses: nil,
	return &batchStatus, nil
}
