package v1

import (
	"github.com/equinor/radix-operator/pkg/apis/kube"
	"k8s.io/apimachinery/pkg/labels"
)

//GetLabelSelectorForBatches Gets a label selector for a batch scheduler job
func GetLabelSelectorForBatches(componentName string) string {
	return labels.SelectorFromSet(map[string]string{
		kube.RadixComponentLabel: componentName,
		kube.RadixJobTypeLabel:   kube.RadixJobTypeBatchSchedule,
	}).String()
}
