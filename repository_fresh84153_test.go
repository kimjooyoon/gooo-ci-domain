package cibudgetdomain

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func git84153(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Gooo fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Gooo fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	return strings.TrimSpace(string(out))
}
func write84153(t *testing.T, root, name, source string) {
	t.Helper()
	target := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
}
func repository84153(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":      "module fixture84153.example/domain\n\ngo 1.27.2\n",
		"a/a.go":      "package a\nfunc Value() int { return 7 }\n",
		"b/b.go":      "package b\nimport \"fixture84153.example/domain/a\"\nfunc Value() int {return a.Value()+11}\n",
		"b/b_test.go": "package b\nimport \"testing\"\nfunc TestValue84153(t *testing.T){if Value()!=23{t.Fatal(Value())}}\n",
		"c/c.go":      "package c\nfunc Value() int {return 13}\n",
		"c/c_test.go": "package c_test\nimport(\"testing\";\"fixture84153.example/domain/a\")\nfunc TestImportedInput84153(t *testing.T){if a.Value()!=12{t.Fatal(a.Value())}}\n",
		"d/d.go":      "package d\nfunc Value() int {return 17}\n",
		"README.md":   "new input domain84153\n",
	}
	for name, source := range files {
		write84153(t, root, name, source)
	}
	git84153(t, root, "init", "-q")
	git84153(t, root, "add", ".")
	git84153(t, root, "commit", "-qm", "fresh base84153")
	base := git84153(t, root, "rev-parse", "HEAD")
	write84153(t, root, "a/a.go", "package a\nfunc Value() int { return 12 }\n")
	git84153(t, root, "add", ".")
	git84153(t, root, "commit", "-qm", "new source84153")
	return root, base, git84153(t, root, "rev-parse", "HEAD")
}
func TestObservedImportsAndUnownedInputs84153(t *testing.T) {
	root, base, head := repository84153(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repository, err := Collect(ctx, root, base, head)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(repository.Graph, nil, 1, 0, 180000)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"test:fixture84153.example/domain/a", "test:fixture84153.example/domain/b", "test:fixture84153.example/domain/c"}
	if !reflect.DeepEqual(plan.RequiredTests, want) || plan.Decision != "COST_UNRESOLVED" || plan.ForecastWorkMS != nil {
		t.Fatal(plan)
	}
	write84153(t, root, "README.md", "additional tracked input84153\n")
	git84153(t, root, "add", ".")
	git84153(t, root, "commit", "-qm", "unowned source84153")
	other, err := Collect(ctx, root, head, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	all, err := BuildPlan(other.Graph, nil, 1, 0, 180000)
	if err != nil || len(all.RequiredTests) != 4 || !reflect.DeepEqual(other.GlobalInputChanges, []string{"README.md"}) {
		t.Fatal(all, other.GlobalInputChanges, err)
	}
	other.Commands[all.RequiredTests[0]] = []string{"test", "./..."}
	other.Digest = other.sourceDigest()
	other.Graph.SourceDigest = other.Digest
	if other.Validate() == nil {
		t.Fatal("command projection mutation accepted")
	}
}
func TestExecuteAndFeedback84153(t *testing.T) {
	root, base, head := repository84153(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	repository, err := Collect(ctx, root, base, head)
	if err != nil {
		t.Fatal(err)
	}
	history := []Observation{}
	for _, unit := range repository.Graph.Units {
		if unit.Kind == "test" {
			history = append(history, Observation{ID: "synthetic-cost84153/" + unit.ID, CostKey: unit.CostKey, ElapsedMS: 5000, PredictedMS: 5000, Completed: true})
		}
	}
	estimates, err := Estimates(history)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(repository.Graph, estimates, 1, 5000, 180000)
	if err != nil {
		t.Fatal(err)
	}
	execution, err := Execute(ctx, root, repository, plan, history)
	if err != nil || execution.Decision != "VERIFIED_SCOPED" || execution.NextOperation != "RECORD_ORIGIN" || len(execution.Observations) != 3 || len(execution.Missing) != 0 {
		t.Fatal(execution, err)
	}
	for _, observation := range execution.Observations {
		if !observation.Completed || observation.Outcome != "PASS" || observation.SourceDigest != repository.Digest || observation.PredictedMS != 5000 {
			t.Fatal(observation)
		}
	}
	history = append(history, execution.Observations...)
	next, err := Estimates(history)
	if err != nil {
		t.Fatal(err)
	}
	nextPlan, err := BuildPlan(repository.Graph, next, 1, 5000, 180000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Execute(ctx, root, repository, nextPlan, history); err == nil || !strings.Contains(err.Error(), "already has an execution origin") {
		t.Fatal("historical source rerun admitted", err)
	}
}
