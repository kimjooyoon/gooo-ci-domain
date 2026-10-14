package cibudgetdomain

import (
	"context"
	"fmt"
	"time"
)

func BudgetContext(parent context.Context, scopeStarted time.Time, budgetMS int64) (context.Context, context.CancelFunc, error) {
	if AdmitPlan(0, 0, budgetMS, true, true) != "READY" || scopeStarted.After(time.Now()) {
		return nil, nil, fmt.Errorf("deadline differs from the source-owned budget")
	}
	ctx, cancel := context.WithDeadline(parent, scopeStarted.Add(time.Duration(budgetMS)*time.Millisecond))
	return ctx, cancel, nil
}
