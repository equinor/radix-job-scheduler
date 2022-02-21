package test

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	fake2 "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"os"
	fake3 "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func SetupTest(appName, appEnvironment, appComponent, appDeployment string, historyLimit int) (versioned.Interface,
	kubernetes.Interface, *kube.Kube) {
	os.Setenv("RADIX_APP", appName)
	os.Setenv("RADIX_ENVIRONMENT", appEnvironment)
	os.Setenv("RADIX_COMPONENT", appComponent)
	os.Setenv("RADIX_DEPLOYMENT", appDeployment)
	os.Setenv("RADIX_JOB_SCHEDULERS_PER_ENVIRONMENT_HISTORY_LIMIT", fmt.Sprint(historyLimit))
	os.Setenv(defaults.OperatorEnvLimitDefaultCPUEnvironmentVariable, "200m")
	os.Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "500M")
	kubeclient := fake.NewSimpleClientset()
	radixclient := fake2.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeclient, radixclient, fake3.NewSimpleClientset())
	return radixclient, kubeclient, kubeUtil
}
