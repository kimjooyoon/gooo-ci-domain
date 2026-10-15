package cibudgetdomain

import (
	"context"
	"math"
	"reflect"
	"testing"
	"time"
)

func graph80717() Graph {
	return Graph{SourceDigest: "new-source-manifest80717", Changed: []int{0}, Units: []Unit{
		{ID: "source-a", Kind: "source"}, {ID: "source-b", Kind: "source", Dependencies: []int{0}},
		{ID: "build-a", Kind: "build", CostKey: "build-a/profile80717", Dependencies: []int{0}},
		{ID: "test-a", Kind: "test", CostKey: "test-a/profile80717", Dependencies: []int{2}},
		{ID: "build-b", Kind: "build", CostKey: "build-b/profile80717", Dependencies: []int{1}},
		{ID: "test-b", Kind: "test", CostKey: "test-b/profile80717", Dependencies: []int{4}},
	}}
}

func estimates80717(t *testing.T, graph Graph, duration int64) map[string]Estimate {
	t.Helper()
	result := map[string]Estimate{}
	for _, unit := range graph.Units {
		if unit.Kind == "source" {
			continue
		}
		estimate, err := Calibrate(unit.CostKey, []Observation{{ID: "observed/" + unit.ID,
			CostKey: unit.CostKey, ElapsedMS: duration, PredictedMS: duration, Completed: true}})
		if err != nil {
			t.Fatal(err)
		}
		result[unit.CostKey] = estimate
	}
	return result
}

func TestImpactQuantityAndFanout80717(t *testing.T) {
	graph := graph80717()
	wide, err := BuildPlan(graph, estimates80717(t, graph, 20000), 1, 10000, 180000)
	if err != nil || wide.Decision != "READY" || !reflect.DeepEqual(wide.RequiredTests, []string{"test-a", "test-b"}) {
		t.Fatal("reverse source effects", wide, err)
	}
	graph.Changed = []int{1}
	narrow, err := BuildPlan(graph, estimates80717(t, graph, 20000), 1, 10000, 180000)
	if err != nil || narrow.Decision != "READY" || !reflect.DeepEqual(narrow.RequiredTests, []string{"test-b"}) ||
		wide.ChangedUnits != narrow.ChangedUnits || *wide.ForecastWorkMS <= *narrow.ForecastWorkMS {
		t.Fatal("same patch quantity must retain different causal work", wide, narrow, err)
	}
	for _, selected := range [][]string{{"test-a"}, {"test-a", "test-a"}, {"test-a", "unrelated"}} {
		if err := ValidateSelection(wide, selected); err == nil {
			t.Fatal("omitted, repeated or unrelated obligations accepted", selected)
		}
	}
}

func TestBudgetAndParallelResources80717(t *testing.T) {
	graph := Graph{SourceDigest: "parallel80717", Changed: []int{0}, Units: []Unit{
		{ID: "source", Kind: "source"},
		{ID: "first", Kind: "test", CostKey: "first80717", Dependencies: []int{0}},
		{ID: "second", Kind: "test", CostKey: "second80717", Dependencies: []int{0}},
	}}
	for workers, want := range map[int]string{1: "OVER_BUDGET", 2: "READY"} {
		plan, err := BuildPlan(graph, estimates80717(t, graph, 110000), workers, 30000, 180000)
		if err != nil || plan.Decision != want || len(plan.RequiredTests) != 2 {
			t.Fatal("resource choice changed completeness", workers, plan, err)
		}
	}
	for _, values := range [][3]int64{{0, 0, math.MaxInt64}, {math.MaxInt64, 1, 180000}, {0, -1, 180000}, {0, 180001, 180000}} {
		if AdmitPlan(values[0], values[1], values[2], true, true) == "READY" {
			t.Fatal("deadline or cost arithmetic admitted invalid plan", values)
		}
	}
}

func TestCostFeedbackAndCensoredOrigin80717(t *testing.T) {
	key := "context80717"
	first, err := Calibrate(key, []Observation{{ID: "first", CostKey: key, ElapsedMS: 40000, PredictedMS: 40000, Completed: true}})
	if err != nil || first.ForecastMS != 40000 {
		t.Fatal(first, err)
	}
	next, err := Calibrate(key, []Observation{
		{ID: "first", CostKey: key, ElapsedMS: 40000, PredictedMS: 40000, Completed: true},
		{ID: "slower", CostKey: key, ElapsedMS: 70000, PredictedMS: 40000, Completed: true},
	})
	if err != nil || next.ForecastMS != 100000 || next.ResidualMS != 30000 || len(next.Origins) != 2 {
		t.Fatal("observed underprediction did not improve the next forecast", next, err)
	}
	graph := graph80717()
	estimates := estimates80717(t, graph, 20000)
	censored, err := Calibrate(graph.Units[3].CostKey, []Observation{{ID: "timeout80717", CostKey: graph.Units[3].CostKey, ElapsedMS: 600000}})
	if err != nil || censored.CensoredFloorMS != 600000 || censored.Samples != 0 {
		t.Fatal("timeout became a completed cost sample", censored, err)
	}
	estimates[censored.CostKey] = censored
	plan, err := BuildPlan(graph, estimates, 2, 10000, 180000)
	if err != nil || plan.Decision != "COST_UNRESOLVED" || plan.ForecastWorkMS != nil {
		t.Fatal("censored context received a zero forecast", plan, err)
	}
}

func TestUnknownImpactAndDeadline80717(t *testing.T) {
	graph := graph80717()
	graph.Unresolved = []string{"unbound filesystem input80717"}
	plan, err := BuildPlan(graph, estimates80717(t, graph, 1), 2, 0, 180000)
	if err != nil || plan.Decision != "IMPACT_UNRESOLVED" || len(plan.RequiredTests) != 2 {
		t.Fatal("unknown side effect disappeared", plan, err)
	}
	graph.Unresolved = nil
	graph.Changed = []int{0, 0}
	if _, err := BuildPlan(graph, nil, 2, 0, 180000); err == nil {
		t.Fatal("repeated patch seed accepted")
	}
	ctx, cancel, err := BudgetContext(context.Background(), time.Now().Add(-181*time.Second), 180000)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if ctx.Err() != context.DeadlineExceeded {
		t.Fatal("queue/setup time was silently given a fresh three minutes")
	}
}
