package ably_test

import (
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"
)

var connTransitions = []ably.ConnectionState{
	ably.ConnectionStateConnecting,
	ably.ConnectionStateConnected,
	ably.ConnectionStateClosing,
	ably.ConnectionStateClosed,
}

func TestRealtimeConn_Connect(t *testing.T) {
	t.Parallel()
	var rec ablytest.ConnStatesRecorder
	app, client := ablytest.NewRealtime(ably.ClientOptions{})
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
	off := rec.Listen(client)
	defer off()

	if err := ablytest.ConnWaiter(client, nil, ably.ConnectionEventConnected).Wait(); err != nil {
		t.Fatalf("Connect()=%v", err)
	}
	if serial := client.Connection.Serial(); serial != -1 {
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
	t.Parallel()
	var rec ablytest.ConnStatesRecorder
	opts := ably.ClientOptions{}.
		AutoConnect(false)
	app, client := ablytest.NewRealtime(opts)
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
	off := rec.Listen(client)
	defer off()

	if err := ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait(); err != nil {
		t.Fatalf("Connect()=%v", err)
	}
	if serial := client.Connection.Serial(); serial != -1 {
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
	t.Parallel()
	var rec ablytest.ConnStatesRecorder
	app, client := ablytest.NewRealtime(ably.ClientOptions{})
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
	off := rec.Listen(client)
	defer off()

	if err := ablytest.ConnWaiter(client, nil, ably.ConnectionEventConnected).Wait(); err != nil {
		t.Fatal(err)
	}
	if err := ablytest.FullRealtimeCloser(client).Close(); err != nil {
		t.Fatalf("ablytest.FullRealtimeCloser(client).Close()=%v", err)
	}
	if err := ablytest.ConnWaiter(client, nil, ably.ConnectionEventClosed).Wait(); err != nil {
		t.Fatal(err)
	}
	if !ablytest.Soon.IsTrue(func() bool {
		return ablytest.Contains(rec.States(), connTransitions)
	}) {
		t.Fatalf("expected %+v, got %+v", connTransitions, rec.States())
	}
}

func TestRealtimeConn_AlreadyConnected(t *testing.T) {
	t.Parallel()
	app, client := ablytest.NewRealtime(ably.ClientOptions{}.AutoConnect(false))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait(); err != nil {
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
	opts := ably.ClientOptions{}.
		Key("abc:abc").
		UseTokenAuth(true).
		AutoConnect(false)
	client, err := ably.NewRealtime(opts)
	if err != nil {
		t.Fatalf("NewRealtime()=%v", err)
	}
	if err = ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait(); err == nil {
		t.Fatal("Connect(): want err != nil")
	}
}

func TestRealtimeConn_ReceiveTimeout(t *testing.T) {
	t.Parallel()

	const maxIdleInterval = 20
	const realtimeRequestTimeout = 10 * time.Millisecond

	in := make(chan *proto.ProtocolMessage, 16)
	out := make(chan *proto.ProtocolMessage, 16)

	connected := &proto.ProtocolMessage{
		Action:       proto.ActionConnected,
		ConnectionID: "connection-id",
		ConnectionDetails: &proto.ConnectionDetails{
			MaxIdleInterval: maxIdleInterval,
		},
	}
	in <- connected

	app, client := ablytest.NewRealtime(ably.ClientOptions{}.
		Dial(ablytest.MessagePipe(in, out, ablytest.MessagePipeWithNowFunc(time.Now))).
		RealtimeRequestTimeout(10 * time.Millisecond).
		AutoConnect(false))
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

func TestRealtimeConn_BreakConnLoopOnInactiveState(t *testing.T) {
	t.Parallel()

	for _, action := range []proto.Action{
		proto.ActionDisconnect,
		proto.ActionError,
		proto.ActionClosed,
	} {
		t.Run(action.String(), func(t *testing.T) {
			t.Parallel()
			in := make(chan *proto.ProtocolMessage)
			out := make(chan *proto.ProtocolMessage, 16)

			app, client := ablytest.NewRealtime(ably.ClientOptions{}.
				Dial(ablytest.MessagePipe(in, out)))
			defer safeclose(t, app, ablytest.FullRealtimeCloser(client))

			connected := &proto.ProtocolMessage{
				Action:            proto.ActionConnected,
				ConnectionID:      "connection-id",
				ConnectionDetails: &proto.ConnectionDetails{},
			}
			select {
			case in <- connected:
			case <-time.After(10 * time.Millisecond):
				t.Fatal("didn't receive incoming protocol message")
			}

			select {
			case in <- &proto.ProtocolMessage{
				Action: action,
			}:
			case <-time.After(10 * time.Millisecond):
				t.Fatal("didn't receive incoming protocol message")
			}

			select {
			case in <- &proto.ProtocolMessage{}:
				t.Fatal("called Receive again; expected end of connection loop")
			case <-time.After(10 * time.Millisecond):
			}
		})
	}
}
