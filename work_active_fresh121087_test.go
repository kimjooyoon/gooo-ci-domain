package cibudgetdomain

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestActiveInputsAndExecutableOrigins121087(t *testing.T) {
	root := workRepository117641(t)
	write83147(t, root, "action.sh", "#!/bin/sh\nprintf new-source121087\n")
	if err := os.Chmod(filepath.Join(root, "action.sh"), 0700); err != nil {
		t.Fatal(err)
	}
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "new executable intent121087")
	ctx := context.Background()
	first, err := ObserveWork(ctx, root, "urn:work121087:active", "HEAD", "", []string{"./action.sh"})
	if err != nil || first.ExecutableDigest == "" {
		t.Fatal(first, err)
	}
	write83147(t, root, "source.go", "package work117641\nconst Value=121087\n")
	if _, err = ObserveWork(ctx, root, first.ID, "HEAD", "", first.Argv); err == nil {
		t.Fatal("uncommitted tracked input accepted")
	}
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "changed source behavior121087")
	write83147(t, root, "action.sh", "#!/bin/sh\nprintf next-body121091\n")
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "changed command implementation121091")
	second, err := ObserveWork(ctx, root, first.ID, "HEAD", "", first.Argv)
	if err != nil || first.CostKey == second.CostKey {
		t.Fatal("executable changed without cost context", err)
	}
	var output bytes.Buffer
	receipt, err := RunWorkWithOutput(ctx, root, filepath.Join(t.TempDir(), "origin.jsonl"), second, time.Now(), 180000, &output)
	if err != nil || receipt.Decision != "COMMAND_COMPLETED" || output.String() != "next-body121091" || receipt.OutputBytes != int64(output.Len()) {
		t.Fatal(receipt, err, output.String())
	}
}

func TestLogicalIndexAndConcurrentReservation121087(t *testing.T) {
	root := workRepository117641(t)
	write83147(t, root, "origin.txt", "logical source121087\n")
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "logical input121087")
	index, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	logical := filepath.Join(t.TempDir(), "logical.index")
	if err = os.WriteFile(logical, index, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_INDEX_FILE", logical)
	t.Setenv("GIT_WORK_TREE", root)
	t.Setenv("GIT_DIR", filepath.Join(root, ".git"))
	ctx := context.Background()
	// The command observes the preserved active logical index, not a stripped environment.
	spec, err := ObserveWork(ctx, root, "urn:work121087:logical", "HEAD", hashBytes([]byte("activation121087")), []string{"/bin/sh", "-c", "test -n \"$GIT_INDEX_FILE\" && cat origin.txt"})
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(t.TempDir(), "shared.jsonl")
	var group sync.WaitGroup
	var successes, failures int
	var mutex sync.Mutex
	for range 2 {
		group.Go(func() {
			receipt, err := RunWork(ctx, root, journal, spec, time.Now(), 180000)
			mutex.Lock()
			defer mutex.Unlock()
			if err == nil && receipt.Decision == "COMMAND_COMPLETED" {
				successes++
			} else {
				failures++
			}
		})
	}
	group.Wait()
	if successes != 1 || failures != 1 {
		t.Fatal("source executed more than once", successes, failures)
	}
	rows, err := ReadObservations(journal)
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	if _, err = ObserveWork(ctx, root, "urn:work121087:oversize", "HEAD", "", []string{"/bin/true", strings.Repeat("x", 65537)}); err == nil {
		t.Fatal("oversize arguments accepted")
	}
}
