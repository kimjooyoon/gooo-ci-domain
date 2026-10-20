package cibudgetdomain

import (
	"context"
	"testing"
	"time"
)

func TestRuntimeGoooInputOrigin94517(t *testing.T) {
	root := t.TempDir()
	write83147(t, root, "go.mod", "module source94517.example/gooo\n\ngo 1.27.3\n")
	write83147(t, root, "source.go", "package source94517\nfunc Revision() int {return 19}\n")
	write83147(t, root, "policy.gooo", "activity Evaluate(Integer) -> Integer computes `return 19`\n")
	write83147(t, root, "source_test.go", "package source94517\nimport(\"testing\";\"os\";\"strings\")\nfunc TestSource94517(t *testing.T){source,err:=os.ReadFile(\"policy.gooo\");if err!=nil||!strings.Contains(string(source),\"return 23\"){t.Fatal(err,string(source))}}\n")
	git83147(t, root, "init", "-q")
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "origin base94517")
	base := git83147(t, root, "rev-parse", "HEAD")
	write83147(t, root, "policy.gooo", "activity Evaluate(Integer) -> Integer computes `return 23`\n")
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "new gooo input94517")
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repository, err := Collect(ctx, root, base, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	execution, err := Improve(ctx, root, repository, nil, start, 180000)
	if err != nil || execution.Decision != "VERIFIED_DISCOVERY" {
		t.Fatal(execution, err)
	}
	found := false
	for _, observation := range execution.Observations {
		if observation.Unit == "test:source94517.example/gooo" {
			if observation.InputLogState != "OBSERVED_OS_API" {
				t.Fatal(observation)
			}
			for _, origin := range observation.InputOrigins {
				if origin.Operation == "open" && origin.Path == "policy.gooo" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("runtime input origin disappeared", execution.Observations)
	}
}
