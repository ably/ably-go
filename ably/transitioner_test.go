package ably_test

import (
	"context"
	"errors"
	"io"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"
)

type ConnTransitioner struct {
	Realtime *ably.Realtime

	t              *testing.T
	dialErr        chan<- error
	fakeDisconnect func() error
	intercept      func(context.Context, ...proto.Action) <-chan *proto.ProtocolMessage
	afterCalls     <-chan ablytest.AfterCall

	next connNextStates
}

// TransitionConn makes a connection with external events (requests, protocol
// messages, timers) mocked, so that state transitions can be controlled by the
// tests. The returned ConnTransitioner's To method moves the connection to the
// desired state, and keeps it there.
func TransitionConn(t *testing.T, dial ablytest.DialFunc, options ...ably.ClientOption) (ConnTransitioner, io.Closer) {
	t.Helper()

	c := ConnTransitioner{t: t}

	dial, disconnect := ablytest.DialFakeDisconnect(dial)
	c.fakeDisconnect = disconnect

	dial, intercept := ablytest.DialIntercept(dial)
	c.intercept = intercept

	dialErr := make(chan error, 1)
	prevDial := dial
	dial = func(proto string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
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
		closed:     c.closeNow,
	}

	return c, ablytest.FullRealtimeCloser(c.Realtime)
}

func (c ConnTransitioner) Channel(name string) ChanTransitioner {
	transitioner := ChanTransitioner{c, c.Realtime.Channels.Get(name), make(chan error, 1), nil}
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

			var timers []ablytest.AfterCall
			for i := 0; i < 2; i++ {
				var timer ablytest.AfterCall
				ablytest.Instantly.Recv(c.t, &timer, c.afterCalls, c.t.Fatalf)
				timers = append(timers, timer)
			}
			// Shortest timer is for reconnection.
			sort.Slice(timers, func(i, j int) bool {
				return timers[i].D < timers[j].D
			})
			reconnect, suspend := timers[0], timers[1]

			return connNextStates{
					connecting: c.recover(reconnect),
					suspended:  c.fireSuspend(suspend),
				}, func() {
					c.Realtime.Close() // cancel timers; don't reconnect
				}
		}

		// Should be CONNECTED.
		return connNextStates{
			disconnected: c.disconnect,
		}, nil
	}
}

func (c ConnTransitioner) fail() (connNextStates, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), ablytest.Timeout)
	defer cancel()

	intercepted := c.intercept(ctx, proto.ActionConnected)

	c.dialErr <- nil

	msg := <-intercepted
	msg.Action = proto.ActionError
	msg.Error = &proto.ErrorInfo{
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

func (c ConnTransitioner) fireSuspend(timer ablytest.AfterCall) connTransitionFunc {
	return func() (connNextStates, func()) {
		timer.Fire()
		return c.suspend()
	}
}

func (c ConnTransitioner) suspend() (connNextStates, func()) {
	change := make(ably.ConnStateChanges, 1)
	c.Realtime.Connection.Once(ably.ConnectionEventSuspended, change.Receive)

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

func (c ConnTransitioner) closeNow() (connNextStates, func()) {
	c.Realtime.Close()
	return nil, nil
}

type connTransitionFunc func() (next connNextStates, cleanUp func())

type connNextStates map[ably.ConnectionState]connTransitionFunc

type ChanTransitioner struct {
	ConnTransitioner
	Channel *ably.RealtimeChannel
	err     chan error
	next    chanNextStates
}

func (c ChanTransitioner) To(path ...ably.ChannelState) (*ably.RealtimeChannel, io.Closer) {
	c.t.Helper()

	from := chInitialized
	c.assertState(from)

	next := chanNextStates{
		chAttaching: c.attach,
	}
	var cleanUp func()

	for _, to := range path {
		if from == to {
			continue
		}

		transition, ok := next[to]
		if !ok {
			c.t.Fatalf("no transition from %v to %v", from, to)
		}
		next, cleanUp = transition()
		from = to
		c.assertState(from)
	}

	return c.Channel, closeFunc(func() error {
		if cleanUp != nil {
			cleanUp()
		}
		return nil
	})
}

// preserves state context for channel
func (c *ChanTransitioner) ToSpecifiedState(path ...ably.ChannelState) (*ably.RealtimeChannel, io.Closer) {
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
	msg := c.intercept(ctx, proto.ActionAttached)

	change := make(ably.ChannelStateChanges, 1)
	c.Channel.Once(ably.ChannelEventAttaching, change.Receive)

	asyncAttach(c.Channel)

	ablytest.Instantly.Recv(c.t, nil, change, c.t.Fatalf)

	return chanNextStates{
		chAttached: c.finishAttach(msg, cancel, nil),
		chFailed:   c.finishAttach(msg, cancel, &proto.ErrorInfo{Message: "fake error"}),
	}, cancel
}

func (c ChanTransitioner) finishAttach(msg <-chan *proto.ProtocolMessage, cancelIntercept func(), err *proto.ErrorInfo) chanTransitionFunc {
	return func() (chanNextStates, func()) {
		var attachMsg *proto.ProtocolMessage
		ablytest.Soon.Recv(c.t, &attachMsg, msg, c.t.Fatalf)

		var next chanNextStates
		if err != nil {
			// Ideally, for moving to FAILED, we'd arrange a real capabilities error
			// to keep things real. But it's too much hassle for now.
			attachMsg.Action = proto.ActionError
			attachMsg.Error = err
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
	msg := c.intercept(ctx, proto.ActionDetached)

	change := make(ably.ChannelStateChanges, 1)
	c.Channel.Once(ably.ChannelEventDetaching, change.Receive)

	async(func() error {
		errCh := asyncDetach(c.Channel)
		err := <-errCh
		c.err <- err
		return err
	})

	// Detach sets a timeout; discard it.
	ablytest.Instantly.Recv(c.t, nil, c.afterCalls, c.t.Fatalf)

	ablytest.Instantly.Recv(c.t, nil, change, c.t.Fatalf)

	return chanNextStates{
		chDetached: c.finishDetach(msg, cancel, nil),
		chFailed:   c.finishDetach(msg, cancel, &proto.ErrorInfo{Message: "fake error", Code: 50001}),
	}, cancel
}

func (c ChanTransitioner) finishDetach(msg <-chan *proto.ProtocolMessage, cancelIntercept func(), err *proto.ErrorInfo) chanTransitionFunc {
	return func() (chanNextStates, func()) {

		var attachMsg *proto.ProtocolMessage
		ablytest.Soon.Recv(c.t, &attachMsg, msg, c.t.Fatalf)

		if err != nil {
			// Ideally, for moving to FAILED, we'd arrange a real capabilities error
			// to keep things real. But it's too much hassle for now.
			attachMsg.Action = proto.ActionError
			attachMsg.Error = err
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
