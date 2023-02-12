package v1

import (
	"context"
	"fmt"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"strings"

	"github.com/equinor/radix-job-scheduler/api/errors"
	defaultsv1 "github.com/equinor/radix-job-scheduler/models/v1/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//GetBatch Gets a job, running a batch
func (handler *Handler) GetBatch(batchName string) (*batchv1.Job, error) {
	batch, err := handler.KubeClient.BatchV1().Jobs(handler.Env.RadixDeploymentNamespace).Get(context.TODO(), batchName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(batch.ObjectMeta.Labels[kube.RadixComponentLabel], handler.Env.RadixComponentName) &&
		strings.EqualFold(batch.ObjectMeta.Labels[kube.RadixJobTypeLabel], kube.RadixJobTypeBatchSchedule) {
		return batch, nil
	}
	return batch, errors.NewNotFound("job", batchName)
}

//GetPodsToJobNameMap Gets a map of pods to K8sJobNameLabel label as keys
func GetPodsToJobNameMap(pods []corev1.Pod) map[string][]corev1.Pod {
	podsMap := make(map[string][]corev1.Pod)
	for _, pod := range pods {
		jobName := pod.Labels[defaultsv1.K8sJobNameLabel]
		if len(jobName) > 0 {
			podsMap[jobName] = append(podsMap[jobName], pod)
		}
	}
	return podsMap
}

//CreateJob Create a job
func (handler *Handler) CreateJob(namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	return handler.KubeClient.BatchV1().Jobs(namespace).Create(context.Background(), job,
		metav1.CreateOptions{})
}

// GetJobStatusFromRadixBatchJobsStatus Get Job status from RadixBatchJob
func GetJobStatusFromRadixBatchJobsStatus(batchName, jobName string, jobStatus modelsv2.RadixBatchJobStatus) modelsv1.JobStatus {
	return modelsv1.JobStatus{
		JobId:     jobStatus.JobId,
		BatchName: batchName,
		Name:      jobName,
		Created:   jobStatus.CreationTime,
		Started:   jobStatus.Started,
		Ended:     jobStatus.Ended,
		Status:    jobStatus.Status,
		Message:   jobStatus.Message,
	}
}

// ComposeSingleJobName from RadixBatchJob
func ComposeSingleJobName(batchName, batchJobName string) string {
	return fmt.Sprintf("%s-%s", batchName, batchJobName)
}

// ParseBatchAndJobNameFromScheduledJobName Decompose V2 batch name and jobs name from V1 job-name
func ParseBatchAndJobNameFromScheduledJobName(scheduleJobName string) (batchName, batchJobName string, ok bool) {
	scheduleJobNameParts := strings.Split(scheduleJobName, "-")
	if len(scheduleJobNameParts) < 2 {
		return
	}
	batchName = strings.Join(scheduleJobNameParts[:len(scheduleJobNameParts)-1], "-")
	batchJobName = scheduleJobNameParts[len(scheduleJobNameParts)-1]
	ok = true
	return
}
