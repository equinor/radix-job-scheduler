package internal

import (
	"context"
	"fmt"
	"github.com/equinor/radix-job-scheduler/pkg/errors"
	"math/rand"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixLabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var defaultSrc = rand.NewSource(time.Now().UnixNano())

// GetRadixBatches Get Radix batches
func GetRadixBatches(ctx context.Context, radixClient radixclient.Interface, namespace string, labels ...map[string]string) ([]*radixv1.RadixBatch, error) {
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
		return nil, err
	}

	return slice.PointersOf(radixBatchList.Items).([]*radixv1.RadixBatch), nil
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

// GetRadixBatch Get Radix batch
func GetRadixBatch(ctx context.Context, radixClient radixclient.Interface, namespace, batchName string) (*radixv1.RadixBatch, error) {
	radixBatch, err := radixClient.RadixV1().RadixBatches(namespace).Get(ctx, batchName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.NewFromError(err)
	}
	return radixBatch, nil
}

// ParseBatchAndJobNameFromScheduledJobName Decompose V2 batch name and jobs name from V1 job-name
func ParseBatchAndJobNameFromScheduledJobName(scheduleJobName string) (batchName, batchJobName string, ok bool) {
	scheduleJobNameParts := strings.Split(scheduleJobName, "-")
	if len(scheduleJobNameParts) < 2 {
		return
	}
	batchName = strings.Join(scheduleJobNameParts[:len(scheduleJobNameParts)-1], "-")
	batchJobName = scheduleJobNameParts[len(scheduleJobNameParts)-1]
	ok = true
	return
}

// IsRadixBatchSucceeded Check if Radix batch is succeeded
func IsRadixBatchSucceeded(batch *radixv1.RadixBatch) bool {
	return batch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted && slice.All(batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return IsRadixBatchJobSucceeded(jobStatus)
	})
}

// IsRadixBatchJobSucceeded Check if Radix batch job is succeeded
func IsRadixBatchJobSucceeded(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseSucceeded || jobStatus.Phase == radixv1.BatchJobPhaseStopped
}

// IsRadixBatchJobFailed Check if Radix batch job is failed
func IsRadixBatchJobFailed(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseFailed
}

// IsRadixBatchNotSucceeded Check if Radix batch is not succeeded
func IsRadixBatchNotSucceeded(batch *radixv1.RadixBatch) bool {
	return batch.Status.Condition.Type == radixv1.BatchConditionTypeCompleted && slice.Any(batch.Status.JobStatuses, func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return !IsRadixBatchJobSucceeded(jobStatus)
	})
}

// GetStatusFromStatusRules Gets BatchStatus by rules
func GetStatusFromStatusRules(radixBatchJobPhases []radixv1.RadixBatchJobPhase, activeRadixDeployJobComponent *radixv1.RadixDeployJobComponent, defaultBatchStatus radixv1.RadixBatchJobApiStatus) radixv1.RadixBatchJobApiStatus {
	if activeRadixDeployJobComponent != nil {
		for _, rule := range activeRadixDeployJobComponent.BatchStatusRules {
			ruleJobStatusesMap := slice.Reduce(rule.JobStatuses, make(map[radixv1.RadixBatchJobPhase]struct{}), func(acc map[radixv1.RadixBatchJobPhase]struct{}, jobStatus radixv1.RadixBatchJobPhase) map[radixv1.RadixBatchJobPhase]struct{} {
				acc[jobStatus] = struct{}{}
				return acc
			})
			evaluateJobStatusByRule := func(jobStatusPhase radixv1.RadixBatchJobPhase) bool {
				return evaluateCondition(jobStatusPhase, rule.Operator, ruleJobStatusesMap)
			}
			switch rule.Condition {
			case radixv1.ConditionAny:
				if slice.Any(radixBatchJobPhases, evaluateJobStatusByRule) {
					return rule.BatchStatus
				}
			case radixv1.ConditionAll:
				if slice.All(radixBatchJobPhases, evaluateJobStatusByRule) {
					return rule.BatchStatus
				}
			}
		}
	}
	if len(defaultBatchStatus) == 0 {
		return radixv1.RadixBatchJobApiStatusWaiting
	}
	return defaultBatchStatus
}

func evaluateCondition(jobStatus radixv1.RadixBatchJobPhase, ruleOperator radixv1.Operator, ruleJobStatusesMap map[radixv1.RadixBatchJobPhase]struct{}) bool {
	_, statusExistInRule := ruleJobStatusesMap[jobStatus]
	return (ruleOperator == radixv1.OperatorNotIn && !statusExistInRule) || (ruleOperator == radixv1.OperatorIn && statusExistInRule)
}
