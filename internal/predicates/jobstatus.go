package predicates

import (
	modelsv1 "github.com/equinor/radix-job-scheduler/models/v1"
)

func IsJobStatusWithName(name string) func(j modelsv1.JobStatus) bool {
	return func(j modelsv1.JobStatus) bool {
		return j.Name == name
	}
}
