//go:build !integration
// +build !integration

package ably_test

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/internal/ablytest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const closeTestRetryTimeout = 50 * time.Millisecond

// closeTestOpTimeout bounds publishes and attaches, so that one left
// unresolved fails the test quickly.
const closeTestOpTimeout = 2 * time.Second

// failingDialer fails every dial. The dial numbered blockOn waits until
// release is sent the error to fail with; every other dial fails immediately
// with a recoverable error.
type failingDialer struct {
	blockOn int32
	dials   int32
	started chan int32
	release chan error
}

func newFailingDialer(blockOn int32) *failingDialer {
	return &failingDialer{
		blockOn: blockOn,
		started: make(chan int32, 100),
		release: make(chan error),
	}
}

func (d *failingDialer) dial(string, *url.URL, time.Duration) (ably.Conn, error) {
	n := atomic.AddInt32(&d.dials, 1)
	d.started <- n
	if n == d.blockOn {
		return nil, <-d.release
	}
	return nil, errors.New("dial failed")
}

func (d *failingDialer) count() int32 {
	return atomic.LoadInt32(&d.dials)
}

// newClientWithDialer creates a client that dials with dial, applying any
// extra options. It authenticates with a fake key unless extra options are
// given.
func newClientWithDialer(t *testing.T, dial func(string, *url.URL, time.Duration) (ably.Conn, error), extra ...ably.ClientOption) (*ably.Realtime, ably.ConnStateChanges) {
	t.Helper()
	if len(extra) == 0 {
		extra = []ably.ClientOption{ably.WithKey("fake.key:secret")}
	}
	client, err := ably.NewRealtime(append([]ably.ClientOption{
		ably.WithAutoConnect(false),
		ably.WithDial(dial),
		ably.WithRealtimeRequestTimeout(closeTestRetryTimeout),
		ably.WithDisconnectedRetryTimeout(closeTestRetryTimeout),
		ably.WithSuspendedRetryTimeout(closeTestRetryTimeout),
	}, extra...)...)
	require.NoError(t, err)
	t.Cleanup(client.Close)

	changes := make(ably.ConnStateChanges, 100)
	off := client.Connection.OnAll(changes.Receive)
	t.Cleanup(off)
	return client, changes
}

// retryLoopGoroutines counts goroutines currently inside the connection retry
// loop.
func retryLoopGoroutines() int {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return bytes.Count(buf[:n], []byte("connectWithRetryLoop"))
		}
		buf = make([]byte, 2*len(buf))
	}
}

func expectState(t *testing.T, changes ably.ConnStateChanges, want ably.ConnectionState) {
	t.Helper()
	var change ably.ConnectionStateChange
	ablytest.Soon.Recv(t, &change, changes, t.Fatalf)
	require.Equal(t, want, change.Current, "unexpected state change: %+v", change)
}

// assertStays checks that, once in the terminal state want, the client makes
// no further connection attempts or state changes over several retry
// intervals, and that the retry loop goroutine has exited.
func assertStays(t *testing.T, want ably.ConnectionState, client *ably.Realtime, changes ably.ConnStateChanges, dialCount func() int32, baseline int) {
	t.Helper()
	dials := dialCount()
	ablytest.Before(5*closeTestRetryTimeout).NoRecv(t, nil, changes, t.Errorf)
	assert.Equal(t, dials, dialCount(), "expected no dials after close")
	assert.Equal(t, want, client.Connection.State())
	assert.True(t, ablytest.Soon.IsTrue(func() bool {
		return retryLoopGoroutines() <= baseline
	}), "expected retry loop goroutine to exit after close")
}

func TestRealtimeConn_CloseDuringFailingConnectAttempt(t *testing.T) {
	t.Run("RTN12f: close during first attempt that fails recoverably ends CLOSED", func(t *testing.T) {
		baseline := retryLoopGoroutines()
		d := newFailingDialer(1)
		client, changes := newClientWithDialer(t, d.dial)

		client.Connect()
		expectState(t, changes, ably.ConnectionStateConnecting)
		ablytest.Soon.Recv(t, nil, d.started, t.Fatalf)

		client.Close()
		expectState(t, changes, ably.ConnectionStateClosing)

		d.release <- errors.New("dial timeout")
		expectState(t, changes, ably.ConnectionStateClosed)

		assertStays(t, ably.ConnectionStateClosed, client, changes, d.count, baseline)
	})

	t.Run("RTN12f: close during retry attempt that fails recoverably ends CLOSED", func(t *testing.T) {
		baseline := retryLoopGoroutines()
		d := newFailingDialer(2)
		client, changes := newClientWithDialer(t, d.dial)

		client.Connect()
		expectState(t, changes, ably.ConnectionStateConnecting)
		expectState(t, changes, ably.ConnectionStateDisconnected)

		// Retry after the disconnected retry timeout.
		expectState(t, changes, ably.ConnectionStateConnecting)
		var n int32
		ablytest.Soon.Recv(t, &n, d.started, t.Fatalf) // first, failed attempt
		ablytest.Soon.Recv(t, &n, d.started, t.Fatalf) // blocked retry
		require.Equal(t, int32(2), n)

		client.Close()
		expectState(t, changes, ably.ConnectionStateClosing)

		d.release <- errors.New("dial timeout")
		expectState(t, changes, ably.ConnectionStateClosed)

		assertStays(t, ably.ConnectionStateClosed, client, changes, d.count, baseline)
	})

	t.Run("RTN12f: close during attempt that fails non-recoverably ends CLOSED, not FAILED", func(t *testing.T) {
		baseline := retryLoopGoroutines()
		d := newFailingDialer(1)
		client, changes := newClientWithDialer(t, d.dial)

		client.Connect()
		expectState(t, changes, ably.ConnectionStateConnecting)
		ablytest.Soon.Recv(t, nil, d.started, t.Fatalf)

		client.Close()
		expectState(t, changes, ably.ConnectionStateClosing)

		d.release <- &ably.ErrorInfo{StatusCode: 401, Code: ably.ErrUnauthorized}
		expectState(t, changes, ably.ConnectionStateClosed)

		assertStays(t, ably.ConnectionStateClosed, client, changes, d.count, baseline)
	})

	t.Run("RTN12d: close while disconnected ends CLOSED", func(t *testing.T) {
		baseline := retryLoopGoroutines()
		d := newFailingDialer(0)
		client, changes := newClientWithDialer(t, d.dial)

		client.Connect()
		expectState(t, changes, ably.ConnectionStateConnecting)
		expectState(t, changes, ably.ConnectionStateDisconnected)

		client.Close()
		expectState(t, changes, ably.ConnectionStateClosed)

		assertStays(t, ably.ConnectionStateClosed, client, changes, d.count, baseline)
	})
}

// pipeDialer connects to a fake server through MessagePipe, counting dials
// and transport closes.
type pipeDialer struct {
	in     chan *ably.ProtocolMessage
	out    chan *ably.ProtocolMessage
	dials  int32
	closes int32
}

func newPipeDialer() *pipeDialer {
	return &pipeDialer{
		in:  make(chan *ably.ProtocolMessage, 10),
		out: make(chan *ably.ProtocolMessage, 10),
	}
}

func (d *pipeDialer) dial(proto string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
	atomic.AddInt32(&d.dials, 1)
	conn, err := MessagePipe(d.in, d.out)(proto, u, timeout)
	return closeCountingConn{Conn: conn, closes: &d.closes}, err
}

func (d *pipeDialer) count() int32 {
	return atomic.LoadInt32(&d.dials)
}

func (d *pipeDialer) closed() bool {
	return atomic.LoadInt32(&d.closes) > 0
}

type closeCountingConn struct {
	ably.Conn
	closes *int32
}

func (c closeCountingConn) Close() error {
	atomic.AddInt32(c.closes, 1)
	return c.Conn.Close()
}

var tokenErr = &ably.ProtoErrorInfo{StatusCode: 401, Code: 40142}

func TestRealtimeConn_ServerDisconnectedWhileClosing(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  *ably.ProtoErrorInfo
		auth []ably.ClientOption
		want ably.ConnectionState
	}{
		// The server closes the transport after DISCONNECTED (RTN15h), so
		// once CLOSE has been sent the connection ends CLOSED (RTN12c)
		// rather than reconnecting.
		{name: "RTN12c, RTN15h3: non-token error ends CLOSED", want: ably.ConnectionStateClosed},
		{name: "RTN12c, RTN15h2: renewable token error ends CLOSED", err: tokenErr, want: ably.ConnectionStateClosed},
		// A token that can't be renewed fails the connection, which won't
		// retry either.
		{name: "RTN15h1: non-renewable token error ends FAILED", err: tokenErr, auth: []ably.ClientOption{ably.WithToken("fake:token")}, want: ably.ConnectionStateFailed},
	} {
		t.Run("DISCONNECTED received while CLOSING: "+tc.name, func(t *testing.T) {
			baseline := retryLoopGoroutines()
			client, changes, d := connectedPipeClient(t, tc.auth...)

			client.Close()
			expectState(t, changes, ably.ConnectionStateClosing)
			expectSent(t, d, ably.ActionClose)

			// The server disconnects the transport before acknowledging the
			// CLOSE.
			d.in <- &ably.ProtocolMessage{Action: ably.ActionDisconnected, Error: tc.err}
			expectState(t, changes, tc.want)
			assert.True(t, d.closed(), "expected transport to be closed")

			assertStays(t, tc.want, client, changes, d.count, baseline)
		})
	}
}

func TestRealtimeConn_ServerErrorWhileClosing(t *testing.T) {
	var renewals int32
	renewOnce := func(context.Context, ably.TokenParams) (ably.Tokener, error) {
		if atomic.AddInt32(&renewals, 1) > 1 {
			return nil, errors.New("token renewal failed")
		}
		return ably.TokenString("fake:token"), nil
	}
	renewAlways := func(context.Context, ably.TokenParams) (ably.Tokener, error) {
		return ably.TokenString("fake:token"), nil
	}

	for _, tc := range []struct {
		name string
		err  *ably.ProtoErrorInfo
		auth []ably.ClientOption
		want ably.ConnectionState
	}{
		// A non-token ERROR is fatal to the connection, whatever its state.
		{name: "RTN14g: non-token error ends FAILED", err: &ably.ProtoErrorInfo{StatusCode: 500, Code: 50000}, want: ably.ConnectionStateFailed},
		// A token ERROR would renew the token and reconnect, but the attempt
		// didn't succeed after Close was called, so it ends CLOSED.
		{name: "RTN12f, RTN14b: token error with successful renewal ends CLOSED", err: tokenErr, auth: []ably.ClientOption{ably.WithAuthCallback(renewAlways)}, want: ably.ConnectionStateClosed},
		{name: "RTN12f, RTN14b: token error with failed renewal ends CLOSED", err: tokenErr, auth: []ably.ClientOption{ably.WithAuthCallback(renewOnce)}, want: ably.ConnectionStateClosed},
	} {
		t.Run("ERROR received while CLOSING: "+tc.name, func(t *testing.T) {
			atomic.StoreInt32(&renewals, 0)
			baseline := retryLoopGoroutines()
			d := newPipeDialer()
			client, changes := newClientWithDialer(t, d.dial, tc.auth...)

			client.Connect()
			expectState(t, changes, ably.ConnectionStateConnecting)
			require.True(t, ablytest.Soon.IsTrue(func() bool { return d.count() == 1 }))

			client.Close()
			expectState(t, changes, ably.ConnectionStateClosing)

			d.in <- &ably.ProtocolMessage{Action: ably.ActionError, Error: tc.err}
			expectState(t, changes, tc.want)
			assert.True(t, d.closed(), "expected transport to be closed")

			assertStays(t, tc.want, client, changes, d.count, baseline)
		})
	}
}

// publishAsync publishes on channel with opAsync.
func publishAsync(channel *ably.RealtimeChannel) <-chan error {
	return opAsync(func(ctx context.Context) error {
		return channel.Publish(ctx, "name", "data")
	})
}

// expectPublishFailed asserts that a publish failed because the connection
// closed, rather than timing out.
func expectPublishFailed(t *testing.T, result <-chan error) {
	t.Helper()
	var err error
	ablytest.Soon.Recv(t, &err, result, t.Fatalf)
	require.Error(t, err)
	assert.NotErrorIs(t, err, context.DeadlineExceeded)
}

func TestRealtimeConn_RTN7e_MessagesFailOnClosed(t *testing.T) {
	t.Run("queued message fails when Close ends a failed connection attempt", func(t *testing.T) {
		d := newFailingDialer(1)
		client, changes := newClientWithDialer(t, d.dial)

		client.Connect()
		expectState(t, changes, ably.ConnectionStateConnecting)
		ablytest.Soon.Recv(t, nil, d.started, t.Fatalf)

		result := publishAsync(client.Channels.Get("test"))
		ablytest.Before(closeTestRetryTimeout).NoRecv(t, nil, result, t.Fatalf) // queued

		client.Close()
		expectState(t, changes, ably.ConnectionStateClosing)
		d.release <- errors.New("dial timeout")
		expectState(t, changes, ably.ConnectionStateClosed)

		expectPublishFailed(t, result)
	})

	for _, tc := range []struct {
		name  string
		reply *ably.ProtocolMessage
	}{
		{name: "server CLOSED", reply: &ably.ProtocolMessage{Action: ably.ActionClosed}},
		{name: "server DISCONNECTED", reply: &ably.ProtocolMessage{Action: ably.ActionDisconnected}},
	} {
		t.Run("message awaiting ACK fails on CLOSED after "+tc.name, func(t *testing.T) {
			client, changes, d := connectedPipeClient(t)

			result := publishAsync(client.Channels.Get("test"))
			expectSent(t, d, ably.ActionMessage)

			client.Close()
			expectState(t, changes, ably.ConnectionStateClosing)
			expectSent(t, d, ably.ActionClose)

			d.in <- tc.reply
			expectState(t, changes, ably.ConnectionStateClosed)

			expectPublishFailed(t, result)
		})
	}

	t.Run("message awaiting ACK fails on FAILED after server ERROR", func(t *testing.T) {
		client, changes, d := connectedPipeClient(t)

		result := publishAsync(client.Channels.Get("test"))
		expectSent(t, d, ably.ActionMessage)

		d.in <- &ably.ProtocolMessage{Action: ably.ActionError, Error: &ably.ProtoErrorInfo{StatusCode: 500, Code: 50000}}
		expectState(t, changes, ably.ConnectionStateFailed)

		expectPublishFailed(t, result)
	})
}

// connectedPipeClient returns a client connected to a fake server.
func connectedPipeClient(t *testing.T, extra ...ably.ClientOption) (*ably.Realtime, ably.ConnStateChanges, *pipeDialer) {
	t.Helper()
	d := newPipeDialer()
	client, changes := newClientWithDialer(t, d.dial, extra...)
	d.in <- &ably.ProtocolMessage{
		Action:            ably.ActionConnected,
		ConnectionID:      "connection-id",
		ConnectionDetails: &ably.ConnectionDetails{},
	}
	client.Connect()
	expectState(t, changes, ably.ConnectionStateConnecting)
	expectState(t, changes, ably.ConnectionStateConnected)
	return client, changes, d
}

// opAsync runs op with a bounded context and returns a channel that receives
// its result.
func opAsync(op func(context.Context) error) <-chan error {
	result := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), closeTestOpTimeout)
		defer cancel()
		result <- op(ctx)
	}()
	return result
}

func expectChannelState(t *testing.T, changes ably.ChannelStateChanges, want ably.ChannelState) {
	t.Helper()
	var change ably.ChannelStateChange
	ablytest.Soon.Recv(t, &change, changes, t.Fatalf)
	require.Equal(t, want, change.Current, "unexpected channel state change: %+v", change)
}

func expectSent(t *testing.T, d *pipeDialer, action ably.ProtoAction) {
	t.Helper()
	var msg *ably.ProtocolMessage
	ablytest.Soon.Recv(t, &msg, d.out, t.Fatalf)
	require.Equal(t, action, msg.Action)
}

func TestRealtimeConn_RTL3b_ChannelsDetachOnClosed(t *testing.T) {
	for _, from := range []ably.ChannelState{
		ably.ChannelStateAttaching,
		ably.ChannelStateAttached,
		ably.ChannelStateDetaching,
	} {
		t.Run(from.String()+" channel becomes DETACHED when the connection closes", func(t *testing.T) {
			// Keep the attach from timing out (RTL4f) before Close is called.
			client, changes, d := connectedPipeClient(t,
				ably.WithKey("fake.key:secret"),
				ably.WithRealtimeRequestTimeout(closeTestOpTimeout))

			channel := client.Channels.Get("test")
			channelChanges := make(ably.ChannelStateChanges, 10)
			off := channel.OnAll(channelChanges.Receive)
			defer off()

			pending := opAsync(channel.Attach)
			expectChannelState(t, channelChanges, ably.ChannelStateAttaching)
			expectSent(t, d, ably.ActionAttach)

			if from != ably.ChannelStateAttaching {
				d.in <- &ably.ProtocolMessage{Action: ably.ActionAttached, Channel: channel.Name}
				expectChannelState(t, channelChanges, ably.ChannelStateAttached)
				var err error
				ablytest.Soon.Recv(t, &err, pending, t.Fatalf)
				require.NoError(t, err)
				pending = nil
			}
			if from == ably.ChannelStateDetaching {
				pending = opAsync(channel.Detach)
				expectChannelState(t, channelChanges, ably.ChannelStateDetaching)
				expectSent(t, d, ably.ActionDetach)
			}

			client.Close()
			expectState(t, changes, ably.ConnectionStateClosing)
			d.in <- &ably.ProtocolMessage{Action: ably.ActionClosed}
			expectState(t, changes, ably.ConnectionStateClosed)

			expectChannelState(t, channelChanges, ably.ChannelStateDetached)
			if pending != nil {
				// The pending attach or detach completes rather than waiting
				// for its context to expire.
				var err error
				ablytest.Soon.Recv(t, &err, pending, t.Fatalf)
				assert.NotErrorIs(t, err, context.DeadlineExceeded)
			}
		})
	}
}

// RTN12c: if Close is called while a token renewal's reconnection attempt is
// in flight, the transport that received the token error is closed, since no
// new transport will replace it.
func TestRealtimeConn_CloseDuringTokenRenewalClosesOldTransport(t *testing.T) {
	baseline := retryLoopGoroutines()
	pipe := newPipeDialer()
	retry := newFailingDialer(1)
	var dials int32
	client, changes := newClientWithDialer(t, func(proto string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
		if atomic.AddInt32(&dials, 1) == 1 {
			return pipe.dial(proto, u, timeout)
		}
		return retry.dial(proto, u, timeout)
	}, ably.WithAuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
		return ably.TokenString("fake:token"), nil
	}))

	client.Connect()
	expectState(t, changes, ably.ConnectionStateConnecting)
	require.True(t, ablytest.Soon.IsTrue(func() bool { return pipe.count() == 1 }))

	// The server rejects the token, so the client renews it and redials (RTN14b).
	pipe.in <- &ably.ProtocolMessage{Action: ably.ActionError, Error: tokenErr}
	ablytest.Soon.Recv(t, nil, retry.started, t.Fatalf)

	client.Close()
	expectState(t, changes, ably.ConnectionStateClosing)
	retry.release <- errors.New("dial timeout")
	expectState(t, changes, ably.ConnectionStateClosed)
	assert.True(t, ablytest.Soon.IsTrue(pipe.closed), "expected the old transport to be closed")

	assertStays(t, ably.ConnectionStateClosed, client, changes, func() int32 { return atomic.LoadInt32(&dials) }, baseline)
}

// tokenDisconnected is a DISCONNECTED message with a token error, which makes
// a client that can renew its token do so (RTN15h2).
var tokenDisconnected = &ably.ProtocolMessage{Action: ably.ActionDisconnected, Error: tokenErr}

func TestRealtimeConn_CloseDuringTokenRenewal(t *testing.T) {
	for _, tc := range []struct {
		name     string
		renewErr error
	}{
		{name: "renewal succeeds"},
		{name: "renewal fails", renewErr: errors.New("token renewal failed")},
	} {
		t.Run("RTN12: Close isn't blocked by a token renewal and ends CLOSED when "+tc.name, func(t *testing.T) {
			baseline := retryLoopGoroutines()
			renewing := make(chan struct{}, 1)
			release := make(chan struct{})
			var renewals int32
			client, changes, d := connectedPipeClient(t, ably.WithAuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
				if atomic.AddInt32(&renewals, 1) > 1 {
					renewing <- struct{}{}
					<-release
					if tc.renewErr != nil {
						return nil, tc.renewErr
					}
				}
				return ably.TokenString("fake:token"), nil
			}))

			d.in <- tokenDisconnected
			expectState(t, changes, ably.ConnectionStateDisconnected)
			ablytest.Soon.Recv(t, nil, renewing, t.Fatalf)

			closed := make(chan struct{})
			go func() {
				client.Close()
				close(closed)
			}()
			ablytest.Soon.Recv(t, nil, closed, t.Fatalf)
			expectState(t, changes, ably.ConnectionStateClosed)

			close(release)
			assert.True(t, ablytest.Soon.IsTrue(d.closed), "expected the old transport to be closed")
			assertStays(t, ably.ConnectionStateClosed, client, changes, d.count, baseline)
		})
	}
}

// RTN15h2, RTN14d: if renewing the token fails, the connection is DISCONNECTED
// and retries like from any other DISCONNECTED state.
func TestRealtimeConn_RetriesAfterTokenRenewalFails(t *testing.T) {
	var renewals int32
	_, changes, d := connectedPipeClient(t, ably.WithAuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
		if atomic.AddInt32(&renewals, 1) == 2 {
			return nil, errors.New("token renewal failed")
		}
		return ably.TokenString("fake:token"), nil
	}))

	d.in <- tokenDisconnected
	expectState(t, changes, ably.ConnectionStateDisconnected)
	var change ably.ConnectionStateChange
	ablytest.Soon.Recv(t, &change, changes, t.Fatalf)
	require.Equal(t, ably.ConnectionStateDisconnected, change.Current, "expected renewal failure: %+v", change)

	d.in <- &ably.ProtocolMessage{
		Action:            ably.ActionConnected,
		ConnectionID:      "connection-id",
		ConnectionDetails: &ably.ConnectionDetails{},
	}
	expectState(t, changes, ably.ConnectionStateConnecting)
	expectState(t, changes, ably.ConnectionStateConnected)
	assert.Equal(t, int32(2), d.count())
}

func TestRealtimeConn_RTN7e_MessagesFailOnSuspended(t *testing.T) {
	const stateTTL = 4 * closeTestRetryTimeout

	t.Run("queued message fails when the connection is SUSPENDED", func(t *testing.T) {
		d := newFailingDialer(0)
		client, changes := newClientWithDialer(t, d.dial,
			ably.WithKey("fake.key:secret"),
			ably.WithConnectionStateTTL(stateTTL))

		client.Connect()
		expectState(t, changes, ably.ConnectionStateConnecting)
		expectState(t, changes, ably.ConnectionStateDisconnected)
		result := publishAsync(client.Channels.Get("test"))
		ablytest.Before(closeTestRetryTimeout/2).NoRecv(t, nil, result, t.Fatalf) // queued

		require.True(t, ablytest.Soon.IsTrue(func() bool {
			return client.Connection.State() == ably.ConnectionStateSuspended
		}))
		expectPublishFailed(t, result)
	})

	t.Run("message awaiting ACK fails when the connection is SUSPENDED", func(t *testing.T) {
		pipe := newPipeDialer()
		var dials int32
		client, changes := newClientWithDialer(t, func(proto string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			if atomic.AddInt32(&dials, 1) == 1 {
				return pipe.dial(proto, u, timeout)
			}
			return nil, errors.New("dial failed")
		}, ably.WithKey("fake.key:secret"), ably.WithConnectionStateTTL(stateTTL))
		pipe.in <- &ably.ProtocolMessage{
			Action:            ably.ActionConnected,
			ConnectionID:      "connection-id",
			ConnectionDetails: &ably.ConnectionDetails{},
		}
		client.Connect()
		expectState(t, changes, ably.ConnectionStateConnecting)
		expectState(t, changes, ably.ConnectionStateConnected)

		result := publishAsync(client.Channels.Get("test"))
		expectSent(t, pipe, ably.ActionMessage)

		// Kept for resending while DISCONNECTED (RTN19a), but not once SUSPENDED.
		pipe.in <- &ably.ProtocolMessage{Action: ably.ActionDisconnected}
		expectState(t, changes, ably.ConnectionStateDisconnected)
		ablytest.Before(closeTestRetryTimeout/2).NoRecv(t, nil, result, t.Fatalf)

		require.True(t, ablytest.Soon.IsTrue(func() bool {
			return client.Connection.State() == ably.ConnectionStateSuspended
		}))
		expectPublishFailed(t, result)
	})
}
