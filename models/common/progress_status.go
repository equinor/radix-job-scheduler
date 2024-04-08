package common

// ProgressStatus Enumeration of the statuses of a job or step
type ProgressStatus int

const (
	// Running Active
	Running ProgressStatus = iota

	// Succeeded JobStatus/step succeeded
	Succeeded

	// Failed JobStatus/step failed
	Failed

	// Waiting JobStatus/step pending
	Waiting

	// Stopping job
	Stopping

	// Stopped job
	Stopped

	// DeadlineExceeded job
	DeadlineExceeded

	// Active job, one or more pods are not ready
	Active

	numStatuses
)

func (p ProgressStatus) String() string {
	if p >= numStatuses {
		return "Unsupported"
	}
	return [...]string{"Running", "Succeeded", "Failed", "Waiting", "Stopping", "Stopped", "DeadlineExceeded", "Active"}[p]
}
