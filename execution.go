package cibudgetdomain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Execution struct {
	Decision, NextOperation, SourceDigest string
	Plan                                  Plan
	Observations                          []Observation
	Missing                               []string
	ScopeElapsedMS                        int64
}

// Execute reobserves the current input domain before starting a planned command.
// Historical durations are predictions, never current-source verification.
func Execute(ctx context.Context, root string, repository Repository, plan Plan, history []Observation) (Execution, error) {
	result := Execution{Decision: plan.Decision, NextOperation: NextStep(plan.Decision), SourceDigest: repository.Digest, Plan: plan}
	if err := repository.Validate(); err != nil {
		return result, err
	}
	if plan.Workers != repository.Workers {
		return result, fmt.Errorf("worker capacity differs from cost profile")
	}
	if plan.SourceDigest != repository.Digest {
		return result, fmt.Errorf("plan source differs")
	}
	estimates, err := Estimates(history)
	if err != nil {
		return result, err
	}
	expected, err := BuildPlan(repository.Graph, estimates, plan.Workers, plan.OverheadMS, plan.DeadlineMS)
	if err != nil {
		return result, err
	}
	a, _ := json.Marshal(expected)
	b, _ := json.Marshal(plan)
	if string(a) != string(b) {
		return result, fmt.Errorf("plan differs from source and timing origins")
	}
	if plan.Decision != "READY" {
		return result, nil
	}
	fresh, err := CollectWithWorkers(ctx, root, repository.Base, repository.Head, plan.Workers)
	if err != nil {
		return result, err
	}
	if fresh.Digest != repository.Digest {
		return result, fmt.Errorf("live source or execution profile differs")
	}
	for _, old := range history {
		if old.SourceDigest == repository.Digest || old.SourceTree == repository.Tree && old.Profile == repository.Profile {
			return result, fmt.Errorf("this exact source cohort already has an execution origin; advance source before retrying")
		}
	}
	return runUnits(ctx, root, repository, plan, result), nil
}

func runUnits(ctx context.Context, root string, repository Repository, plan Plan, result Execution) Execution {
	jobs := make(chan string, len(plan.RequiredTests))
	outputs := make(chan Observation, len(plan.RequiredTests))
	var workers sync.WaitGroup
	for _, id := range plan.RequiredTests {
		jobs <- id
	}
	close(jobs)
	for worker := 0; worker < plan.Workers; worker++ {
		workers.Go(func() {
			for id := range jobs {
				if ctx.Err() != nil {
					return
				}
				outputs <- runTestUnit(ctx, root, id, repository)
			}
		})
	}
	workers.Wait()
	close(outputs)
	observed := map[string]bool{}
	failed := false
	for observation := range outputs {
		observation.PredictionState = "unavailable"
		for _, slot := range plan.Slots {
			if repository.Graph.Units[slot.Unit].ID == observation.Unit {
				observation.PredictedMS = slot.EndMS - slot.StartMS
				observation.PredictionState = "empirical"
			}
		}
		result.Observations = append(result.Observations, observation)
		observed[observation.Unit] = true
		failed = failed || observation.Outcome != "PASS"
	}
	sort.Slice(result.Observations, func(i, j int) bool { return result.Observations[i].Unit < result.Observations[j].Unit })
	for _, id := range plan.RequiredTests {
		if !observed[id] {
			result.Missing = append(result.Missing, id)
		}
	}
	switch {
	case ctx.Err() != nil:
		result.Decision = "DEADLINE_EXCEEDED"
	case failed || len(result.Missing) != 0:
		result.Decision = "VERIFY_FAILED"
	default:
		result.Decision = "VERIFIED_SCOPED"
	}
	result.NextOperation = NextStep(result.Decision)
	return result
}
func runTestUnit(ctx context.Context, root, id string, repository Repository) Observation {
	start := time.Now()
	output := &limitedOutput{}
	originRoot, originErr := os.MkdirTemp("", "gooo-input-origin-")
	if originErr != nil {
		return Observation{ID: repository.Digest + "/" + id + "/" + strconv.FormatInt(start.UnixNano(), 10), CostKey: unitCostKey(repository, id), SourceDigest: repository.Digest, SourceTree: repository.Tree, Profile: repository.Profile, Unit: id, Completed: false, Outcome: "INPUT_OBSERVATION_UNAVAILABLE"}
	}
	defer os.RemoveAll(originRoot)
	args := append([]string(nil), repository.Commands[id]...)
	for index, argument := range args {
		args[index] = strings.ReplaceAll(argument, "{{origin_file}}", filepath.Join(originRoot, "test-inputs.log"))
	}
	command := managedCommand(ctx, root, "go", args, output)
	err := command.Run()
	if command.Process != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	completed := ctx.Err() == nil
	outcome := "PASS"
	data, overflow := output.bytes()
	if !completed {
		outcome = "DEADLINE_EXCEEDED"
	} else if err != nil || overflow {
		outcome = "FAIL"
	}
	key := ""
	for _, unit := range repository.Graph.Units {
		if unit.ID == id {
			key = unit.CostKey
		}
	}
	observation := Observation{ID: repository.Digest + "/" + id + "/" + strconv.FormatInt(start.UnixNano(), 10), CostKey: key, SourceDigest: repository.Digest, SourceTree: repository.Tree, Profile: repository.Profile,
		Unit: id, Outcome: outcome, ElapsedMS: time.Since(start).Milliseconds(), ResolutionMS: 1, Completed: completed}
	observation.OutputDigest = hashBytes(data)
	observation.InputOrigins, observation.InputLogState = observedTestInputs(repository, root, id, filepath.Join(originRoot, "test-inputs.log"))
	return observation
}

// Go's package timeout does not cover compilation or child processes; this
// process-group deadline is the outer execution boundary.
func RunBounded(ctx context.Context, root, name string, args []string) error {
	output := &limitedOutput{}
	command := managedCommand(ctx, root, name, args, output)
	err := command.Run()
	if command.Process != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	_, overflow := output.bytes()
	if overflow {
		return fmt.Errorf("bounded command output exceeded 4 MiB")
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		if failure, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("bounded command exit %d", failure.ExitCode())
		}
		return err
	}
	return nil
}
