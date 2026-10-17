package cibudgetdomain

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkCLIRetainsOutcomeAfterStorageFailure(t *testing.T) {
	root, outside := workRepository117641(t), t.TempDir()
	journal, marker := filepath.Join(outside, "work.jsonl"), filepath.Join(outside, "executions")
	const script = `printf 'once\n' >> "$2"
mv "$1" "$1.start"
mkdir "$1"
printf 'observed command output'`
	const unit = "urn:gooo:work:receipt-storage-failure"
	var stdout, stderr bytes.Buffer
	status := Main([]string{"work", "--root", root, "--unit", unit, "--journal", journal,
		"--budget-ms", "60000", "--", "/bin/sh", "-eu", "-c", script, "storage-fault",
		journal, marker}, &stdout, &stderr)
	var receipt WorkReceipt
	if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil || status != 1 {
		t.Fatal("storage failure lost the observed CLI result", status, stdout.String(), stderr.String(), err)
	}
	const output = "observed command output"
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(output)))
	if receipt.Decision != "COMMAND_COMPLETED" || !receipt.Completed || receipt.ExitCode != 0 ||
		receipt.Spec.ID != unit || receipt.Observation == nil || receipt.OutputBytes != int64(len(output)) ||
		receipt.OutputDigest != digest || receipt.Observation.OutputDigest != digest {
		t.Fatal("command outcome was rewritten behind storage failure", receipt)
	}
	if receipt.Observation.Outcome != "COMMAND_COMPLETED" || !receipt.Observation.Completed ||
		receipt.Spec.SourceHead == "" || receipt.Observation.SourceDigest != receipt.Spec.Digest {
		t.Fatal("original outcome identity was lost", receipt)
	}
	executions, err := os.ReadFile(marker)
	if err != nil || string(executions) != "once\n" {
		t.Fatal("the failed delivery repeated or skipped the command", string(executions), err)
	}
	reservation, err := ReadObservations(journal + ".start")
	if err != nil || len(reservation) != 1 || reservation[0].Outcome != "STARTED" ||
		reservation[0].SourceDigest != receipt.Spec.Digest {
		t.Fatal("original start record was not preserved by the fault fixture", reservation, err)
	}
	info, err := os.Stat(journal)
	if err != nil || !info.IsDir() || !strings.Contains(stderr.String(), output) ||
		!strings.Contains(stderr.String(), journal) {
		t.Fatal("the storage fault or original diagnostic disappeared", stderr.String(), err)
	}
}
