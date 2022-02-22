package models

import batchv1 "k8s.io/api/batch/v1"

type JobList []*batchv1.Job
type wherePredicateFunc func(*batchv1.Job) bool

func (jl JobList) Where(predicate wherePredicateFunc) JobList {
	var filteredList JobList
	for _, j := range jl {
		if predicate(j) {
			filteredList = append(filteredList, j)
		}
	}
	return filteredList
}
