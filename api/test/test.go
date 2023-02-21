package test

import (
	"context"
	"fmt"
	"strings"

	apiv1 "github.com/equinor/radix-job-scheduler/api/v1"
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func AddRadixBatch(radixClient versioned.Interface, jobName, componentName string, batchJobType kube.RadixBatchType, namespace string) *v1.RadixBatch {
	labels := make(map[string]string)

	if len(strings.TrimSpace(componentName)) > 0 {
		labels[kube.RadixComponentLabel] = componentName
	}
	labels[kube.RadixBatchTypeLabel] = string(batchJobType)

	batchName, batchJobName, ok := apiv1.ParseBatchAndJobNameFromScheduledJobName(jobName)
	if !ok {
		panic(fmt.Sprintf("invalid job name %s", jobName))
	}
	radixBatch, err := radixClient.RadixV1().RadixBatches(namespace).Create(
		context.TODO(),
		&v1.RadixBatch{
			ObjectMeta: metav1.ObjectMeta{
				Name:   batchName,
				Labels: labels,
			},
			Spec: v1.RadixBatchSpec{
				Jobs: []v1.RadixBatchJob{
					{
						Name: batchJobName,
						PayloadSecretRef: &v1.PayloadSecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: jobName},
							Key:                  jobName,
						},
					},
				},
			},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		panic(err)
	}
	return radixBatch
}

func CreateSecretForTest(appName, secretName, jobName, radixJobComponentName, namespace string, kubeClient kubernetes.Interface) {
	batchName, batchJobName, _ := apiv1.ParseBatchAndJobNameFromScheduledJobName(jobName)
	_, err := kubeClient.CoreV1().Secrets(namespace).Create(
		context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
				Labels: radixLabels.Merge(
					radixLabels.ForApplicationName(appName),
					radixLabels.ForComponentName(radixJobComponentName),
					radixLabels.ForBatchName(batchName),
				),
			},
			Data: map[string][]byte{batchJobName: []byte("secret")},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		panic(err)
	}
}

func GetJobStatusByNameForTest(jobs []modelsv1.JobStatus, name string) *modelsv1.JobStatus {
	for _, job := range jobs {
		if strings.HasSuffix(job.Name, "-"+name) {
			return &job
		}
	}
	return nil
}

func GetSecretByNameForTest(secrets []corev1.Secret, name string) *corev1.Secret {
	for _, secret := range secrets {
		if secret.Name == name {
			return &secret
		}
	}
	return nil
}

func GetRadixBatchByNameForTest(radixBatches []v1.RadixBatch, jobName string) *v1.RadixBatch {
	batchName, _, _ := apiv1.ParseBatchAndJobNameFromScheduledJobName(jobName)
	for _, radixBatch := range radixBatches {
		if radixBatch.Name == batchName {
			return &radixBatch
		}
	}
	return nil
}
