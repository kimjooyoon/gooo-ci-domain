package cibudgetdomain

import "testing"

func TestCriticalWorkCannotBeHiddenByWorkers84279(t *testing.T) {
	graph := Graph{SourceDigest: "chain84279", Changed: []int{0}, Units: []Unit{
		{ID: "input", Kind: "source"}, {ID: "first", Kind: "build", CostKey: "first84279", Dependencies: []int{0}},
		{ID: "second", Kind: "build", CostKey: "second84279", Dependencies: []int{1}},
		{ID: "assertions", Kind: "test", CostKey: "last84279", Dependencies: []int{2}},
	}}
	estimates := map[string]Estimate{}
	for key, duration := range map[string]int64{"first84279": 51000, "second84279": 67000, "last84279": 73000} {
		estimate, err := Calibrate(key, []Observation{{ID: key, CostKey: key, ElapsedMS: duration, PredictedMS: duration, Completed: true}})
		if err != nil {
			t.Fatal(err)
		}
		estimates[key] = estimate
	}
	plan, err := BuildPlan(graph, estimates, 8, 1000, 180000)
	if err != nil || plan.Decision != "OVER_BUDGET" || plan.CostBounds.CriticalPathMS != 191000 || plan.CostBounds.MinimumMS != 191000 || *plan.ForecastWorkMS != 191000 {
		t.Fatal(plan, err)
	}
	if len(plan.RequiredTests) != 1 || len(plan.RequiredExecution) != 3 {
		t.Fatal("work removed to fit the deadline", plan)
	}
}
