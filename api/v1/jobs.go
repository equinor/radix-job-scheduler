package v1

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-job-scheduler/api/errors"
	apiv2 "github.com/equinor/radix-job-scheduler/api/v2"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	defaultsv1 "github.com/equinor/radix-job-scheduler/models/v1/defaults"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetBatch Gets a job, running a batch
func (handler *Handler) GetBatch(ctx context.Context, batchName string) (*batchv1.Job, error) {
	batch, err := handler.Kube.KubeClient().BatchV1().Jobs(handler.Env.RadixDeploymentNamespace).Get(ctx, batchName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(batch.ObjectMeta.Labels[kube.RadixComponentLabel], handler.Env.RadixComponentName) &&
		strings.EqualFold(batch.ObjectMeta.Labels[kube.RadixJobTypeLabel], kube.RadixJobTypeBatchSchedule) {
		return batch, nil
	}
	return batch, errors.NewNotFound("job", batchName)
}

// GetPodsToJobNameMap Gets a map of pods to K8sJobNameLabel label as keys
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

// GetJobStatusFromRadixBatchJobsStatus Get Job status from RadixBatchJob
func GetJobStatusFromRadixBatchJobsStatus(batchName string, jobStatus modelsv2.RadixBatchJobStatus) modelsv1.JobStatus {
	return modelsv1.JobStatus{
		JobId:     jobStatus.JobId,
		BatchName: batchName,
		Name:      jobStatus.Name,
		Created:   jobStatus.CreationTime,
		Started:   jobStatus.Started,
		Ended:     jobStatus.Ended,
		Status:    jobStatus.Status,
		Message:   jobStatus.Message,
	}
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

// GetJobStatusFromRadixBatchJobsStatuses Get JobStatuses from RadixBatch job statuses v2
func GetJobStatusFromRadixBatchJobsStatuses(radixBatches ...modelsv2.RadixBatch) []modelsv1.JobStatus {
	var jobStatuses []modelsv1.JobStatus
	for _, radixBatch := range radixBatches {
		jobStatusBatchName := getJobStatusBatchName(&radixBatch)
		for _, jobStatus := range radixBatch.JobStatuses {
			jobStatuses = append(jobStatuses, GetJobStatusFromRadixBatchJobsStatus(jobStatusBatchName, jobStatus))
		}
	}
	return jobStatuses
}

// StopJob Stop a job
func StopJob(ctx context.Context, handlerApiV2 apiv2.Handler, jobName string) error {
	if batchName, jobName, ok := ParseBatchAndJobNameFromScheduledJobName(jobName); ok {
		return handlerApiV2.StopRadixBatchJob(ctx, batchName, jobName)
	}
	return fmt.Errorf("stop of this job is not supported")
}

// GetBatchJob Get batch job
func GetBatchJob(ctx context.Context, handlerApiV2 apiv2.Handler, batchName, jobName string) (*modelsv1.JobStatus, error) {
	radixBatch, err := handlerApiV2.GetRadixBatch(ctx, batchName)
	if err != nil {
		return nil, err
	}
	jobStatusBatchName := getJobStatusBatchName(radixBatch)
	for _, jobStatus := range radixBatch.JobStatuses {
		if !strings.EqualFold(jobStatus.Name, jobName) {
			continue
		}
		jobsStatus := GetJobStatusFromRadixBatchJobsStatus(jobStatusBatchName, jobStatus)
		return &jobsStatus, nil
	}
	return nil, fmt.Errorf("not found")
}

func getJobStatusBatchName(radixBatch *modelsv2.RadixBatch) string {
	return utils.TernaryString(radixBatch.BatchType == string(kube.RadixBatchTypeJob), "", radixBatch.Name)
}
