package cibudgetdomain

import (
	"encoding/hex"
	"fmt"
	"slices"
	"time"
)

// RunnerClock is elapsed time with at least one owned job assigned to a runner.
// A polling barrier still spends time. Only gaps with no runner work are removed.
type RunnerClock struct {
	Schema             string    `json:"schema"`
	Decision           string    `json:"decision"`
	SourceHead         string    `json:"source_head"`
	RunID              int64     `json:"run_id"`
	RunAttempt         int64     `json:"run_attempt"`
	ObservedAt         time.Time `json:"observed_at"`
	WorkflowCreatedAt  time.Time `json:"workflow_created_at"`
	EffectiveStartedAt time.Time `json:"effective_started_at"`
	WallElapsedMS      int64     `json:"wall_elapsed_ms"`
	ActiveElapsedMS    int64     `json:"active_elapsed_ms"`
	RunnerFreeMS       int64     `json:"runner_free_ms"`
	RemainingMS        int64     `json:"remaining_ms"`
	BudgetMS           int64     `json:"budget_ms"`
	PolicyDigest       string    `json:"policy_digest"`
	RunSHA256          string    `json:"run_sha256,omitempty"`
	JobsSHA256         string    `json:"jobs_sha256,omitempty"`
	JobIDs             []int64   `json:"job_ids"`
}

type ClockRun struct {
	ID         int64     `json:"id"`
	RunAttempt int64     `json:"run_attempt"`
	HeadSHA    string    `json:"head_sha"`
	CreatedAt  time.Time `json:"created_at"`
}

type ClockJob struct {
	ID          int64      `json:"id"`
	RunID       int64      `json:"run_id"`
	RunAttempt  int64      `json:"run_attempt"`
	HeadSHA     string     `json:"head_sha"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Conclusion  string     `json:"conclusion"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

type ClockJobs struct {
	TotalCount int        `json:"total_count"`
	Jobs       []ClockJob `json:"jobs"`
}

type ClockBinding struct {
	Head, CurrentJob, ProducerJob string
	RunID, Attempt, BudgetMS      int64
}

type runnerInterval struct{ start, end time.Time }

func ObserveRunnerClock(run ClockRun, jobs ClockJobs, bind ClockBinding, now time.Time) (RunnerClock, error) {
	if err := validateClockBinding(run, jobs, bind, now); err != nil {
		return RunnerClock{}, err
	}
	intervals, ids, err := clockIntervals(run, jobs.Jobs, bind, now)
	if err != nil {
		return RunnerClock{}, err
	}
	active := unionRunnerTime(intervals)
	// Round charged milliseconds up, so converting to a deadline never grants time.
	activeMS := (active + time.Millisecond - 1) / time.Millisecond
	wallMS := int64((now.Sub(run.CreatedAt) + time.Millisecond - 1) / time.Millisecond)
	report := RunnerClock{Schema: "gooo/ci-runner-clock/v1", SourceHead: bind.Head, RunID: bind.RunID,
		RunAttempt: bind.Attempt, ObservedAt: now, WorkflowCreatedAt: run.CreatedAt,
		WallElapsedMS: wallMS, ActiveElapsedMS: int64(activeMS), RunnerFreeMS: wallMS - int64(activeMS),
		RemainingMS: bind.BudgetMS - int64(activeMS), BudgetMS: bind.BudgetMS, JobIDs: ids,
		PolicyDigest: SourcePolicyDigest()}
	report.Decision = AdmitRunnerClock(report.ActiveElapsedMS, report.BudgetMS)
	report.EffectiveStartedAt = now.Add(-time.Duration(report.ActiveElapsedMS) * time.Millisecond)
	return report, nil
}

func validateClockBinding(run ClockRun, jobs ClockJobs, bind ClockBinding, now time.Time) error {
	decoded, err := hex.DecodeString(bind.Head)
	if err != nil || len(decoded) != 20 || bind.RunID <= 0 || bind.Attempt <= 0 || bind.CurrentJob == "" || bind.ProducerJob == "" ||
		run.ID != bind.RunID || run.RunAttempt != bind.Attempt || run.HeadSHA != bind.Head {
		return fmt.Errorf("runner clock source, run, attempt or job binding differs")
	}
	if run.CreatedAt.IsZero() || run.CreatedAt.After(now) || now.Sub(run.CreatedAt) > 30*24*time.Hour {
		return fmt.Errorf("runner clock original run time is invalid")
	}
	if jobs.TotalCount != len(jobs.Jobs) || len(jobs.Jobs) == 0 || len(jobs.Jobs) > 4096 {
		return fmt.Errorf("runner clock job population is incomplete or exceeds 4096")
	}
	if AdmitRunnerClock(0, bind.BudgetMS) != "READY" {
		return fmt.Errorf("runner clock budget is invalid")
	}
	return nil
}

func clockIntervals(run ClockRun, jobs []ClockJob, bind ClockBinding, now time.Time) ([]runnerInterval, []int64, error) {
	intervals, ids := []runnerInterval{}, []int64{}
	seen, current, producer := map[int64]bool{}, 0, 0
	for _, job := range jobs {
		if job.ID <= 0 || seen[job.ID] || job.RunID != bind.RunID || job.RunAttempt != bind.Attempt || job.HeadSHA != bind.Head {
			return nil, nil, fmt.Errorf("runner clock job %q has a duplicate or different source origin", job.Name)
		}
		seen[job.ID] = true
		if job.Name == bind.CurrentJob {
			current++
			if job.Status != "in_progress" {
				return nil, nil, fmt.Errorf("current runner job is not in progress")
			}
		}
		if job.Name == bind.ProducerJob {
			producer++
			if job.Status != "completed" || job.Conclusion != "success" {
				return nil, nil, fmt.Errorf("runner clock producer is not successful")
			}
		}
		interval, active, err := clockJobInterval(job, run.CreatedAt, now)
		if err != nil {
			return nil, nil, err
		}
		if active {
			intervals = append(intervals, interval)
			ids = append(ids, job.ID)
		}
	}
	if current != 1 || producer != 1 {
		return nil, nil, fmt.Errorf("runner clock requires one current and one producer job")
	}
	slices.Sort(ids)
	return intervals, ids, nil
}

func clockJobInterval(job ClockJob, created, now time.Time) (runnerInterval, bool, error) {
	fail := func() (runnerInterval, bool, error) {
		return runnerInterval{}, false, fmt.Errorf("invalid runner interval for %q", job.Name)
	}
	if job.Status == "queued" || job.Status == "waiting" || job.Status == "pending" || job.Status == "requested" {
		if job.StartedAt != nil || job.CompletedAt != nil {
			return fail()
		}
		return runnerInterval{}, false, nil
	}
	if job.Status == "completed" && job.Conclusion == "skipped" && job.StartedAt == nil && job.CompletedAt == nil {
		return runnerInterval{}, false, nil
	}
	if job.StartedAt == nil || job.StartedAt.Before(created) || job.StartedAt.After(now) {
		return fail()
	}
	end := now
	switch job.Status {
	case "in_progress":
		if job.CompletedAt != nil {
			return fail()
		}
	case "completed":
		if job.CompletedAt == nil || job.Conclusion == "" {
			return fail()
		}
		end = *job.CompletedAt
	default:
		return fail()
	}
	if end.Before(*job.StartedAt) || end.After(now) {
		return fail()
	}
	return runnerInterval{*job.StartedAt, end}, true, nil
}

func unionRunnerTime(intervals []runnerInterval) time.Duration {
	slices.SortFunc(intervals, func(a, b runnerInterval) int { return a.start.Compare(b.start) })
	var total time.Duration
	var last time.Time
	for _, interval := range intervals {
		start := interval.start
		if start.Before(last) {
			start = last
		}
		if interval.end.After(start) {
			total += interval.end.Sub(start)
		}
		if interval.end.After(last) {
			last = interval.end
		}
	}
	return total
}
