package radix

import (
	"context"

	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	"github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRadixApplicationByName Gets RadixApplication by name
func GetRadixApplicationByName(radixClient radixclient.Interface, appName string) (*radixv1.RadixApplication, error) {
	namespace := utils.GetAppNamespace(appName)
	radixApplication, err := radixClient.RadixV1().RadixApplications(namespace).
		Get(context.Background(), appName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, apiErrors.NewNotFound("radix application", appName)
		}
		return nil, apiErrors.NewUnknown(err)
	}
	return radixApplication, nil
}
