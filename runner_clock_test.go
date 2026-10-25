package cibudgetdomain

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func recordedClock(t *testing.T) (ClockRun, ClockJobs, ClockBinding, time.Time) {
	t.Helper()
	raw, err := os.ReadFile("testdata/runner-clock/1786-at-proof-bind.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Run  ClockRun  `json:"run"`
		Jobs ClockJobs `json:"jobs"`
		Now  time.Time `json:"observed_at"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	bind := ClockBinding{Head: fixture.Run.HeadSHA, RunID: fixture.Run.ID, Attempt: 1,
		CurrentJob: "CI proof bundle", ProducerJob: "Work domain preparation", BudgetMS: 3000000}
	return fixture.Run, fixture.Jobs, bind, fixture.Now
}

func TestRecordedRunnerQueueDoesNotErasePassedChecks(t *testing.T) {
	run, jobs, bind, now := recordedClock(t)
	report, err := ObserveRunnerClock(run, jobs, bind, now)
	if err != nil {
		t.Fatal(err)
	}
	if report.Decision != "READY" || report.ActiveElapsedMS != 2334000 || report.RemainingMS != 666000 ||
		report.WallElapsedMS != 3690000 || report.RunnerFreeMS != 1356000 {
		t.Fatalf("original attempt accounting differs: %+v", report)
	}
	oldStart, _ := time.Parse(time.RFC3339, "2026-10-24T22:51:38Z")
	if now.Sub(oldStart).Milliseconds() != 3294000 {
		t.Fatal("old shared clock was not exhausted")
	}
	// This domain decision admits bounded work. It never certifies any check.
	if len(report.JobIDs) != 12 {
		t.Fatal("observed job population changed", report.JobIDs)
	}
	t.Logf("wall=%d active=%d runner_free=%d remaining=%d", report.WallElapsedMS,
		report.ActiveElapsedMS, report.RunnerFreeMS, report.RemainingMS)
}

func TestRunnerClockDoesNotResetAcrossSequentialStages(t *testing.T) {
	run, _, bind, _ := recordedClock(t)
	start := run.CreatedAt
	makeJob := func(id int64, name, status string, from, into int) ClockJob {
		a, b := start.Add(time.Duration(from)*time.Minute), start.Add(time.Duration(into)*time.Minute)
		runner := id + 100
		j := ClockJob{ID: id, RunID: bind.RunID, RunAttempt: 1, HeadSHA: bind.Head, Name: name,
			Status: status, Conclusion: "success", StartedAt: &a, CompletedAt: &b, RunnerID: &runner}
		if status == "in_progress" {
			j.CompletedAt = nil
			j.Conclusion = ""
		}
		return j
	}
	cases := []struct {
		name                                   string
		producerEnd, currentStart, now, active int
		decision                               string
	}{
		{"parallel", 40, 20, 40, 40, "READY"},
		{"idle gap", 20, 40, 60, 40, "READY"},
		{"sequential exhaustion", 20, 40, 80, 60, "DEADLINE_EXCEEDED"},
		{"exact ceiling", 20, 30, 60, 50, "DEADLINE_EXCEEDED"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			jobs := ClockJobs{TotalCount: 2, Jobs: []ClockJob{
				makeJob(1, bind.ProducerJob, "completed", 0, c.producerEnd),
				makeJob(2, bind.CurrentJob, "in_progress", c.currentStart, c.now),
			}}
			r, err := ObserveRunnerClock(run, jobs, bind, start.Add(time.Duration(c.now)*time.Minute))
			if err != nil || r.ActiveElapsedMS != int64(c.active)*60000 || r.Decision != c.decision {
				t.Fatal(r, err)
			}
		})
	}
}

func TestRunnerClockRejectsIncompleteOrForeignOrigins(t *testing.T) {
	mutations := map[string]func(*ClockRun, *ClockJobs, *ClockBinding, time.Time){
		"truncated population": func(_ *ClockRun, j *ClockJobs, _ *ClockBinding, _ time.Time) { j.TotalCount++ },
		"other attempt":        func(_ *ClockRun, j *ClockJobs, _ *ClockBinding, _ time.Time) { j.Jobs[0].RunAttempt++ },
		"other run":            func(r *ClockRun, _ *ClockJobs, _ *ClockBinding, _ time.Time) { r.ID++ },
		"other head":           func(_ *ClockRun, j *ClockJobs, _ *ClockBinding, _ time.Time) { j.Jobs[0].HeadSHA = "foreign" },
		"duplicate id":         func(_ *ClockRun, j *ClockJobs, _ *ClockBinding, _ time.Time) { j.Jobs[1].ID = j.Jobs[0].ID },
		"duplicate current":    func(_ *ClockRun, j *ClockJobs, b *ClockBinding, _ time.Time) { j.Jobs[1].Name = b.CurrentJob },
		"missing producer":     func(_ *ClockRun, _ *ClockJobs, b *ClockBinding, _ time.Time) { b.ProducerJob = "missing" },
		"future completion": func(_ *ClockRun, j *ClockJobs, _ *ClockBinding, n time.Time) {
			x := n.Add(time.Second)
			j.Jobs[0].CompletedAt = &x
		},
		"missing start": func(_ *ClockRun, j *ClockJobs, _ *ClockBinding, _ time.Time) { j.Jobs[0].StartedAt = nil },
		"reversed interval": func(_ *ClockRun, j *ClockJobs, _ *ClockBinding, _ time.Time) {
			j.Jobs[0].CompletedAt = j.Jobs[0].StartedAt
			x := j.Jobs[0].StartedAt.Add(time.Second)
			j.Jobs[0].StartedAt = &x
		},
		"unknown status": func(_ *ClockRun, j *ClockJobs, _ *ClockBinding, _ time.Time) { j.Jobs[0].Status = "unknown" },
		"zero budget":    func(_ *ClockRun, _ *ClockJobs, b *ClockBinding, _ time.Time) { b.BudgetMS = 0 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			r, j, b, n := recordedClock(t)
			mutate(&r, &j, &b, n)
			if out, err := ObserveRunnerClock(r, j, b, n); err == nil || out.Schema != "" {
				t.Fatal("invalid origin became a clock", out, err)
			}
		})
	}
}

// Independent discrete occupancy oracle: mark seconds, rather than merging intervals.
func TestRunnerUnionMatchesOccupiedSeconds(t *testing.T) {
	base := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	for sample := range 256 {
		var occupied [128]bool
		intervals := []runnerInterval{}
		for k := range 16 {
			from := (sample*17 + k*13) % 100
			into := from + (sample+k*7)%28
			intervals = append(intervals, runnerInterval{base.Add(time.Duration(from) * time.Second), base.Add(time.Duration(into) * time.Second)})
			for second := from; second < into; second++ {
				occupied[second] = true
			}
		}
		want := time.Duration(0)
		for _, on := range occupied {
			if on {
				want += time.Second
			}
		}
		if got := unionRunnerTime(intervals); got != want {
			t.Fatal(sample, got, want)
		}
	}
}
