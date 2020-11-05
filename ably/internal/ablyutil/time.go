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

	ch := make(chan time.Time, 1)

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

type TimerFunc func(context.Context, time.Duration) <-chan time.Time

// NewTicker repeatedly calls the given TimerFunc, which should behave like
// After, until the context it cancelled. It returns a channel to which it sends
// every value produced by the TimerFunc.
func NewTicker(after TimerFunc) TimerFunc {
	return func(ctx context.Context, d time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)

		go func() {
			for {
				t, ok := <-After(ctx, d)
				if !ok {
					close(ch)
					return
				}
				ch <- t
			}
		}()

		return ch
	}
}
