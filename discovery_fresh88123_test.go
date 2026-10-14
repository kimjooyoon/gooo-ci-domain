package cibudgetdomain

import (
	"context"
	"testing"
	"time"
)

func TestAutomaticCostDiscoveryAndNextSource88123(t *testing.T) {
	root := t.TempDir()
	write83147(t, root, "go.mod", "module discovery88123.example/g\n\ngo 1.27.2\n")
	write83147(t, root, "core/core.go", "package core\nfunc Value() int { return 31 }\n")
	write83147(t, root, "adapter/adapter.go", "package adapter\nimport \"discovery88123.example/g/core\"\nfunc Value() int {return core.Value()+7}\n")
	write83147(t, root, "adapter/adapter_test.go", "package adapter\nimport \"testing\"\nfunc TestAdapt88123(t *testing.T){if Value()!=50{t.Fatal(Value())}}\n")
	git83147(t, root, "init", "-q")
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "fresh discovery base88123")
	base := git83147(t, root, "rev-parse", "HEAD")
	write83147(t, root, "core/core.go", "package core\nfunc Value() int { return 43 }\n")
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "new discovery input88123")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	start := time.Now()
	repository, err := CollectWithWorkers(ctx, root, base, "HEAD", 2)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Improve(ctx, root, repository, nil, start, 180000)
	if err != nil || first.Decision != "VERIFIED_DISCOVERY" || first.Plan.Decision != "COST_UNRESOLVED" || first.Plan.ForecastWorkMS != nil || len(first.Observations) != 3 || len(first.Plan.UnknownCosts) != 3 {
		t.Fatal(first, err)
	}
	for _, observation := range first.Observations {
		if !observation.Completed || observation.PredictionState != "unavailable" || observation.SourceTree != repository.Tree {
			t.Fatal(observation)
		}
	}
	t.Logf("first source: prediction unavailable; complete packages=%d; observed origins=%d; scope_ms=%d", len(first.Plan.RequiredTests), len(first.Observations), first.ScopeElapsedMS)
	oldHead := repository.Head
	write83147(t, root, "core/core.go", "package core\nfunc Value() int { return 47 }\n")
	write83147(t, root, "adapter/adapter_test.go", "package adapter\nimport \"testing\"\nfunc TestAdapt88123(t *testing.T){if Value()!=54{t.Fatal(Value())}}\n")
	git83147(t, root, "add", ".")
	git83147(t, root, "commit", "-qm", "next functional source88123")
	nextStart := time.Now()
	nextRepository, err := CollectWithWorkers(ctx, root, oldHead, "HEAD", 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Improve(ctx, root, nextRepository, first.Observations, nextStart, 180000)
	if err != nil || second.Decision != "VERIFIED_SCOPED" || second.Plan.Decision != "READY" || second.Plan.ForecastWorkMS == nil || len(second.Observations) != 3 {
		t.Fatal(second, err)
	}
	for _, observation := range second.Observations {
		if !observation.Completed || observation.PredictionState != "empirical" {
			t.Fatal(observation)
		}
	}
	t.Logf("next source: forecast_work_ms=%d; observed scope_ms=%d; required_packages=%d", *second.Plan.ForecastWorkMS, second.ScopeElapsedMS, len(second.Plan.RequiredTests))
	for _, values := range [][4]int64{{180001, 0, 1, 1}, {180000, 180000, 1, 1}, {180000, 0, 0, 1}, {180000, 0, 1, 0}} {
		if AdmitDiscovery(values[0], values[1], values[2] == 1, values[3] == 1) == "MEASURE_NEW_COHORT" {
			t.Fatal("invalid discovery admitted", values)
		}
	}
}
