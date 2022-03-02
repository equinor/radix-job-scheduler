package api

import (
	"context"
	"github.com/equinor/radix-job-scheduler/api/errors"
	schedulerDefaults "github.com/equinor/radix-job-scheduler/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	corev1 "k8s.io/api/core/v1"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
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
		jobName := pod.Labels[schedulerDefaults.K8sJobNameLabel]
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
