package ablytest

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ably/ably-go/ably"
)

func Wait(res ably.Result, err error) error {
	if err != nil {
		return err
	}
	errch := make(chan error)
	go func() {
		errch <- res.Wait()
	}()
	select {
	case err := <-errch:
		return err
	case <-time.After(Timeout):
		return errors.New("waiting on Result timed out after " + Timeout.String())
	}
}

// ResultGroup is like sync.WaitGroup, but for ably.Result values.
//
// ResultGroup blocks till last added ably.Result has completed successfully.
//
// If at least ably.Result value failed, ResultGroup returns first encountered
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

func (rg *ResultGroup) Add(res ably.Result, err error) {
	if !rg.check(err) {
		return
	}
	rg.wg.Add(1)
	go func() {
		err := res.Wait()
		if err != nil {
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

func ConnWaiter(client *ably.Realtime, do func(), expectedEvent ...ably.ConnectionEvent) ably.Result {
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
			return ResultFunc(func() error { return err })
		}
	}
	return ResultFunc(func() error {
		defer off()
		timer := time.After(Timeout)

		for {
			select {
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

type ResultFunc func() error

func (f ResultFunc) Wait() error {
	return f()
}

func (f ResultFunc) Go() ably.Result {
	err := make(chan error, 1)
	go func() {
		err <- f()
	}()
	return ResultFunc(func() error {
		return <-err
	})
}
