package cibudgetdomain

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCommandOriginForecastsNextContent118397(t *testing.T) {
	root := workRepository117641(t)
	write83147(t, root, "payload.txt", "source intent118397:131\n")
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "first behavior118397")
	journal := filepath.Join(t.TempDir(), "learning.jsonl")
	ctx := context.Background()
	start := time.Now()
	argv := []string{"/bin/cat", "payload.txt"}
	first, err := ObserveWork(ctx, root, "urn:work118397:read", "HEAD", "", argv)
	if err != nil {
		t.Fatal(err)
	}
	var before bytes.Buffer
	one, err := RunWorkWithOutput(ctx, root, journal, first, start, 180000, &before)
	if err != nil || one.Decision != "COMMAND_COMPLETED" || one.PredictionMS != nil {
		t.Fatal(one, err)
	}
	write83147(t, root, "payload.txt", "source intent118397:137\n")
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "next functional content118397")
	nextStart := time.Now()
	second, err := ObserveWork(ctx, root, first.ID, "HEAD", "", argv)
	if err != nil {
		t.Fatal(err)
	}
	var after bytes.Buffer
	two, err := RunWorkWithOutput(ctx, root, journal, second, nextStart, 180000, &after)
	if err != nil || two.Decision != "COMMAND_COMPLETED" || two.PredictionMS == nil || two.Admission != "READY" || first.CostKey != second.CostKey || first.Digest == second.Digest || before.String() == after.String() {
		t.Fatal("new content did not use matching command-cost origin", one, two, err)
	}
	if two.Observation == nil || two.Observation.PredictionState != "empirical" {
		t.Fatal(two)
	}
	// A changed command definition cannot inherit the old unit's cost.
	altered, err := ObserveWork(ctx, root, first.ID, "HEAD", "", []string{"/bin/sh", "-c", "cat payload.txt"})
	if err != nil || altered.CostKey == second.CostKey {
		t.Fatal("command identity drift was hidden", altered, err)
	}
	tampered := second
	tampered.Argv = []string{"/bin/true"}
	if _, err = RunWork(ctx, root, journal, tampered, time.Now(), 180000); err == nil {
		t.Fatal("altered command projection executed")
	}
}
