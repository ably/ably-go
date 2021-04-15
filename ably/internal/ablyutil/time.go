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
				t, ok := <-after(ctx, d)
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

// ContextWithTimeout is like context.WithTimeout, but using the provided
// TimerFunc for setting the timer.
func ContextWithTimeout(ctx context.Context, after TimerFunc, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	// If the timer expires, we cancel the context. But then we need the context's
	// error to be context.DeadlineExceeded instead of context.Canceled.
	ctx, setErr := newContextWithCustomError(ctx)

	go func() {
		_, timerFired := <-after(ctx, timeout)
		if timerFired {
			setErr(context.DeadlineExceeded)
		} else {
			// after returned because the parent context was canceled, not
			// because the timer fired.
			setErr(ctx.Err())
		}
		cancel()
	}()
	return ctx, cancel
}

type contextWithCustomErr struct {
	context.Context
	err <-chan error
}

func newContextWithCustomError(ctx context.Context) (_ context.Context, setError func(error)) {
	errCh := make(chan error, 1)
	return contextWithCustomErr{ctx, errCh}, func(err error) {
		errCh <- err
	}
}

func (ctx contextWithCustomErr) Err() error {
	select {
	case err := <-ctx.err:
		return err
	default:
		return ctx.Context.Err()
	}
}
