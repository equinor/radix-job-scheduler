package job

import (
	"context"

	"github.com/equinor/radix-operator/pkg/apis/kube"
	v1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	k8sJobNameLabel = "job-name" // A label that k8s automatically adds to a Pod created by a Job
)

func (jh *jobHandler) createService(jobName string, jobComponent *v1.RadixDeployJobComponent, rd *v1.RadixDeployment) error {
	if len(jobComponent.GetPorts()) > 0 {
		serviceName := jobName
		service := buildServiceSpec(serviceName, jobName, jobComponent.Name, rd.Spec.AppName, jobComponent.GetPorts())
		return jh.kube.ApplyService(jh.env.RadixDeploymentNamespace, service)
	}
	return nil
}

func (jh *jobHandler) getServiceForJob(jobName string) (*corev1.ServiceList, error) {
	return jh.kubeClient.CoreV1().Services(jh.env.RadixDeploymentNamespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: getLabelSelectorForService(jobName, jh.env.RadixComponentName),
		},
	)
}

func (jh *jobHandler) deleteService(service *corev1.Service) error {
	return jh.kubeClient.CoreV1().Services(service.Namespace).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
}

func getLabelSelectorForService(jobName, componentName string) string {
	return labels.SelectorFromSet(labels.Set(map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobNameLabel:   jobName,
	})).String()
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
				k8sJobNameLabel: jobName, // K8s adds a "job-name" label to a Pod created from a Job
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
			Port:       int32(v.Port),
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(int(v.Port)),
		}
		ports = append(ports, servicePort)
	}
	return ports
}
