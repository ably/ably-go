//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"errors"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"
)

type ConnTransitioner struct {
	Realtime *ably.Realtime

	t              *testing.T
	dialErr        chan<- error
	fakeDisconnect func() error
	intercept      func(context.Context, ...ably.ProtoAction) <-chan *ably.ProtocolMessage
	afterCalls     <-chan ablytest.AfterCall

	next connNextStates
}

// TransitionConn makes a connection with external events (requests, protocol
// messages, timers) mocked, so that state transitions can be controlled by the
// tests. The returned ConnTransitioner's To method moves the connection to the
// desired state, and keeps it there.
func TransitionConn(t *testing.T, dial DialFunc, options ...ably.ClientOption) (ConnTransitioner, io.Closer) {
	t.Helper()

	c := ConnTransitioner{t: t}

	dial, disconnect := DialFakeDisconnect(dial)
	c.fakeDisconnect = disconnect

	dial, intercept := DialIntercept(dial)
	c.intercept = intercept

	dialErr := make(chan error, 1)
	prevDial := dial
	dial = func(proto string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
		if err := <-dialErr; err != nil {
			return nil, err
		}
		return prevDial(proto, u, timeout)
	}
	c.dialErr = dialErr

	afterCalls := make(chan ablytest.AfterCall, 2)
	c.afterCalls = afterCalls
	now, after := ablytest.TimeFuncs(afterCalls)

	realtime, err := ably.NewRealtime(append(options,
		ably.WithAutoConnect(false),
		ably.WithDial(dial),
		ably.WithNow(now),
		ably.WithAfter(after),
	)...)
	if err != nil {
		t.Fatal(err)
	}
	c.Realtime = realtime

	c.next = connNextStates{
		connecting: c.connect,
	}

	return c, ablytest.FullRealtimeCloser(c.Realtime)
}

func (c ConnTransitioner) Channel(name string) ChanTransitioner {
	chanStateErrMap := map[ably.ChannelState]chan error{}

	// add mapping to channel states to capture respective errors
	chanStateErrMap[chAttaching] = make(chan error, 1)
	chanStateErrMap[chDetaching] = make(chan error, 1)

	transitioner := ChanTransitioner{c, c.Realtime.Channels.Get(name), chanStateErrMap, nil}
	transitioner.next = chanNextStates{
		chAttaching: transitioner.attach,
	}
	return transitioner
}

func (c *ConnTransitioner) To(path ...ably.ConnectionState) io.Closer {
	c.t.Helper()

	from := c.Realtime.Connection.State()

	var cleanUp func()

	for _, to := range path {
		if from == to {
			continue
		}

		transition, ok := c.next[to]
		if !ok {
			c.t.Fatalf("no transition from %v to %v", from, to)
		}
		c.next, cleanUp = transition()
		from = to
		c.assertState(from)
	}

	return closeFunc(func() error {
		if cleanUp != nil {
			cleanUp()
		}
		return nil
	})
}

func (c ConnTransitioner) assertState(state ably.ConnectionState) {
	c.t.Helper()
	if expected, got := state, c.Realtime.Connection.State(); expected != got {
		c.t.Fatalf("expected connection to be %v; is %v", expected, got)
	}
}

func (c ConnTransitioner) connect() (connNextStates, func()) {
	change := make(ably.ConnStateChanges, 1)
	c.Realtime.Connection.Once(ably.ConnectionEventConnecting, change.Receive)

	c.Realtime.Connect()

	ablytest.Instantly.Recv(c.t, nil, change, c.t.Fatalf)

	return connNextStates{
			connected:    c.finishConnecting(nil),
			disconnected: c.finishConnecting(errors.New("fake connection error")),
			failed:       c.fail,
		}, func() {
			c.Realtime.Close() // don't reconnect
			c.finishConnecting(errors.New("test done"))
		}
}

func (c ConnTransitioner) finishConnecting(err error) connTransitionFunc {
	return func() (connNextStates, func()) {
		change := make(ably.ConnStateChanges, 1)
		c.Realtime.Connection.OnceAll(change.Receive)

		c.dialErr <- err

		ablytest.Soon.Recv(c.t, nil, change, c.t.Fatalf)

		if err != nil {
			// Should be DISCONNECTED.

			// Expect to timers: one for reconnecting, another for suspending.
			reconnect, suspend := ablytest.GetReconnectionTimersFrom(c.t, c.afterCalls)

			return connNextStates{
					connecting: c.recover(reconnect),
					suspended:  c.fireSuspend(reconnect, suspend),
				}, func() {
					c.Realtime.Close() // cancel timers; don't reconnect
				}
		}

		// Should be CONNECTED.
		return connNextStates{
			disconnected: c.disconnect,
			closing:      c.close,
		}, nil
	}
}

func (c ConnTransitioner) failOnIntercept(msg <-chan *ably.ProtocolMessage, cancelIntercept func()) connTransitionFunc {
	return func() (connNextStates, func()) {

		var incomingMsg *ably.ProtocolMessage
		ablytest.Soon.Recv(c.t, &incomingMsg, msg, c.t.Fatalf)

		incomingMsg.Action = ably.ActionError
		incomingMsg.Error = &ably.ProtoErrorInfo{
			StatusCode: 400,
			Code:       int(ably.ErrUnauthorized),
			Message:    "fake error",
		}

		change := make(ably.ConnStateChanges, 1)
		c.Realtime.Connection.Once(ably.ConnectionEventFailed, change.Receive)

		cancelIntercept()

		ablytest.Instantly.Recv(c.t, nil, change, c.t.Fatalf)

		return nil, nil
	}
}

func (c ConnTransitioner) fail() (connNextStates, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), ablytest.Timeout)
	defer cancel()

	intercepted := c.intercept(ctx, ably.ActionConnected)

	c.dialErr <- nil

	msg := <-intercepted
	msg.Action = ably.ActionError
	msg.Error = &ably.ProtoErrorInfo{
		StatusCode: 400,
		Code:       int(ably.ErrUnauthorized),
		Message:    "fake error",
	}

	change := make(ably.ConnStateChanges, 1)
	c.Realtime.Connection.Once(ably.ConnectionEventFailed, change.Receive)
	cancel()
	ablytest.Instantly.Recv(c.t, nil, change, c.t.Fatalf)

	return nil, nil
}

func (c ConnTransitioner) disconnect() (connNextStates, func()) {
	c.fakeDisconnect()
	return c.finishConnecting(errors.New("can't reconnect"))()
}

func (c ConnTransitioner) recover(timer ablytest.AfterCall) connTransitionFunc {
	return func() (connNextStates, func()) {
		change := make(ably.ConnStateChanges, 1)
		c.Realtime.Connection.Once(ably.ConnectionEventConnecting, change.Receive)

		timer.Fire()

		ablytest.Instantly.Recv(c.t, nil, change, c.t.Fatalf)

		return connNextStates{
				connected: c.finishConnecting(nil),
			}, func() {
				c.Realtime.Close() // don't reconnect
				c.finishConnecting(errors.New("test done"))
			}
	}
}

func (c ConnTransitioner) fireSuspend(reconnectTimer ablytest.AfterCall, suspendTimer ablytest.AfterCall) connTransitionFunc {
	return func() (connNextStates, func()) {
		suspendTimer.Fire()
		err := ablytest.Wait(ablytest.AssertionWaiter(func() bool { // Make sure suspend/connectionStateTTL timer is triggered first
			return suspendTimer.IsTriggered()
		}), nil)
		if err != nil {
			c.t.Fatalf("Failed to trigger suspend timeout with error %v", err)
		}
		reconnectTimer.Fire()
		err = ablytest.Wait(ablytest.AssertionWaiter(func() bool { // Make sure reconnect timer is triggered
			return reconnectTimer.IsTriggered()
		}), nil)
		if err != nil {
			c.t.Fatalf("Failed to trigger reconnect timeout with error %v", err)
		}
		return c.suspend()
	}
}

func (c ConnTransitioner) suspend() (connNextStates, func()) {
	change := make(ably.ConnStateChanges, 1)
	c.Realtime.Connection.Once(ably.ConnectionEventSuspended, change.Receive)

	var timer ablytest.AfterCall
	ablytest.Instantly.Recv(c.t, &timer, c.afterCalls, c.t.Fatalf) // Get timer with suspended timeout
	timer.Fire()

	c.dialErr <- errors.New("suspending")

	ablytest.Instantly.Recv(c.t, nil, change, c.t.Fatalf)

	var reconnectTimer ablytest.AfterCall
	ablytest.Instantly.Recv(c.t, &reconnectTimer, c.afterCalls, c.t.Fatalf)

	return connNextStates{
			connecting: c.reconnectSuspended(reconnectTimer),
		}, func() {
			c.Realtime.Close() // don't reconnect
			c.finishConnecting(errors.New("test done"))
		}
}

func (c ConnTransitioner) reconnectSuspended(timer ablytest.AfterCall) connTransitionFunc {
	return func() (connNextStates, func()) {
		change := make(ably.ConnStateChanges, 1)
		c.Realtime.Connection.Once(ably.ConnectionEventConnecting, change.Receive)

		timer.Fire()

		ablytest.Instantly.Recv(c.t, nil, change, c.t.Fatalf)

		return connNextStates{
				connected: c.finishConnecting(nil),
				suspended: c.suspend,
			}, func() {
				c.Realtime.Close() // don't reconnect
				c.finishConnecting(errors.New("test done"))
			}
	}
}

func (c ConnTransitioner) close() (connNextStates, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), ablytest.Timeout)
	msg := c.intercept(ctx, ably.ActionClosed)

	change := make(ably.ConnStateChanges, 1)
	c.Realtime.Connection.Once(ably.ConnectionEventClosing, change.Receive)

	c.Realtime.Close()

	ablytest.Instantly.Recv(c.t, nil, change, c.t.Fatalf)

	return connNextStates{
		closed: c.finishClosing(cancel),
		failed: c.failOnIntercept(msg, cancel),
	}, cancel
}

func (c ConnTransitioner) finishClosing(cancelIntercept func()) connTransitionFunc {
	return func() (connNextStates, func()) {

		change := make(ably.ConnStateChanges, 1)
		c.Realtime.Connection.Once(ably.ConnectionEventClosed, change.Receive)

		cancelIntercept()

		ablytest.Soon.Recv(c.t, nil, change, c.t.Fatalf)

		return nil, nil
	}
}

type connTransitionFunc func() (next connNextStates, cleanUp func())

type connNextStates map[ably.ConnectionState]connTransitionFunc

type ChanTransitioner struct {
	ConnTransitioner
	Channel *ably.RealtimeChannel
	err     map[ably.ChannelState]chan error
	next    chanNextStates
}

func (c *ChanTransitioner) To(path ...ably.ChannelState) (*ably.RealtimeChannel, io.Closer) {
	c.t.Helper()

	from := c.Channel.State()
	c.assertState(from)

	var cleanUp func()

	for _, to := range path {
		if from == to {
			continue
		}

		transition, ok := c.next[to]
		if !ok {
			c.t.Fatalf("no transition from %v to %v", from, to)
		}
		c.next, cleanUp = transition()
		from = to
	}

	return c.Channel, closeFunc(func() error {
		if cleanUp != nil {
			cleanUp()
		}
		return nil
	})
}

func (c ChanTransitioner) assertState(state ably.ChannelState) {
	c.t.Helper()
	if expected, got := state, c.Channel.State(); expected != got {
		c.t.Fatalf("expected channel to be %v; is %v", expected, got)
	}
}

func (c ChanTransitioner) attach() (chanNextStates, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), ablytest.Timeout)
	msg := c.intercept(ctx, ably.ActionAttached)

	change := make(ably.ChannelStateChanges, 1)
	c.Channel.Once(ably.ChannelEventAttaching, change.Receive)

	async(func() error {
		err := c.Channel.Attach(context.Background())
		c.err[chAttaching] <- err
		return err
	})

	// Attach sets a timeout; discard it.
	ablytest.Soon.Recv(c.t, nil, c.afterCalls, c.t.Fatalf)

	ablytest.Instantly.Recv(c.t, nil, change, c.t.Fatalf)

	return chanNextStates{
		chAttached: c.finishAttach(msg, cancel, nil, chAttached),
		chFailed:   c.finishAttach(msg, cancel, &ably.ProtoErrorInfo{Message: "fake error", Code: 50001}, chFailed),
		chDetached: c.finishAttach(msg, cancel, &ably.ProtoErrorInfo{Message: "fake error", Code: 50001}, chDetached),
	}, cancel
}

func (c ChanTransitioner) finishAttach(msg <-chan *ably.ProtocolMessage, cancelIntercept func(), err *ably.ProtoErrorInfo, channelState ably.ChannelState) chanTransitionFunc {
	return func() (chanNextStates, func()) {
		var attachMsg *ably.ProtocolMessage
		ablytest.Soon.Recv(c.t, &attachMsg, msg, c.t.Fatalf)

		var next chanNextStates
		if err != nil {
			// Ideally, for moving to FAILED, we'd arrange a real capabilities error
			// to keep things real. But it's too much hassle for now.
			attachMsg.Error = err
			if channelState == chFailed {
				attachMsg.Action = ably.ActionError
			}
			if channelState == chDetached {
				attachMsg.Action = ably.ActionDetached
			}
		} else {
			next = chanNextStates{
				chDetaching: c.detach,
			}
		}

		change := make(ably.ChannelStateChanges, 1)
		c.Channel.OnceAll(change.Receive)

		cancelIntercept()

		ablytest.Instantly.Recv(c.t, nil, change, c.t.Fatalf)

		return next, nil
	}
}

func (c ChanTransitioner) detach() (chanNextStates, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), ablytest.Timeout)
	msg := c.intercept(ctx, ably.ActionDetached)

	change := make(ably.ChannelStateChanges, 1)
	c.Channel.Once(ably.ChannelEventDetaching, change.Receive)

	async(func() error {
		err := c.Channel.Detach(context.Background())
		c.err[chDetaching] <- err
		return err
	})

	// Detach sets a timeout; discard it.
	ablytest.Soon.Recv(c.t, nil, c.afterCalls, c.t.Fatalf)

	ablytest.Instantly.Recv(c.t, nil, change, c.t.Fatalf)

	return chanNextStates{
		chDetached: c.finishDetach(msg, cancel, nil),
		chFailed:   c.finishDetach(msg, cancel, &ably.ProtoErrorInfo{Message: "fake error", Code: 50001}),
	}, cancel
}

func (c ChanTransitioner) finishDetach(msg <-chan *ably.ProtocolMessage, cancelIntercept func(), err *ably.ProtoErrorInfo) chanTransitionFunc {
	return func() (chanNextStates, func()) {

		var detachMsg *ably.ProtocolMessage
		ablytest.Soon.Recv(c.t, &detachMsg, msg, c.t.Fatalf)

		if err != nil {
			// Ideally, for moving to FAILED, we'd arrange a real capabilities error
			// to keep things real. But it's too much hassle for now.
			detachMsg.Action = ably.ActionError
			detachMsg.Error = err
		}

		change := make(ably.ChannelStateChanges, 1)
		c.Channel.OnceAll(change.Receive)

		cancelIntercept()

		ablytest.Instantly.Recv(c.t, nil, change, c.t.Fatalf)

		return nil, nil
	}
}

type chanTransitionFunc func() (next chanNextStates, cleanUp func())

type chanNextStates map[ably.ChannelState]chanTransitionFunc

// This file is full of those. Avoid "ably.ConnectionState" noise.
var (
	initialized  = ably.ConnectionStateInitialized
	connecting   = ably.ConnectionStateConnecting
	connected    = ably.ConnectionStateConnected
	disconnected = ably.ConnectionStateDisconnected
	suspended    = ably.ConnectionStateSuspended
	closing      = ably.ConnectionStateClosing
	closed       = ably.ConnectionStateClosed
	failed       = ably.ConnectionStateFailed

	chInitialized = ably.ChannelStateInitialized
	chAttaching   = ably.ChannelStateAttaching
	chAttached    = ably.ChannelStateAttached
	chDetaching   = ably.ChannelStateDetaching
	chDetached    = ably.ChannelStateDetached
	chSuspended   = ably.ChannelStateSuspended
	chFailed      = ably.ChannelStateFailed
)

func asyncAttach(c *ably.RealtimeChannel) <-chan error {
	return async(func() error { return c.Attach(context.Background()) })
}

func asyncDetach(c *ably.RealtimeChannel) <-chan error {
	return async(func() error { return c.Detach(context.Background()) })
}

func asyncPublish(c *ably.RealtimeChannel) <-chan error {
	return async(func() error { return c.Publish(context.Background(), "event", nil) })
}

func async(f func() error) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- f()
	}()
	return done
}
