package cibudgetdomain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWorkAdmissionDiagnosticRetainsUnexecutedOrigin(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	write83147(t, root, "go.mod", "module admission-diagnostic.example/domain\n\ngo 1.27.2\n")
	write83147(t, root, "source.go", "package diagnostic\nconst Value = 18018\n")
	git83147(t, root, "init", "-q")
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "fresh admission diagnostic")
	journal, marker := filepath.Join(outside, "cost.jsonl"), filepath.Join(outside, "executed")
	const unit = "urn:gooo:work:admission-diagnostic"
	argv := []string{"/bin/sh", "-c", `printf 'private-command-output' > "$1"`, "private-command-argument", marker}
	spec, err := ObserveWork(context.Background(), root, unit, "HEAD", "", argv)
	if err != nil {
		t.Fatal(err)
	}
	// A declared fixture cost exercises admission; it is not a measured command.
	history := []Observation{{ID: "fixture-cost", CostKey: spec.CostKey, Unit: unit,
		SourceTree: "fixture-prior-tree", ElapsedMS: 90000, ResolutionMS: 1,
		Completed: true, PredictionState: "unavailable"}}
	if err := AppendObservations(journal, history); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := append([]string{"work", "--root", root, "--unit", unit, "--journal", journal,
		"--budget-ms", "30000", "--"}, argv...)
	status := Main(args, &stdout, &stderr)
	var receipt WorkReceipt
	if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil || status != 1 {
		t.Fatal("admission receipt missing", err, status, stdout.String(), stderr.String())
	}
	if receipt.Decision != "OVER_BUDGET" || receipt.NextOperation != "REFINE_REQUIRED_WORK" ||
		receipt.Admission != "OVER_BUDGET" || receipt.PredictionMS == nil || *receipt.PredictionMS != 90001 ||
		receipt.Observation != nil || receipt.UnitElapsedMS != 0 || receipt.Completed || receipt.ExitCode != -1 {
		t.Fatal("refusal became an executed outcome", receipt)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("refused command ran", err)
	}
	got, err := ReadObservations(journal)
	if err != nil || !reflect.DeepEqual(got, history) {
		t.Fatal("refusal wrote a reservation or outcome", got, err)
	}
	for _, want := range []string{`decision="OVER_BUDGET"`, `next_operation="REFINE_REQUIRED_WORK"`,
		`admission="OVER_BUDGET"`, "prediction_ms=90001", "deadline_ms=30000",
		"unit_elapsed_ms=0", "completed=false", "exit_code=-1"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatal("diagnostic omitted receipt value", want, stderr.String())
		}
	}
	for _, omitted := range []string{root, marker, "private-command", spec.Digest} {
		if strings.Contains(stderr.String(), omitted) {
			t.Fatal("diagnostic copied command or source details", omitted)
		}
	}
	if strings.Count(stderr.String(), "\n") != 1 {
		t.Fatal("expected one diagnostic line", stderr.String())
	}
}

func TestWorkDiagnosticUnknownPredictionAndQuotedValues(t *testing.T) {
	result := WorkReceipt{Decision: "decision\nvalue", NextOperation: "next\"value", ExitCode: -1}
	var stdout, stderr bytes.Buffer
	if writeWorkResult(&stdout, &stderr, result, nil) != 1 || !json.Valid(stdout.Bytes()) ||
		!strings.Contains(stderr.String(), "prediction_ms=unknown") || strings.Count(stderr.String(), "\n") != 1 {
		t.Fatal("unknown cost or quoted values changed", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `decision="decision\nvalue"`) ||
		!strings.Contains(stderr.String(), `next_operation="next\"value"`) {
		t.Fatal("decision text was not quoted", stderr.String())
	}
}
