package ablytest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ably/ably-go/ably"
)

type Result interface {
	Wait(context.Context) error
}

func Wait(res Result, err error) error {
	if err != nil {
		return err
	}
	errch := make(chan error)
	go func() {
		errch <- res.Wait(context.Background())
	}()
	select {
	case err := <-errch:
		return err
	case <-time.After(Timeout):
		return errors.New("waiting on Result timed out after " + Timeout.String())
	}
}

// ResultGroup is like sync.WaitGroup, but for Result values.
//
// ResultGroup blocks till last added Result has completed successfully.
//
// If at least Result value failed, ResultGroup returns first encountered
// error immediately.
type ResultGroup struct {
	mu    sync.Mutex
	wg    sync.WaitGroup
	err   error
	errch chan error
}

func (rg *ResultGroup) check(err error) bool {
	rg.mu.Lock()
	defer rg.mu.Unlock()
	if rg.errch == nil {
		rg.errch = make(chan error, 1)
	}
	rg.err = nonil(rg.err, err)
	return rg.err == nil
}

func (rg *ResultGroup) Add(res Result, err error) {
	if !rg.check(err) {
		return
	}
	rg.wg.Add(1)
	go func() {
		err := res.Wait(context.Background())
		if err != nil && err != (*ably.ErrorInfo)(nil) {
			select {
			case rg.errch <- err:
			default:
			}
		}
		rg.wg.Done()
	}()
}

func (rg *ResultGroup) GoAdd(f ResultFunc) {
	rg.Add(f.Go(), nil)
}

func (rg *ResultGroup) Wait() error {
	if rg.err != nil {
		return rg.err
	}
	done := make(chan struct{})
	go func() {
		rg.wg.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
		return nil
	case err := <-rg.errch:
		return err
	}
}

type resultFunc func(context.Context) error

func (f resultFunc) Wait(ctx context.Context) error {
	return f(ctx)
}

func ValueWaiter(generator func() interface{}, expectedValue interface{}) Result {
	return resultFunc(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				time.Sleep(time.Millisecond * 200)
				value := generator()
				if value == expectedValue {
					return nil
				}
			}
		}
	})
}
func ConnWaiter(client *ably.Realtime, do func(), expectedEvent ...ably.ConnectionEvent) Result {
	change := make(chan ably.ConnectionStateChange, 1)
	off := client.Connection.OnAll(func(ev ably.ConnectionStateChange) {
		change <- ev
	})
	if do != nil {
		do()
	}
	for _, ev := range expectedEvent {
		if ev == ably.ConnectionEvent(client.Connection.State()) {
			var err error
			if errInfo := client.Connection.ErrorReason(); errInfo != nil {
				err = errInfo
			}
			return ResultFunc(func(context.Context) error { return err })
		}
	}
	return ResultFunc(func(ctx context.Context) error {
		defer off()
		timer := time.After(Timeout)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()

			case <-timer:
				return fmt.Errorf("timeout waiting for event %v", expectedEvent)

			case change := <-change:
				if change.Reason != nil {
					return change.Reason
				}

				if len(expectedEvent) > 0 {
					for _, ev := range expectedEvent {
						if ev == change.Event {
							return nil
						}
					}
					continue
				}

				return nil
			}
		}
	})
}

type ResultFunc func(context.Context) error

func (f ResultFunc) Wait(ctx context.Context) error {
	return f(ctx)
}

func (f ResultFunc) Go() Result {
	err := make(chan error, 1)
	go func() {
		err <- f(context.Background())
	}()
	return ResultFunc(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-err:
			return err
		}
	})
}
