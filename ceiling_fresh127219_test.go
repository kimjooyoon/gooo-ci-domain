package cibudgetdomain

import (
	"context"
	"testing"
	"time"
)

func TestExplicitRequestedCeiling127219(t *testing.T) {
	// The request owns the deadline; three minutes remains a default target.
	if AdmitPlan(221031, 23107, 301219, true, true) != "READY" ||
		AdmitDiscovery(301219, 23107, true, true) != "MEASURE_NEW_COHORT" ||
		AdmitPlan(281117, 23107, 301219, true, true) != "OVER_BUDGET" {
		t.Fatal("explicit bounded work request was replaced by the default target")
	}
	start := time.Now().Add(-200 * time.Second)
	ctx, cancel, err := BudgetContext(context.Background(), start, 301219)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok || deadline != start.Add(301219*time.Millisecond) || ctx.Err() != nil {
		t.Fatal("original request clock reset", deadline, ctx.Err())
	}
	if AdmitPlan(1, 0, 9223372036855, true, true) != "INVALID_DEADLINE" {
		t.Fatal("unrepresentable clock accepted")
	}
}
