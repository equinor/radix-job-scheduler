package models

import (
	"os"

	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixv1 "github.com/equinor/radix-operator/pkg/client/clientset/versioned/typed/radix/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeUtil interface {
	KubeClient() *kubernetes.Interface
	RadixV1() *radixv1.RadixV1Interface
	CurrentNamespace() string
}

type kubeUtil struct {
	kubeClient *kubernetes.Interface
	radixv1    *radixv1.RadixV1Interface
	namespace  string
}

func NewKubeUtil(env *Env) KubeUtil {
	config := getInClusterClientConfig()
	kubeClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("failed to create k8s client: %v", err)
	}
	kubeClient := kubernetes.Interface(kubeClientset)

	radixClientset, err := radixclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("failed to create Radix client: %v", err)
	}
	radixV1 := radixClientset.RadixV1()

	return &kubeUtil{
		kubeClient: &kubeClient,
		radixv1:    &radixV1,
		namespace:  env.RadixDeploymentNamespace,
	}
}

func (kube *kubeUtil) KubeClient() *kubernetes.Interface {
	return kube.kubeClient
}

func (kube *kubeUtil) RadixV1() *radixv1.RadixV1Interface {
	return kube.radixv1
}

func (kube *kubeUtil) CurrentNamespace() string {
	return kube.namespace
}

func getInClusterClientConfig() *rest.Config {
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)

	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("getClusterConfig InClusterConfig: %v", err)
		}
	}
	return config
}
