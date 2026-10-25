package cibudgetdomain

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"
)

func recordedReversedSkip(t *testing.T) ClockJob {
	t.Helper()
	raw, err := os.ReadFile("testdata/runner-clock/1805-unassigned-reversed-skip.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct{ Job ClockJob }
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Job.StartedAt == nil || fixture.Job.CompletedAt == nil ||
		!fixture.Job.CompletedAt.Before(*fixture.Job.StartedAt) {
		t.Fatal("original skipped-job timestamp reversal was lost")
	}
	return fixture.Job
}

func TestRecordedUnassignedSkipHasNoRunnerInterval(t *testing.T) {
	job := recordedReversedSkip(t)
	created, now := job.StartedAt.Add(-time.Hour), job.StartedAt.Add(time.Hour)
	for _, conclusion := range []string{"skipped", "cancelled"} {
		variant := job
		variant.Conclusion = conclusion
		if interval, active, err := clockJobInterval(variant, created, now); err != nil || active ||
			interval != (runnerInterval{}) {
			t.Fatal("unassigned observation became an executed interval", conclusion, interval, active, err)
		}
	}
	// Add the observed shape to an independent population, explicitly rebinding
	// its origin and timestamps to that fixture. This is not a live CI receipt.
	run, jobs, bind, observed := recordedClock(t)
	want, err := ObserveRunnerClock(run, jobs, bind, observed)
	if err != nil {
		t.Fatal(err)
	}
	start, end := run.CreatedAt.Add(time.Second), run.CreatedAt
	job.RunID, job.RunAttempt, job.HeadSHA = run.ID, run.RunAttempt, run.HeadSHA
	job.StartedAt, job.CompletedAt = &start, &end
	jobs.Jobs = append(jobs.Jobs, job)
	jobs.TotalCount++
	got, err := ObserveRunnerClock(run, jobs, bind, observed)
	if err != nil || got.ActiveElapsedMS != want.ActiveElapsedMS || got.RemainingMS != want.RemainingMS ||
		got.Decision != want.Decision || !slices.Equal(got.JobIDs, want.JobIDs) ||
		!slices.Equal(got.UnassignedJobIDs, []int64{job.ID}) {
		t.Fatal("skipped observation changed work accounting or disappeared", got, err)
	}
}

func TestUnassignedReversalDoesNotExcuseUnknownOrExecutedWork(t *testing.T) {
	mutations := map[string]func(*ClockJob, time.Time, time.Time){
		"assigned runner": func(j *ClockJob, _, _ time.Time) { id := int64(1); j.RunnerID = &id },
		"observed steps":  func(j *ClockJob, _, _ time.Time) { j.Steps = []struct{}{{}} },
		"unknown steps":   func(j *ClockJob, _, _ time.Time) { j.Steps = nil },
		"executed success": func(j *ClockJob, _, _ time.Time) {
			id := int64(1)
			j.RunnerID, j.Conclusion = &id, "success"
		},
		"before run": func(j *ClockJob, created, _ time.Time) { end := created.Add(-time.Second); j.CompletedAt = &end },
		"after observation": func(j *ClockJob, _, now time.Time) {
			start := now.Add(time.Second)
			j.StartedAt = &start
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			job := recordedReversedSkip(t)
			created, now := job.StartedAt.Add(-time.Hour), job.StartedAt.Add(time.Hour)
			mutate(&job, created, now)
			if _, _, err := clockJobInterval(job, created, now); err == nil {
				t.Fatal("invalid assignment or time admitted")
			}
		})
	}
}
