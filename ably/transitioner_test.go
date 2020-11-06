package ably_test

import (
	"context"
	"errors"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"
)

type ConnTransitioner struct {
	Realtime *ably.Realtime

	t          *testing.T
	dialErr    chan<- error
	disconnect func() error
	intercept  func(context.Context, ...proto.Action) <-chan *proto.ProtocolMessage
}

func TransitionConn(t *testing.T, options ...ably.ClientOption) ConnTransitioner {
	t.Helper()

	c := ConnTransitioner{t: t}

	dial, disconnect := ablytest.DialFakeDisconnect(nil)
	c.disconnect = disconnect

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

	afterCalls := make(chan ablytest.AfterCall, 1)
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

	return c
}

func (c ConnTransitioner) Channel(name string) ChanTransitioner {
	return ChanTransitioner{c, c.Realtime.Channels.Get(name)}
}

func (c ConnTransitioner) To(path ...ably.ConnectionState) (ConnTransitioner, io.Closer) {
	c.t.Helper()

	from := initialized
	c.assertState(from)

	next := connNextStates{
		connecting: c.connect,
		closed:     c.closeNow,
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

	return c, closeFunc(func() error {
		if cleanUp != nil {
			cleanUp()
		}
		return ablytest.FullRealtimeCloser(c.Realtime).Close()
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

	ablytest.Soon.Recv(c.t, nil, change, c.t.Fatalf)

	return connNextStates{
			connected:    c.finishConnecting(nil),
			disconnected: c.finishConnecting(errors.New("fake connection error")),
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
			return connNextStates{
				connecting: c.connect,
			}, nil
		}

		// Should be CONNECTED.
		return connNextStates{}, nil
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

func (c ChanTransitioner) assertState(state ably.ChannelState) {
	c.t.Helper()
	if expected, got := state, c.Channel.State(); expected != got {
		c.t.Fatalf("expected channel to be %v; is %v", expected, got)
	}
}

func (c ChanTransitioner) attach() (chanNextStates, func()) {
	c.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), ablytest.Timeout)
	msg := c.intercept(ctx, proto.ActionAttached)

	change := make(ably.ChannelStateChanges, 1)
	c.Channel.Once(ably.ChannelEventAttaching, change.Receive)

	asyncAttach(c.Channel)

	ablytest.Soon.Recv(c.t, nil, change, c.t.Fatalf)

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
		c.Channel.Once(ably.ChannelEventAttached, change.Receive)

		cancelIntercept()

		ablytest.Instantly.Recv(c.t, nil, change, c.t.Fatalf)

		return next, nil
	}
}

func (c ChanTransitioner) detach() (chanNextStates, func()) {
	c.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), ablytest.Timeout)
	c.intercept(ctx, proto.ActionDetached)

	change := make(ably.ChannelStateChanges, 1)
	c.Channel.Once(ably.ChannelEventDetaching, change.Receive)

	asyncDetach(c.Channel)

	ablytest.Soon.Recv(c.t, nil, change, c.t.Fatalf)

	return chanNextStates{
		chDetached: c.finishDetach(cancel),
	}, cancel
}

func (c ChanTransitioner) finishDetach(cancelIntercept func()) chanTransitionFunc {
	return func() (chanNextStates, func()) {
		change := make(ably.ChannelStateChanges, 1)
		c.Channel.Once(ably.ChannelEventDetached, change.Receive)

		cancelIntercept()

		ablytest.Soon.Recv(c.t, nil, change, c.t.Fatalf)

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
