package cibudgetdomain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func workRepository117641(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write83147(t, root, "go.mod", "module work117641.example/domain\n\ngo 1.27.2\n")
	write83147(t, root, "source.go", "package work117641\nconst Value=83\n")
	git83147(t, root, "init", "-q")
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "work source117641")
	return root
}
func TestIndependentUnitsShareDeadline117641(t *testing.T) {
	root := workRepository117641(t)
	journal := filepath.Join(t.TempDir(), "work.jsonl")
	ctx := context.Background()
	start := time.Now()
	first, err := ObserveWork(ctx, root, "urn:work117641:first", "HEAD", "", []string{"/bin/sh", "-c", "printf first117641"})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	receipt, err := RunWorkWithOutput(ctx, root, journal, first, start, 180000, &output)
	if err != nil || receipt.Decision != "COMMAND_COMPLETED" || receipt.PredictionMS != nil || receipt.NextOperation != "RECORD_ORIGIN" || output.String() != "first117641" {
		t.Fatal(receipt, err, output.String())
	}
	if _, err = RunWork(ctx, root, journal, first, start, 180000); err == nil {
		t.Fatal("historical unit executed again")
	}
	second, err := ObserveWork(ctx, root, "urn:work117641:second", "HEAD", "", []string{"/bin/sh", "-c", "sleep 20"})
	if err != nil {
		t.Fatal(err)
	}
	scopeStart := time.Now().Add(-120 * time.Millisecond)
	begin := time.Now()
	interrupted, err := RunWork(ctx, root, journal, second, scopeStart, 200)
	if err != nil || interrupted.Decision != "DEADLINE_EXCEEDED" || interrupted.Completed || interrupted.Observation == nil || interrupted.Observation.Completed || time.Since(begin) > 2*time.Second {
		t.Fatal("deadline refreshed between units", interrupted, err, time.Since(begin))
	}
	observations, err := ReadObservations(journal)
	if err != nil || len(observations) != 4 {
		t.Fatal(observations, err)
	}
	estimates, err := Estimates(observations)
	if err != nil || estimates[second.CostKey].Samples != 0 || estimates[second.CostKey].Censored != 1 {
		t.Fatal("deadline became a finished cost", estimates, err)
	}
}
func TestWorkCLIAndStreamingOrigin117641(t *testing.T) {
	root := workRepository117641(t)
	journal := filepath.Join(t.TempDir(), "cli-work.jsonl")
	var result, output bytes.Buffer
	status := Main([]string{"work", "--root", root, "--unit", "urn:work117641:cli", "--journal", journal, "--", "/bin/sh", "-c", "printf cli117641"}, &result, &output)
	var receipt WorkReceipt
	err := json.Unmarshal(result.Bytes(), &receipt)
	if status != 0 || err != nil || receipt.Decision != "COMMAND_COMPLETED" || output.String() != "cli117641" {
		t.Fatal(status, err, result.String(), output.String())
	}
	sink := &workOutput{sink: &bytes.Buffer{}, hash: sha256.New()}
	payload := bytes.Repeat([]byte("z"), 4*1024*1024+17)
	n, err := sink.Write(payload)
	if err != nil || n != len(payload) || sink.bytes != int64(len(payload)) {
		t.Fatal("large original output truncated", n, err, sink.bytes)
	}
}
