package test

import (
	"fmt"
	"os"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixclientfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes"
	kubeclientfake "k8s.io/client-go/kubernetes/fake"
)

// SetupTest Setup test
func SetupTest(appName, appEnvironment, appComponent, appDeployment string, historyLimit int) (radixclient.Interface, kubernetes.Interface, *kube.Kube) {
	_ = os.Setenv("RADIX_APP", appName)
	_ = os.Setenv("RADIX_ENVIRONMENT", appEnvironment)
	_ = os.Setenv("RADIX_COMPONENT", appComponent)
	_ = os.Setenv("RADIX_DEPLOYMENT", appDeployment)
	_ = os.Setenv("RADIX_JOB_SCHEDULERS_PER_ENVIRONMENT_HISTORY_LIMIT", fmt.Sprint(historyLimit))
	_ = os.Setenv(defaults.OperatorRollingUpdateMaxUnavailable, "25%")
	_ = os.Setenv(defaults.OperatorRollingUpdateMaxSurge, "25%")
	_ = os.Setenv(defaults.OperatorEnvLimitDefaultMemoryEnvironmentVariable, "500M")
	kubeClient := kubeclientfake.NewSimpleClientset()
	radixClient := radixclientfake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeClient, radixClient, nil, nil)
	return radixClient, kubeClient, kubeUtil
}
