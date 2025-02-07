package predicates

import radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

func IsRadixBatchJobWithName(name string) func(j radixv1.RadixBatchJob) bool {
	return func(j radixv1.RadixBatchJob) bool {
		return j.Name == name
	}
}
