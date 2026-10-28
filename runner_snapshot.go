package cibudgetdomain

import "fmt"

// A pending snapshot never admits work or supplies a partial clock. A bounded
// reader may obtain another observation of the same source/run/attempt.
type ClockSnapshotPendingError struct {
	Job    string
	Status string
}

func (err *ClockSnapshotPendingError) Error() string {
	return fmt.Sprintf("runner job %q has no execution-ready snapshot (status=%q)", err.Job, err.Status)
}

func validateClockJobIdentities(jobs []ClockJob, bind ClockBinding) error {
	seen := map[int64]bool{}
	for _, job := range jobs {
		if job.ID <= 0 || seen[job.ID] || job.RunID != bind.RunID || job.RunAttempt != bind.Attempt || job.HeadSHA != bind.Head {
			return fmt.Errorf("runner clock job %q has a duplicate or different source origin", job.Name)
		}
		seen[job.ID] = true
	}
	return nil
}
