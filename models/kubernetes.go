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
	CurrentNamespace() string
}

type kubeUtil struct {
	config    *rest.Config
	namespace string
}

func NewKubeUtil(env *Env) KubeUtil {
	return &kubeUtil{
		config:    getInClusterClientConfig(),
		namespace: env.RadixDeploymentNamespace,
	}
}

func (kube *kubeUtil) Client() kubernetes.Interface {
	client, err := kubernetes.NewForConfig(kube.config)
	if err != nil {
		log.Fatalf("failed to create k8s client: %v", err)
	}
	return client
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
