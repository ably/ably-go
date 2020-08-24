package ablyutil

import (
	"context"
	"time"
)

// After returns a channel that is sent the current time once the given
// duration has passed. If the context is cancelled before that, the channel
// is immediately closed.
func After(ctx context.Context, d time.Duration) <-chan time.Time {
	timer := time.NewTimer(d)

	ch := make(chan time.Time)

	go func() {
		defer timer.Stop()
		select {
		case <-ctx.Done():
			close(ch)
		case t := <-timer.C:
			ch <- t
		}
	}()

	return ch
}
