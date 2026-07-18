package devices

import (
	"context"
	"fmt"
	"time"
)

// waitFor polls check until it returns true, the timeout elapses, or ctx is
// canceled. It replaces fixed sleeps in device provisioning: a fast device
// proceeds as soon as the condition holds, a slow one gets the full window,
// and a genuinely broken one fails with a named error instead of limping
// into "live" with a dead service. check runs once immediately, so an
// already-true condition costs no wait at all.
func waitFor(ctx context.Context, timeout, interval time.Duration, label string, check func() bool) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		if check() {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for %s: %w", label, ctx.Err())
		case <-deadline.C:
			return fmt.Errorf("wait for %s: timed out after %s", label, timeout)
		case <-tick.C:
		}
	}
}
