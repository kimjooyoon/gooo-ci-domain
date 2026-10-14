package cibudgetdomain

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeadlineAndBoundedOutput83147(t *testing.T) {
	output := &limitedOutput{}
	payload := bytes.Repeat([]byte("x"), 4*1024*1024+31)
	n, err := output.Write(payload)
	data, overflow := output.bytes()
	if err != nil || n != len(payload) || len(data) != 4*1024*1024 || !overflow {
		t.Fatal(n, len(data), overflow, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = RunBounded(ctx, t.TempDir(), "/bin/sh", []string{"-c", "sleep 20 & wait"})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 2*time.Second {
		t.Fatal("descendant process retained execution budget", time.Since(start), err)
	}
}
func TestJournalAndFiniteNextAction83147(t *testing.T) {
	name := filepath.Join(t.TempDir(), "timing.jsonl")
	first := Observation{ID: "new-origin83147", CostKey: "profile83147", ElapsedMS: 1700, PredictedMS: 1000, ResolutionMS: 1, Completed: true}
	if err := AppendObservations(name, []Observation{first}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if err = AppendObservations(name, []Observation{first}); err == nil {
		t.Fatal("repeated timing origin accepted")
	}
	after, err := os.ReadFile(name)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("duplicate append altered origins", err)
	}
	observations, err := ReadObservations(name)
	if err != nil {
		t.Fatal(err)
	}
	estimates, err := Estimates(observations)
	if err != nil || estimates[first.CostKey].ForecastMS != 2402 {
		t.Fatal(estimates, err)
	}
	for decision, want := range map[string]string{"READY": "EXECUTE_SCOPED_PLAN", "OVER_BUDGET": "REFINE_REQUIRED_WORK", "COST_UNRESOLVED": "COLLECT_COST_ORIGIN", "IMPACT_UNRESOLVED": "RESOLVE_INPUT_ORIGIN", "DEADLINE_EXCEEDED": "RECALIBRATE_CENSORED_COST", "VERIFY_FAILED": "REPAIR_SOURCE", "VERIFIED_SCOPED": "RECORD_ORIGIN"} {
		if NextStep(decision) != want {
			t.Fatal(decision, NextStep(decision), want)
		}
	}
}
