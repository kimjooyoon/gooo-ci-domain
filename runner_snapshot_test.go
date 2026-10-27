package cibudgetdomain

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// These snapshots are synthetic shifts of a retained fixture, not fresh API evidence.
func snapshotCLI(t *testing.T, change func(*ClockJobs, ClockBinding)) (int, string) {
	t.Helper()
	run, jobs, bind, observed := recordedClock(t)
	delta := time.Now().UTC().Add(-time.Second).Sub(observed)
	run.CreatedAt = run.CreatedAt.Add(delta)
	for i := range jobs.Jobs {
		job := &jobs.Jobs[i]
		if job.StartedAt != nil {
			value := job.StartedAt.Add(delta)
			job.StartedAt = &value
		}
		if job.CompletedAt != nil {
			value := job.CompletedAt.Add(delta)
			job.CompletedAt = &value
		}
	}
	change(&jobs, bind)
	root := t.TempDir()
	for name, value := range map[string]any{"run.json": run, "jobs.json": jobs} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var output, diagnostic bytes.Buffer
	code := MainRunnerClock([]string{"--run-json", filepath.Join(root, "run.json"), "--jobs-json", filepath.Join(root, "jobs.json"), "--head", bind.Head, "--run-id", strconv.FormatInt(bind.RunID, 10), "--attempt", "1", "--budget-ms", "3000000", "--job", bind.CurrentJob, "--producer", bind.ProducerJob}, &output, &diagnostic)
	return code, output.String()
}

func pendingCurrent(jobs *ClockJobs, bind ClockBinding) {
	for i := range jobs.Jobs {
		if jobs.Jobs[i].Name == bind.CurrentJob {
			job := &jobs.Jobs[i]
			job.Status = "queued"
			job.Conclusion = ""
			job.StartedAt = nil
			job.CompletedAt = nil
			job.RunnerID = nil
			job.Steps = nil
		}
	}
}

func TestRunnerSnapshotDistinguishesPendingPublicationFromClockAdmission(t *testing.T) {
	code, output := snapshotCLI(t, pendingCurrent)
	if code != 75 || output != "" {
		t.Fatalf("unpublished job emitted admission or an undifferentiated failure: code=%d output=%s", code, output)
	}
	code, output = snapshotCLI(t, func(*ClockJobs, ClockBinding) {})
	if code != 0 || !strings.Contains(output, `"decision":"READY"`) {
		t.Fatal("published snapshot did not retain the original allowance", code, output)
	}
}

func TestRunnerSnapshotDoesNotRefreshForeignOrFailedOrigins(t *testing.T) {
	for _, mutation := range []string{"foreign", "duplicate", "producer-failure"} {
		t.Run(mutation, func(t *testing.T) {
			code, output := snapshotCLI(t, func(jobs *ClockJobs, bind ClockBinding) {
				pendingCurrent(jobs, bind)
				if mutation == "producer-failure" {
					for i := range jobs.Jobs {
						if jobs.Jobs[i].Name == bind.ProducerJob {
							jobs.Jobs[i].Conclusion = "failure"
						}
					}
					// Put the pending current row before its failed producer.
					for i := range jobs.Jobs {
						if jobs.Jobs[i].Name == bind.CurrentJob {
							jobs.Jobs[0], jobs.Jobs[i] = jobs.Jobs[i], jobs.Jobs[0]
							break
						}
					}
				} else {
					extra := ClockJob{ID: 999, RunID: bind.RunID, RunAttempt: bind.Attempt, HeadSHA: bind.Head, Name: "late-row", Status: "queued"}
					if mutation == "foreign" {
						extra.HeadSHA = strings.Repeat("b", 40)
					} else {
						extra.ID = jobs.Jobs[0].ID
					}
					jobs.Jobs = append(jobs.Jobs, extra)
					jobs.TotalCount = len(jobs.Jobs)
				}
			})
			if code != 1 || output != "" {
				t.Fatal("a contradiction became refreshable or admitted", code, output)
			}
		})
	}
}

func TestRunnerSnapshotPendingStatesNeverEmitAClock(t *testing.T) {
	for _, status := range []string{"queued", "waiting", "pending", "requested", "absent"} {
		t.Run(status, func(t *testing.T) {
			code, output := snapshotCLI(t, func(jobs *ClockJobs, bind ClockBinding) {
				pendingCurrent(jobs, bind)
				for i := range jobs.Jobs {
					if jobs.Jobs[i].Name != bind.CurrentJob {
						continue
					}
					if status == "absent" {
						jobs.Jobs = append(jobs.Jobs[:i], jobs.Jobs[i+1:]...)
						jobs.TotalCount = len(jobs.Jobs)
					} else {
						jobs.Jobs[i].Status = status
					}
					break
				}
			})
			if code != 75 || output != "" {
				t.Fatal("pending publication emitted a clock", code, output)
			}
		})
	}
}

func TestRunnerSnapshotContradictionsAreTerminal(t *testing.T) {
	for _, mutation := range []string{"terminal-current", "assigned-queued", "unknown-later-status", "missing-producer", "incomplete"} {
		t.Run(mutation, func(t *testing.T) {
			code, output := snapshotCLI(t, func(jobs *ClockJobs, bind ClockBinding) {
				pendingCurrent(jobs, bind)
				if mutation == "incomplete" {
					jobs.TotalCount++
				}
				for i := range jobs.Jobs {
					job := &jobs.Jobs[i]
					if job.Name == bind.CurrentJob {
						switch mutation {
						case "terminal-current":
							job.Status, job.Conclusion = "completed", "cancelled"
						case "assigned-queued":
							runner := int64(1)
							job.RunnerID = &runner
						}
					}
					if job.Name == bind.ProducerJob && mutation == "missing-producer" {
						job.Name = "unrelated producer"
					}
				}
				if mutation == "unknown-later-status" {
					jobs.Jobs = append(jobs.Jobs, ClockJob{ID: 999, RunID: bind.RunID,
						RunAttempt: bind.Attempt, HeadSHA: bind.Head, Name: "late-row", Status: "unknown"})
					jobs.TotalCount = len(jobs.Jobs)
				}
			})
			if code != 1 || output != "" {
				t.Fatal("contradiction became refreshable or emitted a clock", code, output)
			}
		})
	}
}
