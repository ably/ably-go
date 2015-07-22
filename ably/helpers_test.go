package ably

import (
	"fmt"
	"reflect"
	"sync"
	"time"
)

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

// StateRecorder provides:
//
//   * send ably.State channel for recording state transitions
//   * goroutine-safe access to recorded state enums
//
type StateRecorder struct {
	ch     chan State
	states []StateEnum
	wg     sync.WaitGroup
	done   chan struct{}
	mtx    sync.Mutex
}

// NewStateRecorder gives new recorder which purpose is to record states via
// (*ClientOptions).Listener channel.
//
// If buffer is > 0, the recorder will use it as a buffer to ensure all states
// transitions are received.
// If buffer is <= 0, the recorder will not buffer any states, which can
// result in some of them being dropped.
func NewStateRecorder(buffer int) *StateRecorder {
	if buffer < 0 {
		buffer = 0
	}
	rec := &StateRecorder{
		ch:     make(chan State, buffer),
		done:   make(chan struct{}),
		states: make([]StateEnum, 0, buffer),
	}
	rec.wg.Add(1)
	go rec.processIncomingStates()
	return rec
}

func (rec *StateRecorder) processIncomingStates() {
	defer rec.wg.Done()
	for {
		select {
		case state, ok := <-rec.ch:
			if !ok {
				return
			}
			rec.add(state.State)
		case <-rec.done:
			return
		}
	}
}

// Add appends state to the list of recorded ones, used to ensure ordering
// of the states by injecting values at certain points of the test.
func (rec *StateRecorder) Add(state StateEnum) {
	rec.ch <- State{State: state}
}

func (rec *StateRecorder) add(state StateEnum) {
	rec.mtx.Lock()
	rec.states = append(rec.states, state)
	rec.mtx.Unlock()
}

func (rec *StateRecorder) Channel() chan<- State {
	return rec.ch
}

// Stop stops separate recording gorouting and waits until it terminates.
func (rec *StateRecorder) Stop() {
	close(rec.done)
	rec.wg.Wait()
	// Drain listener channel.
	for {
		select {
		case <-rec.ch:
		default:
			return
		}
	}
}

// States gives copy of the recorded states, safe for use while the recorder
// is still running.
func (rec *StateRecorder) States() []StateEnum {
	rec.mtx.Lock()
	defer rec.mtx.Unlock()
	states := make([]StateEnum, len(rec.states))
	copy(states, rec.states)
	return states
}

// WaitFor blocks until we observe the given exact states were recorded.
func (rec *StateRecorder) WaitFor(states []StateEnum, timeout time.Duration) error {
	done := make(chan struct{})
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				if recorded := rec.States(); reflect.DeepEqual(states, recorded) {
					close(done)
					return
				}
				time.Sleep(20 * time.Millisecond)
			}
		}
	}()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		close(stop)
		return fmt.Errorf("WaitFor(%v) has timed out after %v: recorded states were %v",
			states, timeout, rec.States())
	}
}

// MustRealtimeClient is like NewRealtimeClient, but panics on error.
func MustRealtimeClient(opts *ClientOptions) *RealtimeClient {
	client, err := NewRealtimeClient(opts)
	if err != nil {
		panic("ably.NewRealtimeClient failed: " + err.Error())
	}
	return client
}

// GetAndAttach is a helper method, which returns attached channel or panics if
// the attaching failed.
func (ch *Channels) GetAndAttach(name string) *RealtimeChannel {
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
