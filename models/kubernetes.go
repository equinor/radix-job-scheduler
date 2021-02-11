package models

import (
	"os"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeUtil interface {
	Client() kubernetes.Interface
}

type kubeUtil struct {
	config *rest.Config
}

func NewKubeUtil() KubeUtil {
	return &kubeUtil{
		config: getInClusterClientConfig(),
	}
}

func (kube *kubeUtil) Client() kubernetes.Interface {
	client, err := kubernetes.NewForConfig(kube.config)
	if err != nil {
		log.Fatalf("failed to create k8s client: %v", err)
	}
	return client
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
