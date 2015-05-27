package ably

import "sync"

func Wait(res Result, err error) error {
	return wait(res, err)
}

func (p *PaginatedResult) BuildPath(base, rel string) (string, error) {
	return p.buildPath(base, rel)
}

func (opts *ClientOptions) RestURL() string {
	return opts.restURL()
}

func (opts *ClientOptions) RealtimeURL() string {
	return opts.realtimeURL()
}

// MustRealtimeClient is like NewRealtimeClient, but panics on error.
func MustRealtimeClient(opts *ClientOptions) *RealtimeClient {
	client, err := NewRealtimeClient(opts)
	if err != nil {
		panic("ably.NewRealtimeClient failed: " + err.Error())
	}
	return client
}

// TestGet is a helper method, which returns attached channel or panics if
// the attaching failed.
func (ch *Channels) TestGet(name string) *RealtimeChannel {
	channel := ch.Get(name)
	if err := Wait(channel.Attach()); err != nil {
		panic(`attach to "` + name + `" failed: ` + err.Error())
	}
	return channel
}

// ResultGroup is like sync.WaitGroup, but for ably.Result values.
// ResultGroup blocks till last added ably.Result has completed successfully.
// If at least ably.Result value failed, ResultGroup returns first encountered
// error immadiately.
type ResultGroup struct {
	wg    sync.WaitGroup
	err   error
	errch chan error
}

func (rg *ResultGroup) init() {
	if rg.errch == nil {
		rg.errch = make(chan error, 1)
	}
}

func (rg *ResultGroup) Add(res Result, err error) {
	rg.init()
	if rg.err != nil {
		return
	}
	if err != nil {
		rg.err = err
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
