package job

import batchv1 "k8s.io/api/batch/v1"

type jobList []*batchv1.Job
type wherePredicateFunc func(*batchv1.Job) bool

func (jl jobList) Where(predicate wherePredicateFunc) jobList {
	var filteredList jobList
	for _, j := range jl {
		if predicate(j) {
			filteredList = append(filteredList, j)
		}
	}
	return filteredList
}
