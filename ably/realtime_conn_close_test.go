//go:build !integration
// +build !integration

package ably_test

import (
	"bytes"
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

func newClientWithDialer(t *testing.T, dial func(string, *url.URL, time.Duration) (ably.Conn, error)) (*ably.Realtime, ably.ConnStateChanges) {
	t.Helper()
	client, err := ably.NewRealtime(
		ably.WithKey("fake.key:secret"),
		ably.WithAutoConnect(false),
		ably.WithDial(dial),
		ably.WithRealtimeRequestTimeout(closeTestRetryTimeout),
		ably.WithDisconnectedRetryTimeout(closeTestRetryTimeout),
		ably.WithSuspendedRetryTimeout(closeTestRetryTimeout),
	)
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

// assertStaysClosed checks that, once closed, the client makes no further
// connection attempts or state changes over several retry intervals, and
// that the retry loop goroutine has exited.
func assertStaysClosed(t *testing.T, client *ably.Realtime, changes ably.ConnStateChanges, dialCount func() int32, baseline int) {
	t.Helper()
	dials := dialCount()
	ablytest.Before(5*closeTestRetryTimeout).NoRecv(t, nil, changes, t.Errorf)
	assert.Equal(t, dials, dialCount(), "expected no dials after close")
	assert.Equal(t, ably.ConnectionStateClosed, client.Connection.State())
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

		assertStaysClosed(t, client, changes, d.count, baseline)
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

		assertStaysClosed(t, client, changes, d.count, baseline)
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

		assertStaysClosed(t, client, changes, d.count, baseline)
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

		assertStaysClosed(t, client, changes, d.count, baseline)
	})
}

func TestRealtimeConn_ServerDisconnectedWhileClosing(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  *ably.ProtoErrorInfo
	}{
		{name: "RTN15h3: non-token error"},
		{name: "RTN15h2: renewable token error", err: &ably.ProtoErrorInfo{StatusCode: 401, Code: 40142}},
	} {
		t.Run("RTN12f: DISCONNECTED received while CLOSING ends CLOSED: "+tc.name, func(t *testing.T) {
			baseline := retryLoopGoroutines()
			in := make(chan *ably.ProtocolMessage, 10)
			out := make(chan *ably.ProtocolMessage, 10)
			pipe := MessagePipe(in, out)
			var dials int32
			client, changes := newClientWithDialer(t, func(proto string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				atomic.AddInt32(&dials, 1)
				return pipe(proto, u, timeout)
			})

			in <- &ably.ProtocolMessage{
				Action:            ably.ActionConnected,
				ConnectionID:      "connection-id",
				ConnectionDetails: &ably.ConnectionDetails{},
			}
			client.Connect()
			expectState(t, changes, ably.ConnectionStateConnecting)
			expectState(t, changes, ably.ConnectionStateConnected)

			client.Close()
			expectState(t, changes, ably.ConnectionStateClosing)
			var msg *ably.ProtocolMessage
			ablytest.Soon.Recv(t, &msg, out, t.Fatalf)
			require.Equal(t, ably.ActionClose, msg.Action)

			// The server disconnects the transport before acknowledging the
			// CLOSE.
			in <- &ably.ProtocolMessage{Action: ably.ActionDisconnected, Error: tc.err}
			expectState(t, changes, ably.ConnectionStateClosed)

			assertStaysClosed(t, client, changes, func() int32 { return atomic.LoadInt32(&dials) }, baseline)
		})
	}
}
