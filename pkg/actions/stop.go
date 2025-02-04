package actions

import (
	"errors"

	"github.com/equinor/radix-common/utils/pointers"
	"github.com/equinor/radix-common/utils/slice"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
)

var (
	ErrStopCompletedRadixBatch = errors.New("batch is completed")
	ErrStopJobNotFound         = errors.New("job not found")
)

// StopAllRadixBatchJobs Returns a RadixBatch where stop is set to true for all jobs that are not completed.
// The original RadixBatch is not modified.
// Returns a ErrStopCompletedRadixBatch error if the RadixBatch condition is "Completed".
func StopAllRadixBatchJobs(rb *radixv1.RadixBatch) (*radixv1.RadixBatch, error) {
	return stopRadixBatchJobs(rb, &stopMatchAllJobs{})
}

// StopNamedRadixBatchJob Returns a RadixBatch where stop is set to true for the named job.
// The original RadixBatch is not modified.
// Returns a ErrStopCompletedRadixBatch error if the RadixBatch condition is "Completed",
// and ErrStopJobNotFound if the job does not exist.
func StopNamedRadixBatchJob(rb *radixv1.RadixBatch, jobName string) (*radixv1.RadixBatch, error) {
	matcher := stopMatchNamedJob{name: jobName}
	updatedRb, err := stopRadixBatchJobs(rb, &matcher)
	if err != nil {
		return nil, err
	}
	if !matcher.matched {
		return nil, ErrStopJobNotFound
	}
	return updatedRb, nil
}

type stopJobMatcher interface {
	match(j radixv1.RadixBatchJob) bool
}

type stopMatchAllJobs struct{}

func (*stopMatchAllJobs) match(radixv1.RadixBatchJob) bool { return true }

type stopMatchNamedJob struct {
	name    string
	matched bool
}

func (m *stopMatchNamedJob) match(j radixv1.RadixBatchJob) bool {
	if match := j.Name == m.name; !match {
		return false
	}
	m.matched = true
	return true
}

func stopRadixBatchJobs(rb *radixv1.RadixBatch, jobMatcher stopJobMatcher) (*radixv1.RadixBatch, error) {
	if rb.Status.Condition.Type == radixv1.BatchConditionTypeCompleted {
		return nil, ErrStopCompletedRadixBatch
	}

	rbCopy := rb.DeepCopy()
	for idx, job := range rbCopy.Spec.Jobs {
		if !jobMatcher.match(job) {
			continue
		}
		status, _ := slice.FindFirst(rbCopy.Status.JobStatuses, isRadixBatchJobStatusWithName(job.Name))
		if isJobStatusDone(status) {
			continue
		}
		rbCopy.Spec.Jobs[idx].Stop = pointers.Ptr(true)
	}

	return rbCopy, nil
}

func isRadixBatchJobStatusWithName(name string) func(j radixv1.RadixBatchJobStatus) bool {
	return func(j radixv1.RadixBatchJobStatus) bool {
		return j.Name == name
	}
}

func isJobStatusDone(jobStatus radixv1.RadixBatchJobStatus) bool {
	return jobStatus.Phase == radixv1.BatchJobPhaseSucceeded ||
		jobStatus.Phase == radixv1.BatchJobPhaseFailed ||
		jobStatus.Phase == radixv1.BatchJobPhaseStopped
}
