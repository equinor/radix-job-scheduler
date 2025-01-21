package internal

import (
	"context"
	"fmt"
	"strings"

	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRadixDeployJobComponentByName Gets RadixDeployJobComponent by name from the RadixDeployment
func GetRadixDeployJobComponentByName(ctx context.Context, radixClient radixclient.Interface, namespace, radixDeploymentName, radixJobComponentName string) (*radixv1.RadixDeployJobComponent, error) {
	radixDeployment, err := radixClient.RadixV1().RadixDeployments(namespace).Get(ctx, radixDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	job, ok := slice.FindFirst(radixDeployment.Spec.Jobs, func(j radixv1.RadixDeployJobComponent) bool {
		return strings.EqualFold(j.GetName(), radixJobComponentName)
	})
	if !ok {
		return nil, fmt.Errorf("job %q not found in deployment %q", radixJobComponentName, radixDeploymentName)
	}
	return &job, nil
}
