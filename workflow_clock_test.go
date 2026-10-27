package cibudgetdomain

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowRetainsBothClocksBeforeSetup(t *testing.T) {
	source, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	parts := strings.Split(text, "      - name: Bind original run and admitted work clocks\n")
	if len(parts) != 2 {
		t.Fatal("one clock owner required")
	}
	clock, _, _ := strings.Cut(parts[1], "      - uses: actions/checkout@v7\n")
	run := strings.Split(clock, "        run: |\n")
	if len(run) != 2 {
		t.Fatal("clock must bind before checkout and setup")
	}
	var lines []string
	for line := range strings.SplitSeq(strings.TrimSuffix(run[1], "\n"), "\n") {
		if !strings.HasPrefix(line, "          ") {
			t.Fatal("unexpected clock script indentation", line)
		}
		lines = append(lines, strings.TrimPrefix(line, "          "))
	}
	script := strings.Join(lines, "\n")
	for _, row := range []struct {
		name, created string
		attempt       int
		apiFails      bool
	}{
		{"long_queue", "2026-10-18T00:00:00Z", 1, false},
		{"stale_attempt", "2026-10-18T00:00:00Z", 2, false},
		{"future_clock", "2026-10-18T02:00:00Z", 1, false},
		{"empty_clock", "", 1, false},
		{"invalid_calendar", "2026-19-18T00:00:00Z", 1, false},
		{"api_failure", "2026-10-18T00:00:00Z", 1, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			exerciseWorkflowClock(t, script, row.created, row.attempt, row.apiFails, row.name == "long_queue")
		})
	}
	if strings.Count(text, "CI_SCOPE_STARTED_AT=%s") != 1 || !strings.Contains(text, "timeout-minutes: 3") ||
		!strings.Contains(text, `--started-at "$CI_SCOPE_STARTED_AT" --budget-ms 180000`) ||
		!strings.Contains(text, "-- go test -race -mod=readonly -p=1 -count=1 -timeout=180s ./...") {
		t.Fatal("work clock, deadline or check population changed")
	}
	for _, path := range []string{"workflow-run.json", "work-started-at.txt", "domain-work-receipt.json"} {
		if !strings.Contains(text, "${{ runner.temp }}/"+path) {
			t.Fatal("original observation omitted from always-upload", path)
		}
	}
}

func exerciseWorkflowClock(t *testing.T, script, created string, attempt int, apiFails, accepted bool) {
	t.Helper()
	root := t.TempDir()
	data, err := json.Marshal(map[string]any{"id": 7018, "run_attempt": attempt, "created_at": created, "head_sha": "original-source"})
	if err != nil {
		t.Fatal(err)
	}
	write83147(t, root, "run.json", string(data)+"\n")
	write83147(t, root, "gh", "#!/bin/sh\nif test \"$CLOCK_API_FAILS\" = true; then exit 7; fi\ncat \"$CLOCK_FIXTURE\"\n")
	write83147(t, root, "date", `#!/bin/sh
if test "$1" = -u; then printf '2026-10-18T01:00:00Z\n'; exit; fi
case "$2" in
  2026-10-18T00:00:00Z) echo 100000 ;;
  2026-10-18T01:00:00Z) echo 103600 ;;
  2026-10-18T02:00:00Z) echo 107200 ;;
  *) exit 1 ;;
esac
`)
	for _, name := range []string{"gh", "date"} {
		if err := os.Chmod(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("/bin/bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", script)
	command.Env = append(os.Environ(), "PATH="+root+string(os.PathListSeparator)+os.Getenv("PATH"),
		"RUNNER_TEMP="+root, "GITHUB_ENV="+filepath.Join(root, "environment"),
		"GITHUB_STEP_SUMMARY="+filepath.Join(root, "summary"), "GITHUB_REPOSITORY=fixture/clock",
		"GITHUB_RUN_ID=7018", "GITHUB_RUN_ATTEMPT=1", "CLOCK_FIXTURE="+filepath.Join(root, "run.json"))
	apiFailure := "false"
	if apiFails {
		apiFailure = "true"
	}
	command.Env = append(command.Env, "CLOCK_API_FAILS="+apiFailure)
	output, runErr := command.CombinedOutput()
	if (runErr == nil) != accepted {
		t.Fatal("clock admission differs", runErr, string(output))
	}
	start, err := os.ReadFile(filepath.Join(root, "work-started-at.txt"))
	if err != nil || string(start) != "2026-10-18T01:00:00Z\n" {
		t.Fatal("admission timestamp not preserved", err, string(start))
	}
	if !apiFails {
		got, err := os.ReadFile(filepath.Join(root, "workflow-run.json"))
		if err != nil || string(got) != string(data)+"\n" {
			t.Fatal("original response was changed", err)
		}
	}
	env, err := os.ReadFile(filepath.Join(root, "environment"))
	if !accepted {
		if !os.IsNotExist(err) {
			t.Fatal("refused clock published a scope", string(env), err)
		}
		return
	}
	if err != nil || string(env) != "CI_SCOPE_STARTED_AT=2026-10-18T01:00:00Z\nCI_WORKFLOW_CREATED_AT=2026-10-18T00:00:00Z\n" {
		t.Fatal("initial wait reset or hid a clock", string(env), err)
	}
	summary, err := os.ReadFile(filepath.Join(root, "summary"))
	if err != nil || !strings.Contains(string(summary), "3600 seconds. Admitted work budget: 180 seconds.") {
		t.Fatal("outer waiting disappeared", string(summary), err)
	}
}
