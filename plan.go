package cibudgetdomain

import "fmt"

func BuildPlan(graph Graph, estimates map[string]Estimate, workers int, overhead, deadline int64) (Plan, error) {
	plan := Plan{SourceDigest: graph.SourceDigest, DeadlineMS: deadline, OverheadMS: overhead, Workers: workers,
		ChangedUnits: len(graph.Changed), Unresolved: append([]string(nil), graph.Unresolved...)}
	reverse, err := validateGraph(graph)
	if err != nil {
		return plan, err
	}
	affected, tests, err := impactClosure(graph, reverse)
	if err != nil {
		return plan, err
	}
	for _, selected := range affected {
		if selected {
			plan.AffectedUnits++
		}
	}
	for _, index := range tests {
		plan.RequiredTests = append(plan.RequiredTests, graph.Units[index].ID)
	}
	if len(tests) == 0 {
		plan.Unresolved = append(plan.Unresolved, "NO_BOUND_TEST_OBLIGATION")
	}
	required := prerequisiteClosure(graph, tests)
	costs, known := planCosts(graph, required, estimates, &plan)
	slots, wall, err := schedule(graph, required, costs, workers)
	if err != nil {
		return plan, err
	}
	if known {
		bounds, boundsErr := workBounds(graph, required, costs, workers)
		if boundsErr != nil {
			return plan, boundsErr
		}
		plan.CostBounds = &bounds
		plan.Slots, plan.ForecastWorkMS = slots, &wall
	}
	plan.Decision = AdmitPlan(wall, overhead, deadline, len(plan.Unresolved) == 0, known)
	return plan, nil
}

func planCosts(graph Graph, required []bool, estimates map[string]Estimate, plan *Plan) ([]int64, bool) {
	costs, known := make([]int64, len(graph.Units)), true
	for index, selected := range required {
		unit := graph.Units[index]
		if !selected || unit.Kind == "source" {
			continue
		}
		plan.RequiredExecution = append(plan.RequiredExecution, unit.ID)
		estimate, exists := estimates[unit.CostKey]
		if !exists || unit.CostKey == "" || estimate.CostKey != unit.CostKey || estimate.Samples < 1 ||
			estimate.Censored != 0 && estimate.ObservedMaxMS <= estimate.CensoredFloorMS || estimate.ForecastMS < 0 {
			reason := "COST_CONTEXT_UNAVAILABLE"
			if exists && estimate.Censored > 0 {
				reason = "CENSORED_COST_UNRESOLVED"
			}
			plan.UnknownCosts = append(plan.UnknownCosts, CostCause{unit.ID, unit.CostKey, reason})
			known = false
			continue
		}
		plan.TimingOrigins = append(plan.TimingOrigins, estimate.Origins...)
		costs[index] = estimate.ForecastMS
	}
	return costs, known
}

func ValidateSelection(plan Plan, selected []string) error {
	wanted := map[string]bool{}
	for _, id := range plan.RequiredTests {
		wanted[id] = true
	}
	if len(selected) != len(wanted) {
		return fmt.Errorf("selected tests differ from the derived obligation population")
	}
	for _, id := range selected {
		if !wanted[id] {
			return fmt.Errorf("unknown, omitted or repeated test obligation")
		}
		delete(wanted, id)
	}
	return nil
}
