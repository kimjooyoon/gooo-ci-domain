package cibudgetdomain

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIReservesSourceBeforeAutomaticExecution95573(t *testing.T) {
	root := t.TempDir()
	write83147(t, root, "go.mod", "module cohort95573.example/g\n\ngo 1.27.2\n")
	write83147(t, root, "source.go", "package cohort95573\nfunc Value() int {return 61}\n")
	write83147(t, root, "source_test.go", "package cohort95573\nimport \"testing\"\nfunc TestValue95573(t *testing.T){if Value()!=71{t.Fatal(Value())}}\n")
	git83147(t, root, "init", "-q")
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "base95573")
	base := git83147(t, root, "rev-parse", "HEAD")
	write83147(t, root, "source.go", "package cohort95573\nfunc Value() int {return 71}\n")
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "new functional input95573")
	journal := filepath.Join(t.TempDir(), "origins.jsonl")
	args := []string{"improve", "--root", root, "--base", base, "--journal", journal}
	var output, diagnostics bytes.Buffer
	if status := Main(args, &output, &diagnostics); status != 0 {
		t.Fatal(status, diagnostics.String(), output.String())
	}
	var result Execution
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.Decision != "VERIFIED_DISCOVERY" {
		t.Fatal(result, err)
	}
	observations, err := ReadObservations(journal)
	if err != nil || len(observations) != 3 || observations[0].Unit != "cohort-start" || observations[0].Outcome != "STARTED" {
		t.Fatal(observations, err)
	}
	output.Reset()
	diagnostics.Reset()
	if status := Main(args, &output, &diagnostics); status != 1 || !strings.Contains(diagnostics.String(), "existing origin") {
		t.Fatal("same source started again", status, diagnostics.String())
	}
	after, err := ReadObservations(journal)
	if err != nil || len(after) != len(observations) {
		t.Fatal("rejected repeat changed origins", after, err)
	}
}
