package test

import (
	"fmt"
	"os"

	"github.com/equinor/radix-operator/pkg/apis/defaults"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	radixclientfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	kedafake "github.com/kedacore/keda/v2/pkg/generated/clientset/versioned/fake"
	prometheusclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	prometheusfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"k8s.io/client-go/kubernetes"
	kubeclientfake "k8s.io/client-go/kubernetes/fake"
	secretstoragefake "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

// SetupTest Setup test
func SetupTest(appName, appEnvironment, appComponent, appDeployment string, historyLimit int) (radixclient.Interface,
	kubernetes.Interface, prometheusclient.Interface, *kube.Kube) {
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
	kedaClient := kedafake.NewSimpleClientset()
	prometheusClient := prometheusfake.NewSimpleClientset()
	secretStoreClient := secretstoragefake.NewSimpleClientset()
	kubeUtil, _ := kube.New(kubeClient, radixClient, kedaClient, secretStoreClient)
	return radixClient, kubeClient, prometheusClient, kubeUtil
}
