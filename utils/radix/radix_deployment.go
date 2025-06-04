package radix

import (
	"context"
	apiErrors "github.com/equinor/radix-job-scheduler/pkg/errors"
	"strings"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRadixDeployJobComponentByName Gets RadixDeployJobComponent by name from the RadixDeployment
func GetRadixDeployJobComponentByName(ctx context.Context, radixClient radixclient.Interface, namespace, radixDeploymentName, radixJobComponentName string) (*radixv1.RadixDeployJobComponent, error) {
	radixDeployment, err := radixClient.RadixV1().RadixDeployments(namespace).
		Get(ctx, radixDeploymentName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, apiErrors.NewNotFound("radix deployment", radixDeploymentName)
		}
		return nil, apiErrors.NewFromError(err)
	}
	for _, radixDeployJobComponent := range radixDeployment.Spec.Jobs {
		if strings.EqualFold(radixDeployJobComponent.GetName(), radixJobComponentName) {
			return &radixDeployJobComponent, nil
		}
	}
	return nil, apiErrors.NewNotFound("radix deployment job component", radixJobComponentName)
}
