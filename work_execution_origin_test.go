package cibudgetdomain

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestCurrentExecutionKeepsHistoricalCostWithoutReusingItsResult(t *testing.T) {
	root := workRepository117641(t)
	journal := filepath.Join(t.TempDir(), "cost.jsonl")
	arguments := []string{"work", "--root", root, "--unit", "urn:work:replay",
		"--journal", journal, "--", "/bin/sh", "-c", "printf current"}
	run := func(args []string) (int, WorkReceipt, string) {
		var stdout, stderr bytes.Buffer
		status := Main(args, &stdout, &stderr)
		var receipt WorkReceipt
		if stdout.Len() > 0 {
			if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
				t.Fatalf("work receipt is invalid: %v %s", err, stderr.String())
			}
		}
		return status, receipt, stderr.String()
	}
	status, historical, errText := run(arguments)
	if status != 0 || historical.Decision != "COMMAND_COMPLETED" {
		t.Fatalf("historical cost was not recorded: %d %+v %s", status, historical, errText)
	}
	withExecution := append([]string{}, arguments[:len(arguments)-4]...)
	withExecution = append(withExecution, "--execution-id", "run-2/attempt-1")
	withExecution = append(withExecution, arguments[len(arguments)-4:]...)
	status, current, errText := run(withExecution)
	if status != 0 || current.Decision != "COMMAND_COMPLETED" ||
		current.Spec.CostKey != historical.Spec.CostKey ||
		current.Spec.Digest == historical.Spec.Digest ||
		current.PredictionMS == nil || current.Spec.ExecutionID != "run-2/attempt-1" {
		t.Fatalf("new execution did not reuse cost while running anew: %d %+v %s", status, current, errText)
	}
	before, err := ReadObservations(journal)
	if err != nil || len(before) != 4 || before[2].ExecutionID != "run-2/attempt-1" ||
		before[3].ExecutionID != "run-2/attempt-1" {
		t.Fatalf("current execution origin was not retained: %+v %v", before, err)
	}
	status, _, errText = run(withExecution)
	if status != 1 || !strings.Contains(errText, "existing origin") {
		t.Fatalf("the same execution ran twice: %d %s", status, errText)
	}
	after, err := ReadObservations(journal)
	if err != nil || len(after) != len(before) {
		t.Fatalf("rejected duplicate changed the historical journal: %+v %v", after, err)
	}
}
