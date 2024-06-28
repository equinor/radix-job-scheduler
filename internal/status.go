package internal

import "github.com/equinor/radix-operator/pkg/apis/radix/v1"

// GetRadixBatchJobsStatusesMap Get map of RadixBatchJobStatuses
func GetRadixBatchJobsStatusesMap(radixBatchJobStatuses []v1.RadixBatchJobStatus) map[string]v1.RadixBatchJobStatus {
	radixBatchJobsStatuses := make(map[string]v1.RadixBatchJobStatus)
	for _, jobStatus := range radixBatchJobStatuses {
		jobStatus := jobStatus
		radixBatchJobsStatuses[jobStatus.Name] = jobStatus
	}
	return radixBatchJobsStatuses
}
