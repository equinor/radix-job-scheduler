package query

import (
	"context"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListRadixBatches Get Radix batches
func ListRadixBatches(ctx context.Context, namespace string, radixClient radixclient.Interface, labels ...map[string]string) ([]radixv1.RadixBatch, error) {
	options := metav1.ListOptions{LabelSelector: radixLabels.Merge(labels...).String()}
	radixBatchList, err := radixClient.RadixV1().RadixBatches(namespace).List(ctx, options)
	if err != nil {
		return nil, err
	}
	return radixBatchList.Items, nil
}

func GetRadixBatch(ctx context.Context, radixClient radixclient.Interface, namespace, batchName string) (*radixv1.RadixBatch, error) {
	return radixClient.RadixV1().RadixBatches(namespace).Get(ctx, batchName, metav1.GetOptions{})
}
