package names

import "strings"

// ParseRadixBatchAndJobNameFromJobStatusName Extract RadixBatch and RadixBatchJob names from JobStatus name
func ParseRadixBatchAndJobNameFromJobStatusName(jobStatusName string) (batchName, batchJobName string, ok bool) {
	nameParts := strings.Split(jobStatusName, "-")
	if len(nameParts) < 2 {
		return
	}
	batchName = strings.Join(nameParts[:len(nameParts)-1], "-")
	batchJobName = nameParts[len(nameParts)-1]
	ok = true
	return
}
