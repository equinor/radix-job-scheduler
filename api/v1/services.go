package v1

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

//GetServiceForJob Get the service for the job
func (handler *Handler) GetServiceForJob(jobName string) (*corev1.ServiceList, error) {
	return handler.KubeClient.CoreV1().Services(handler.Env.RadixDeploymentNamespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: getLabelSelectorForService(jobName, handler.Env.RadixComponentName),
		},
	)
}

//DeleteService Deletes a service
func (handler *Handler) DeleteService(service *corev1.Service) error {
	if service == nil {
		return nil
	}
	return handler.KubeClient.CoreV1().Services(service.Namespace).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
}

func getLabelSelectorForService(jobName, componentName string) string {
	return labels.SelectorFromSet(map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobNameLabel:   jobName,
	}).String()
}
