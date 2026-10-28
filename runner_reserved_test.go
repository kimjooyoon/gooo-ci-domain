package cibudgetdomain

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func assignedQueuedClock(t *testing.T) (ClockRun, ClockJobs, ClockBinding, time.Time) {
	t.Helper()
	raw, err := os.ReadFile("testdata/runner-clock/40765497394-assigned-queued.zip")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != "1ee3b45aa504a0b26adfb9b998e8b52264b1722706afb7d4353554e82a4a3a87" {
		t.Fatal("original API artifact changed")
	}
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var run ClockRun
	var jobs ClockJobs
	for _, f := range archive.File {
		var into any
		switch {
		case strings.HasSuffix(f.Name, "/workflow-run.json"):
			into = &run
		case strings.HasSuffix(f.Name, "/attempt-jobs.json"):
			into = &jobs
		default:
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(r)
		r.Close()
		if readErr != nil || json.Unmarshal(data, into) != nil {
			t.Fatal("original API response unreadable", f.Name, readErr)
		}
	}
	observed := time.Date(2026, 10, 28, 20, 44, 8, 0, time.UTC)
	bind := ClockBinding{Head: run.HeadSHA, RunID: run.ID, Attempt: run.RunAttempt,
		CurrentJob: "go test", ProducerJob: "Work domain preparation", BudgetMS: 3000000}
	return run, jobs, bind, observed
}

func TestAssignedQueuedSnapshotDoesNotAdmitAClock(t *testing.T) {
	run, jobs, bind, observed := assignedQueuedClock(t)
	report, err := ObserveRunnerClock(run, jobs, bind, observed)
	pending, ok := errors.AsType[*ClockSnapshotPendingError](err)
	if !ok || pending.Job != "go test -race" || pending.Status != "queued" || report.Schema != "" {
		t.Fatal("reserved runner was admitted or classified as a terminal contradiction", report, err)
	}
}

func TestAssignedQueuedCLIReturnsRefreshWithoutClock(t *testing.T) {
	code, output := snapshotCLI(t, func(jobs *ClockJobs, bind ClockBinding) {
		runner := int64(7)
		jobs.Jobs = append(jobs.Jobs, ClockJob{ID: 988888, RunID: bind.RunID, RunAttempt: bind.Attempt,
			HeadSHA: bind.Head, Name: "reserved sibling", Status: "queued", RunnerID: &runner,
			Steps: []struct{}{}})
		jobs.TotalCount = len(jobs.Jobs)
	})
	if code != 75 || output != "" {
		t.Fatal("reserved sibling emitted a clock or lost bounded refresh classification", code, output)
	}
}

func TestAssignedQueuedSnapshotCannotHideContradictions(t *testing.T) {
	for _, mutation := range []string{"foreign", "duplicate", "failed-producer", "steps", "negative-runner", "completed-queued"} {
		t.Run(mutation, func(t *testing.T) {
			run, jobs, bind, observed := assignedQueuedClock(t)
			for i, job := range jobs.Jobs {
				if job.Name == "go test -race" {
					jobs.Jobs[0], jobs.Jobs[i] = jobs.Jobs[i], jobs.Jobs[0]
					break
				}
			}
			switch mutation {
			case "foreign":
				jobs.Jobs[len(jobs.Jobs)-1].HeadSHA = strings.Repeat("b", 40)
			case "duplicate":
				jobs.Jobs[len(jobs.Jobs)-1].ID = jobs.Jobs[0].ID
			case "failed-producer":
				for i := range jobs.Jobs {
					if jobs.Jobs[i].Name == bind.ProducerJob {
						jobs.Jobs[i].Conclusion = "failure"
					}
				}
			case "steps":
				jobs.Jobs[0].Steps = []struct{}{{}}
			case "negative-runner":
				invalid := int64(-1)
				jobs.Jobs[0].RunnerID = &invalid
			case "completed-queued":
				jobs.Jobs[0].CompletedAt = &observed
			}
			report, err := ObserveRunnerClock(run, jobs, bind, observed)
			if err == nil || report.Schema != "" {
				t.Fatal("invalid snapshot admitted a clock", report, err)
			}
			if _, pending := errors.AsType[*ClockSnapshotPendingError](err); pending {
				t.Fatal("pending row hid a later contradiction", err)
			}
		})
	}
}

func TestAssignedQueuedPublicationPreservesOriginalAllowance(t *testing.T) {
	run, jobs, bind, observed := assignedQueuedClock(t)
	reserved := -1
	for i := range jobs.Jobs {
		if jobs.Jobs[i].Name == "go test -race" {
			reserved = i
			zero := int64(0)
			jobs.Jobs[i].RunnerID = &zero
		}
	}
	if reserved < 0 {
		t.Fatal("original reserved runner missing")
	}
	// Synthetic before/after publication around the retained API population.
	before, err := ObserveRunnerClock(run, jobs, bind, observed)
	if err != nil {
		t.Fatal(err)
	}
	runner := int64(1000420533)
	started := observed.Add(time.Second)
	jobs.Jobs[reserved].RunnerID, jobs.Jobs[reserved].StartedAt = &runner, &started
	jobs.Jobs[reserved].Status = "in_progress"
	after, err := ObserveRunnerClock(run, jobs, bind, observed.Add(5*time.Second))
	if err != nil || after.BudgetMS != before.BudgetMS || after.ActiveElapsedMS != before.ActiveElapsedMS+5000 ||
		after.RemainingMS != before.RemainingMS-5000 || !after.EffectiveStartedAt.Equal(before.EffectiveStartedAt) {
		t.Fatal("publication reset allowance or dropped occupied work", before, after, err)
	}
}
