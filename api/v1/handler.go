package v1

import (
	"github.com/equinor/radix-job-scheduler/models"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

type Handler struct {
	Kube        *kube.Kube
	Env         *models.Env
	KubeClient  kubernetes.Interface
	RadixClient versioned.Interface
}
