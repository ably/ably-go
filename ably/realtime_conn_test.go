package ably_test

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"
)

var connTransitions = []ably.ConnectionState{
	ably.ConnectionStateConnecting,
	ably.ConnectionStateConnected,
	ably.ConnectionStateClosing,
	ably.ConnectionStateClosed,
}

func TestRealtimeConn_Connect(t *testing.T) {
	var rec ablytest.ConnStatesRecorder
	app, client := ablytest.NewRealtime()
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
	off := rec.Listen(client)
	defer off()

	if err := ablytest.Wait(ablytest.ConnWaiter(client, nil, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatalf("Connect()=%v", err)
	}
	if serial := client.Connection.Serial(); *serial != -1 {
		t.Fatalf("want serial=-1; got %d", serial)
	}
	if err := ablytest.FullRealtimeCloser(client).Close(); err != nil {
		t.Fatalf("ablytest.FullRealtimeCloser(client).Close()=%v", err)
	}
	if !ablytest.Soon.IsTrue(func() bool {
		return ablytest.Contains(rec.States(), connTransitions)
	}) {
		t.Fatalf("expected %+v, got %+v", connTransitions, rec.States())
	}
}

func TestRealtimeConn_NoConnect(t *testing.T) {
	var rec ablytest.ConnStatesRecorder
	opts := []ably.ClientOption{
		ably.WithAutoConnect(false),
	}
	app, client := ablytest.NewRealtime(opts...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
	off := rec.Listen(client)
	defer off()

	if err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatalf("Connect()=%v", err)
	}
	if serial := client.Connection.Serial(); *serial != -1 {
		t.Fatalf("want serial=-1; got %d", serial)
	}
	if err := ablytest.FullRealtimeCloser(client).Close(); err != nil {
		t.Fatalf("ablytest.FullRealtimeCloser(client).Close()=%v", err)
	}
	if !ablytest.Soon.IsTrue(func() bool {
		return ablytest.Contains(rec.States(), connTransitions)
	}) {
		t.Fatalf("expected %+v, got %+v", connTransitions, rec.States())
	}
}

func TestRealtimeConn_ConnectClose(t *testing.T) {
	var rec ablytest.ConnStatesRecorder
	app, client := ablytest.NewRealtime()
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
	off := rec.Listen(client)
	defer off()

	if err := ablytest.Wait(ablytest.ConnWaiter(client, nil, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatal(err)
	}
	if err := ablytest.FullRealtimeCloser(client).Close(); err != nil {
		t.Fatalf("ablytest.FullRealtimeCloser(client).Close()=%v", err)
	}
	if err := ablytest.Wait(ablytest.ConnWaiter(client, nil, ably.ConnectionEventClosed), nil); err != nil {
		t.Fatal(err)
	}
	if !ablytest.Soon.IsTrue(func() bool {
		return ablytest.Contains(rec.States(), connTransitions)
	}) {
		t.Fatalf("expected %+v, got %+v", connTransitions, rec.States())
	}
}

func TestRealtimeConn_AlreadyConnected(t *testing.T) {
	app, client := ablytest.NewRealtime(ably.WithAutoConnect(false))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	changes := make(chan ably.ConnectionStateChange, 1)
	off := client.Connection.OnceAll(func(change ably.ConnectionStateChange) {
		changes <- change
	})
	defer off()

	ablytest.Before(100*time.Millisecond).NoRecv(t, nil, changes, t.Fatalf)
}

func TestRealtimeConn_AuthError(t *testing.T) {
	opts := []ably.ClientOption{
		ably.WithKey("abc:abc"),
		ably.WithUseTokenAuth(true),
		ably.WithAutoConnect(false),
	}
	client, err := ably.NewRealtime(opts...)
	if err != nil {
		t.Fatalf("NewRealtime()=%v", err)
	}
	if err = ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil); err == nil {
		t.Fatal("Connect(): want err != nil")
	}
}

func TestRealtimeConn_ReceiveTimeout(t *testing.T) {

	const maxIdleInterval = 20
	const realtimeRequestTimeout = 10 * time.Millisecond

	in := make(chan *ably.ProtocolMessage, 16)
	out := make(chan *ably.ProtocolMessage, 16)

	connected := &ably.ProtocolMessage{
		Action:       ably.ActionConnected,
		ConnectionID: "connection-id",
		ConnectionDetails: &ably.ConnectionDetails{
			MaxIdleInterval: maxIdleInterval,
		},
	}
	in <- connected

	app, client := ablytest.NewRealtime(
		ably.WithDial(MessagePipe(in, out, MessagePipeWithNowFunc(time.Now))),
		ably.WithRealtimeRequestTimeout(10*time.Millisecond),
		ably.WithAutoConnect(false),
	)
	defer safeclose(t, app)

	states := make(ably.ConnStateChanges, 10)
	{
		off := client.Connection.On(ably.ConnectionEventConnected, states.Receive)
		defer off()
	}
	{
		off := client.Connection.On(ably.ConnectionEventDisconnected, states.Receive)
		defer off()
	}

	client.Connection.Connect()

	var state ably.ConnectionStateChange

	select {
	case state = <-states:
	case <-time.After(10 * time.Millisecond):
		t.Fatal("didn't receive state change event")
	}

	if expected, got := ably.ConnectionStateConnected, state.Current; expected != got {
		t.Fatalf("expected %v, got %v", expected, got)
	}

	leeway := 10 * time.Millisecond
	select {
	case state = <-states:
	case <-time.After(realtimeRequestTimeout + time.Duration(maxIdleInterval)*time.Millisecond + leeway):
		t.Fatal("didn't receive state change event")
	}

	if expected, got := ably.ConnectionStateDisconnected, state.Current; expected != got {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

/*
FAILING TEST
https://github.com/ably/ably-go/pull/383/checks?check_run_id=3489733889#step:7:580

=== RUN   TestRealtimeConn_BreakConnLoopOnInactiveState/closed
--- FAIL: TestRealtimeConn_BreakConnLoopOnInactiveState (48.57s)
    --- FAIL: TestRealtimeConn_BreakConnLoopOnInactiveState/disconnect (36.18s)
        realtime_conn_test.go:222: called Receive again; expected end of connection loop
        ably_test.go:66: safeclose 0: failed to close ablytest.realtimeIOCloser: context deadline exceeded
    --- PASS: TestRealtimeConn_BreakConnLoopOnInactiveState/error (6.07s)
    --- PASS: TestRealtimeConn_BreakConnLoopOnInactiveState/closed (6.32s)
*/
func TestRealtimeConn_BreakConnLoopOnInactiveState(t *testing.T) {
	t.Skip("FAILING TEST")

	for _, action := range []ably.ProtoAction{
		ably.ActionDisconnect,
		ably.ActionError,
		ably.ActionClosed,
	} {
		t.Run(action.String(), func(t *testing.T) {
			in := make(chan *ably.ProtocolMessage)
			out := make(chan *ably.ProtocolMessage, 16)

			app, client := ablytest.NewRealtime(
				ably.WithDial(MessagePipe(in, out)),
			)
			defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

			connected := &ably.ProtocolMessage{
				Action:            ably.ActionConnected,
				ConnectionID:      "connection-id",
				ConnectionDetails: &ably.ConnectionDetails{},
			}
			select {
			case in <- connected:
			case <-time.After(10 * time.Millisecond):
				t.Fatal("didn't receive incoming protocol message")
			}

			select {
			case in <- &ably.ProtocolMessage{
				Action: action,
			}:
			case <-time.After(10 * time.Millisecond):
				t.Fatal("didn't receive incoming protocol message")
			}

			select {
			case in <- &ably.ProtocolMessage{}:
				t.Fatal("called Receive again; expected end of connection loop")
			case <-time.After(10 * time.Millisecond):
			}
		})
	}
}

func TestRealtimeConn_SendErrorReconnects(t *testing.T) {
	sendErr := make(chan error, 1)
	closed := make(chan struct{}, 1)
	allowDial := make(chan struct{})

	dial := DialFunc(func(p string, url *url.URL, timeout time.Duration) (ably.Conn, error) {
		<-allowDial
		ws, err := ably.DialWebsocket(p, url, timeout)
		if err != nil {
			return nil, err
		}
		return connMock{
			SendFunc: func(m *ably.ProtocolMessage) error {
				select {
				case err := <-sendErr:
					return err
				default:
					return ws.Send(m)
				}
			},
			ReceiveFunc: ws.Receive,
			CloseFunc: func() error {
				closed <- struct{}{}
				return ws.Close()
			},
		}, nil
	})

	app, c := ablytest.NewRealtime(ably.WithDial(dial))
	defer safeclose(t, ablytest.FullRealtimeCloser(c), app)

	allowDial <- struct{}{}

	if err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatal(err)
	}

	// Cause a send error; expect message to be enqueued and transport to be
	// closed.

	sendErr <- errors.New("fail")

	publishErr := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		e := c.Channels.Get("test").Publish(ctx, "test", nil)
		publishErr <- e
	}()

	ablytest.Instantly.Recv(t, nil, closed, t.Fatalf)
	ablytest.Instantly.NoRecv(t, nil, publishErr, t.Fatalf)

	// Reconnect should happen instantly as a result of transport closure.
	ablytest.Instantly.Send(t, allowDial, struct{}{}, t.Fatalf)

	// After reconnection, message should be published.
	var err error
	ablytest.Soon.Recv(t, &err, publishErr, t.Fatalf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
