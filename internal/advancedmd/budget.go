package advancedmd

import (
	"context"
	"time"
)

const reconciliationReserve = 5 * time.Second

// Mutations stop before the workflow deadline. Reconciliation uses the original
// caller context, so an uncertain write can still be checked without retrying it.
func mutationContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, classify(err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		deadline = deadline.Add(-reconciliationReserve)
		if !time.Now().Before(deadline) {
			return nil, nil, classify(context.DeadlineExceeded)
		}
		ctx, cancel := context.WithDeadline(ctx, deadline)
		return ctx, cancel, nil
	}
	ctx, cancel := context.WithCancel(ctx)
	return ctx, cancel, nil
}
