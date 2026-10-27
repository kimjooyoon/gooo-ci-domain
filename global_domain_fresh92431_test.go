package cibudgetdomain

import (
	"fmt"
	"testing"
)

func TestFactoredGlobalInputsPreserveObligations92431(t *testing.T) {
	repository := Repository{Digest: "fresh-domain92431", Profile: "declared-runtime92431", Workers: 2}
	for index := range 512 {
		repository.Packages = append(repository.Packages, PackageInput{ImportPath: fmt.Sprintf("domain92431.example/p%d", index), Dir: fmt.Sprintf("p%d", index), GoFiles: []string{"source.go"}})
	}
	for index := range 400 {
		repository.Changes = append(repository.Changes, fmt.Sprintf("input-%d.gooo", index))
	}
	if err := repository.deriveGraph(); err != nil {
		t.Fatal(err)
	}
	edges := 0
	for _, unit := range repository.Graph.Units {
		edges += len(unit.Dependencies)
	}
	plan, err := BuildPlan(repository.Graph, nil, 2, 0, 180000)
	if err != nil || len(plan.RequiredTests) != 512 || len(repository.Graph.Changed) != 400 || len(repository.GlobalInputChanges) != 400 || edges >= 3000 {
		t.Fatal("global scope population was expanded or dropped", len(plan.RequiredTests), edges, err)
	}
	if plan.ForecastWorkMS != nil || plan.Decision != "COST_UNRESOLVED" {
		t.Fatal("missing observations got a fabricated ETA", plan)
	}
	t.Logf("inputs=400; required_packages=512; dependency_edges=%d; unfactored_global_edges=204800", edges)
}
