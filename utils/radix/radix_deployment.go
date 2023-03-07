package radix

import (
	"context"
	"strings"

	apiErrors "github.com/equinor/radix-job-scheduler/api/errors"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRadixDeployJobComponentByName Gets RadixDeployJobComponent by name from the RadixDeployment
func GetRadixDeployJobComponentByName(radixClient radixclient.Interface, namespace, radixDeploymentName, radixJobComponentName string) (*radixv1.RadixDeployJobComponent, error) {
	radixDeployment, err := radixClient.RadixV1().RadixDeployments(namespace).
		Get(context.Background(), radixDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, apiErrors.NewNotFound("radix deployment", radixDeploymentName)
	}
	for _, radixDeployJobComponent := range radixDeployment.Spec.Jobs {
		if strings.EqualFold(radixDeployJobComponent.GetName(), radixJobComponentName) {
			return &radixDeployJobComponent, nil
		}
	}
	return nil, apiErrors.NewNotFound("radix deployment job component", radixJobComponentName)
}