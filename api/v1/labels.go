package v1

import (
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
)

//GetLabelSelectorForBatches Gets a label selector for a batch scheduler job
func GetLabelSelectorForBatches(componentName string) string {
	return labels.Merge(
		labels.ForComponentName(componentName),
		labels.ForBatchScheduleJobType(),
	).String()
}
