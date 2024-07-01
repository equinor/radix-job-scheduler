package internal

import (
	"context"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	"github.com/equinor/radix-job-scheduler/api/errors"
	modelsv2 "github.com/equinor/radix-job-scheduler/models/v2"
	"github.com/equinor/radix-job-scheduler/pkg/batch"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRadixBatches Get Radix batches
func GetRadixBatches(ctx context.Context, namespace string, radixClient radixclient.Interface, labels ...map[string]string) ([]*radixv1.RadixBatch, error) {
	radixBatchList, err := radixClient.
		RadixV1().
		RadixBatches(namespace).
		List(
			ctx,
			metav1.ListOptions{
				LabelSelector: radixLabels.Merge(labels...).String(),
			},
		)

	if err != nil {
		return nil, errors.NewFromError(err)
	}

	return slice.PointersOf(radixBatchList.Items).([]*radixv1.RadixBatch), nil
}

// GetRadixBatchModelsFromRadixBatches Get Radix batch statuses from Radix batches
func GetRadixBatchModelsFromRadixBatches(radixBatches []*radixv1.RadixBatch, radixDeployJobComponent *radixv1.RadixDeployJobComponent) []*modelsv2.RadixBatch {
	batches := make([]*modelsv2.RadixBatch, 0, len(radixBatches))
	for _, radixBatch := range radixBatches {
		batches = append(batches, pointers.Ptr(batch.GetRadixBatchStatus(radixBatch, radixDeployJobComponent)))
	}
	return batches
}
