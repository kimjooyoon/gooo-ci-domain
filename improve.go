package cibudgetdomain

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"
)

func controlCostKey(profile string) string {
	return hashBytes([]byte(profile + "\x00rebind-and-completion/v1"))
}

// Improve measures a first-seen complete obligation population under the same
// deadline when prior costs are unavailable. A later source uses those origins
// to forecast work. No timed-out or historical cohort becomes a current pass.
func Improve(ctx context.Context, root string, repository Repository, history []Observation, started time.Time, deadline int64) (Execution, error) {
	if err := repository.Validate(); err != nil {
		return Execution{}, err
	}
	plan, control, known, err := ObservedPlan(repository, history, time.Since(started).Milliseconds(), deadline)
	if err != nil {
		return Execution{}, err
	}
	prefix := time.Since(started).Milliseconds()
	result := Execution{Decision: plan.Decision, NextOperation: NextStep(plan.Decision), SourceDigest: repository.Digest, Plan: plan}
	novel := true
	for _, old := range history {
		if old.SourceDigest == repository.Digest || old.SourceTree == repository.Tree && old.Profile == repository.Profile {
			novel = false
		}
	}
	if !novel {
		return result, fmt.Errorf("this exact source cohort already has an execution origin; advance source before retrying")
	}
	if plan.Decision != "READY" && plan.Decision != "COST_UNRESOLVED" {
		return result, nil
	}
	discovery := plan.Decision == "COST_UNRESOLVED"
	if discovery {
		decision := AdmitDiscovery(deadline, prefix, len(plan.Unresolved) == 0, novel)
		if decision != "MEASURE_NEW_COHORT" {
			result.Decision = decision
			result.NextOperation = NextStep(decision)
			return result, nil
		}
	}
	controlStart := time.Now()
	fresh, err := CollectWithWorkers(ctx, root, repository.Base, repository.Head, repository.Workers)
	if err != nil {
		return result, err
	}
	if fresh.Digest != repository.Digest {
		return result, fmt.Errorf("live source or execution profile differs")
	}
	workStart := time.Now()
	result = runUnits(ctx, root, repository, plan, result)
	workElapsed := time.Since(workStart)
	elapsed := time.Since(controlStart) - workElapsed
	observation := Observation{ID: repository.Digest + "/scope-control/" + strconv.FormatInt(controlStart.UnixNano(), 10), CostKey: controlCostKey(repository.Profile),
		SourceDigest: repository.Digest, SourceTree: repository.Tree, Profile: repository.Profile, Unit: "scope-control", ElapsedMS: elapsed.Milliseconds(), ResolutionMS: 1, Completed: ctx.Err() == nil, Outcome: result.Decision, PredictionState: "unavailable"}
	if known {
		observation.PredictedMS = control.ForecastMS
		observation.PredictionState = "empirical"
	}
	result.Observations = append(result.Observations, observation)
	if discovery && result.Decision == "VERIFIED_SCOPED" {
		result.Decision = "VERIFIED_DISCOVERY"
	}
	result.NextOperation = NextStep(result.Decision)
	result.ScopeElapsedMS = time.Since(started).Milliseconds()
	return result, nil
}

// Already elapsed queue/setup/collection time is observed at this invocation.
// The rebind/completion reserve is calibrated from system-owned prior origins.
func ObservedPlan(repository Repository, history []Observation, prefix, deadline int64) (Plan, Estimate, bool, error) {
	estimates, err := Estimates(history)
	if err != nil {
		return Plan{}, Estimate{}, false, err
	}
	control, known := estimates[controlCostKey(repository.Profile)]
	known = known && control.Samples > 0 && (control.Censored == 0 || control.ObservedMaxMS > control.CensoredFloorMS)
	overhead := prefix
	if known {
		if control.ForecastMS > math.MaxInt64-prefix {
			return Plan{}, control, known, fmt.Errorf("control forecast exceeds representation")
		}
		overhead += control.ForecastMS
	}
	plan, err := BuildPlan(repository.Graph, estimates, repository.Workers, overhead, deadline)
	if err != nil {
		return plan, control, known, err
	}
	if !known {
		plan.UnknownCosts = append(plan.UnknownCosts, CostCause{"scope-control", controlCostKey(repository.Profile), "COST_CONTEXT_UNAVAILABLE"})
		if plan.Decision == "READY" {
			plan.Decision = AdmitPlan(*plan.ForecastWorkMS, overhead, deadline, true, false)
		}
	}
	return plan, control, known, nil
}
