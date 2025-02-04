package names

import "strings"

// ParseRadixBatchAndJobNameFromFullyQualifiedJobName Extract RadixBatch name and RadixBatchJob name from a fully qualified job name.
//
// For example if fully qualified job name is batch-computerlfeqcqtm-20250124115254-f34haoo4-iz4k1oko, then
// this function will return batch-computerlfeqcqtm-20250124115254-f34haoo4 as batch name, and f34haoo4 as job name
func ParseRadixBatchAndJobNameFromFullyQualifiedJobName(jobStatusName string) (batchName, batchJobName string, ok bool) {
	nameParts := strings.Split(jobStatusName, "-")
	if len(nameParts) < 2 {
		return
	}
	batchName = strings.Join(nameParts[:len(nameParts)-1], "-")
	batchJobName = nameParts[len(nameParts)-1]
	ok = true
	return
}
