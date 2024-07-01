package internal

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils"
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

var defaultSrc = rand.NewSource(time.Now().UnixNano())

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

// GenerateBatchName Generate batch name
func GenerateBatchName(jobComponentName string) string {
	timestamp := time.Now().Format("20060102150405")
	return fmt.Sprintf("batch-%s-%s-%s", getJobComponentNamePart(jobComponentName), timestamp, strings.ToLower(utils.RandString(8)))
}

func getJobComponentNamePart(jobComponentName string) string {
	componentNamePart := jobComponentName
	if len(componentNamePart) > 12 {
		componentNamePart = componentNamePart[:12]
	}
	return fmt.Sprintf("%s%s", componentNamePart, strings.ToLower(utils.RandString(16-len(componentNamePart))))
}

// CreateJobName create a job name
func CreateJobName() string {
	return strings.ToLower(utils.RandStringSeed(8, defaultSrc))
}
