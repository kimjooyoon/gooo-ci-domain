package cibudgetdomain

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestQueuedAndUnassignedCancellationDoNotSpendRunnerTime(t *testing.T) {
	raw, err := os.ReadFile("testdata/runner-clock/1789-unassigned-cancellation.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Jobs []ClockJob `json:"jobs"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	created, _ := time.Parse(time.RFC3339, "2026-10-25T00:19:18Z")
	now := created.Add(10 * time.Minute)
	for _, job := range fixture.Jobs {
		if _, active, err := clockJobInterval(job, created, now); err != nil || active {
			t.Fatal(job.Name, active, err)
		}
	}
	// Reproduce the queued preparation tuple observed before cancellation.
	queued := fixture.Jobs[0]
	queued.Status, queued.Conclusion, queued.CompletedAt = "queued", "", nil
	if queued.StartedAt == nil {
		t.Fatal("counterexample lost its populated queued timestamp")
	}
	if _, active, err := clockJobInterval(queued, created, now); err != nil || active {
		t.Fatal(active, err)
	}
}

func TestUnknownRunnerAssignmentCannotBecomeFreeTime(t *testing.T) {
	_, jobs, _, now := recordedClock(t)
	created := now.Add(-2 * time.Hour)
	for _, variant := range []string{"missing runner", "queued with runner", "cancelled with steps", "cancelled unknown"} {
		t.Run(variant, func(t *testing.T) {
			job := jobs.Jobs[0]
			switch variant {
			case "missing runner":
				job.RunnerID = nil
			case "queued with runner":
				job.Status = "queued"
				job.CompletedAt = nil
			case "cancelled with steps":
				zero := int64(0)
				job.RunnerID = &zero
				job.Conclusion = "cancelled"
				job.Steps = []struct{}{{}}
			case "cancelled unknown":
				job.RunnerID = nil
				job.Conclusion = "cancelled"
			}
			if _, _, err := clockJobInterval(job, created, now); err == nil {
				t.Fatal("unknown assignment was accepted")
			}
		})
	}
}
