package cibudgetdomain

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestWriteWorkResultRetainsObservedOutcome(t *testing.T) {
	storageErr := errors.New("end observation could not be stored")
	cases := []struct {
		name, decision string
		completed      bool
		exit           int
		runErr         error
		status         int
	}{
		{"completed", "COMMAND_COMPLETED", true, 0, nil, 0},
		{"completed-storage-error", "COMMAND_COMPLETED", true, 0, storageErr, 1},
		{"failed", "VERIFY_FAILED", true, 2, nil, 1},
		{"failed-storage-error", "VERIFY_FAILED", true, 2, storageErr, 1},
		{"deadline", "DEADLINE_EXCEEDED", false, -1, nil, 1},
		{"deadline-storage-error", "DEADLINE_EXCEEDED", false, -1, storageErr, 1},
		{"origin-unavailable", "ORIGIN_UNAVAILABLE", false, 1, nil, 1},
		{"origin-unavailable-storage-error", "ORIGIN_UNAVAILABLE", false, 1, storageErr, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			forecast := int64(30)
			observation := Observation{ID: "original/end", SourceDigest: "source", Unit: "unit",
				Outcome: tc.decision, Completed: tc.completed, ElapsedMS: 17, OutputDigest: "output",
				InputOrigins: []InputOrigin{{Operation: "read", Path: "policy.gooo"}}}
			result := WorkReceipt{Schema: "gooo/command-work-origin/v1", Decision: tc.decision,
				Spec: WorkSpec{ID: "unit", SourceHead: "head", SourceTree: "tree", Digest: "source"},
				NextOperation: "original-next-operation", Admission: "READY", PredictionMS: &forecast,
				UnitElapsedMS: 17, ScopeElapsedMS: 21, DeadlineMS: 40, ExitCode: tc.exit,
				Completed: tc.completed, OutputDigest: "output", OutputBytes: 9,
				Observation: &observation}
			var stdout, stderr bytes.Buffer
			if status := writeWorkResult(&stdout, &stderr, result, tc.runErr); status != tc.status {
				t.Fatal("delivery changed the operation status", status, stderr.String())
			}
			var decoded WorkReceipt
			err := json.Unmarshal(stdout.Bytes(), &decoded)
			if err != nil || !reflect.DeepEqual(decoded, result) {
				t.Fatal("observed result was lost or rewritten", decoded, err)
			}
			if tc.runErr == nil && stderr.Len() != 0 ||
				tc.runErr != nil && !strings.Contains(stderr.String(), tc.runErr.Error()) {
				t.Fatal("original persistence cause was lost", stderr.String())
			}
		})
	}
}

func TestWriteWorkResultPreservesPreExecutionBehavior(t *testing.T) {
	var stdout, stderr bytes.Buffer
	result := WorkReceipt{Schema: "gooo/command-work-origin/v1", ExitCode: -1}
	if status := writeWorkResult(&stdout, &stderr, result, errors.New("source differs")); status != 1 ||
		stdout.Len() != 0 || !strings.Contains(stderr.String(), "source differs") {
		t.Fatal("pre-execution refusal became a command observation", status, stdout.String(), stderr.String())
	}
	stderr.Reset()
	result.Decision = "OVER_BUDGET"
	if status := writeWorkResult(&stdout, &stderr, result, nil); status != 1 || stderr.Len() != 0 {
		t.Fatal("admission refusal status changed", status, stderr.String())
	}
	var decoded WorkReceipt
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil || !reflect.DeepEqual(decoded, result) {
		t.Fatal("existing admission result was lost", decoded, err)
	}
}

func TestWriteWorkResultReportsOutputAndPersistenceErrors(t *testing.T) {
	for _, failedStorage := range []bool{false, true} {
		name := "output-only"
		var runErr error
		if failedStorage {
			name, runErr = "output-and-storage", errors.New("end observation could not be stored")
		}
		t.Run(name, func(t *testing.T) {
			stdout := &partialReceiptWriter{}
			var stderr bytes.Buffer
			result := WorkReceipt{Decision: "COMMAND_COMPLETED", Completed: true,
				Observation: &Observation{ID: "original/end", Completed: true}}
			if status := writeWorkResult(stdout, &stderr, result, runErr); status != 1 {
				t.Fatal("failed output was reported successful", status)
			}
			if !strings.Contains(stderr.String(), "receipt output failed") ||
				failedStorage && !strings.Contains(stderr.String(), runErr.Error()) {
				t.Fatal("output or persistence cause disappeared", stderr.String())
			}
			if stdout.Len() == 0 || json.Valid(stdout.Bytes()) {
				t.Fatal("partial receipt became a complete result", stdout.String())
			}
		})
	}
}

type partialReceiptWriter struct{ bytes.Buffer }

func (writer *partialReceiptWriter) Write(value []byte) (int, error) {
	n := min(5, len(value))
	_, _ = writer.Buffer.Write(value[:n])
	return n, errors.New("receipt output failed")
}
