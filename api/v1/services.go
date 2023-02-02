package v1

import (
	"context"
	defaultsV1 "github.com/equinor/radix-job-scheduler/models/v1/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
)

//CreateService Create a service for the job API
func (handler *Handler) CreateService(appName, jobName string, jobComponent *v1.RadixDeployJobComponent) (*corev1.
	Service, error) {
	if len(jobComponent.GetPorts()) == 0 {
		return nil, nil
	}
	serviceName := jobName
	service := buildServiceSpec(serviceName, jobName, jobComponent.Name, appName, jobComponent.GetPorts())
	err := handler.Kube.ApplyService(handler.Env.RadixDeploymentNamespace, service)
	if err != nil {
		return nil, err
	}
	return service, nil

}

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

func buildServiceSpec(serviceName, jobName, componentName, appName string, componentPorts []v1.ComponentPort) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
			Labels: map[string]string{
				kube.RadixAppLabel:       appName,
				kube.RadixComponentLabel: componentName,
				kube.RadixJobTypeLabel:   kube.RadixJobTypeJobSchedule,
				kube.RadixJobNameLabel:   jobName,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeClusterIP,
			Ports: buildServicePorts(componentPorts),
			Selector: map[string]string{
				defaultsV1.K8sJobNameLabel: jobName, // K8s adds a "job-name" label to a Pod created from a Job
			},
		},
	}

	return service
}

func buildServicePorts(componentPorts []v1.ComponentPort) []corev1.ServicePort {
	var ports []corev1.ServicePort
	for _, v := range componentPorts {
		servicePort := corev1.ServicePort{
			Name:       v.Name,
			Port:       v.Port,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(int(v.Port)),
		}
		ports = append(ports, servicePort)
	}
	return ports
}
