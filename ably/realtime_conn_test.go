package ably_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"
)

func await(fn func() ably.StateEnum, state ably.StateEnum) error {
	t := time.After(ablytest.Timeout)
	for {
		select {
		case <-t:
			return fmt.Errorf("waiting for %s state has timed out after %v", state, ablytest.Timeout)
		default:
			if fn() == state {
				return nil
			}
		}
	}
}

var connTransitions = []ably.StateEnum{
	ably.StateConnConnecting,
	ably.StateConnConnected,
	ably.StateConnClosing,
	ably.StateConnClosed,
}

func TestRealtimeConn_Connect(t *testing.T) {
	t.Parallel()
	rec := ablytest.NewStateRecorder(4)
	app, client := ablytest.NewRealtime(&ably.ClientOptions{Listener: rec.Channel()})
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := await(client.Connection.State, ably.StateConnConnected); err != nil {
		t.Fatal(err)
	}
	if serial := client.Connection.Serial(); serial != -1 {
		t.Fatalf("want serial=-1; got %d", serial)
	}
	if err := ablytest.FullRealtimeCloser(client).Close(); err != nil {
		t.Fatalf("ablytest.FullRealtimeCloser(client).Close()=%v", err)
	}
	if err := rec.WaitFor(connTransitions); err != nil {
		t.Fatal(err)
	}
}

func TestRealtimeConn_NoConnect(t *testing.T) {
	t.Parallel()
	rec := ablytest.NewStateRecorder(4)
	opts := &ably.ClientOptions{
		Listener:  rec.Channel(),
		NoConnect: true,
	}
	app, client := ablytest.NewRealtime(opts)
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	client.Connection.OnState(rec.Channel())
	if err := ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait(); err != nil {
		t.Fatalf("Connect()=%v", err)
	}
	if serial := client.Connection.Serial(); serial != -1 {
		t.Fatalf("want serial=-1; got %d", serial)
	}
	if err := ablytest.FullRealtimeCloser(client).Close(); err != nil {
		t.Fatalf("ablytest.FullRealtimeCloser(client).Close()=%v", err)
	}
	rec.Stop()
	if err := rec.WaitFor(connTransitions); err != nil {
		t.Error(err)
	}
}

var connCloseTransitions = []ably.StateEnum{
	ably.StateConnConnecting,
	ably.StateConnConnected,
	ably.StateConnClosing,
	ably.StateConnClosed,
}

func TestRealtimeConn_ConnectClose(t *testing.T) {
	t.Parallel()
	rec := ablytest.NewStateRecorder(4)
	app, client := ablytest.NewRealtime(&ably.ClientOptions{Listener: rec.Channel()})
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := await(client.Connection.State, ably.StateConnConnected); err != nil {
		t.Fatal(err)
	}
	if err := ablytest.FullRealtimeCloser(client).Close(); err != nil {
		t.Fatalf("ablytest.FullRealtimeCloser(client).Close()=%v", err)
	}
	if err := await(client.Connection.State, ably.StateConnClosed); err != nil {
		t.Fatal(err)
	}
	rec.Stop()
	if err := rec.WaitFor(connCloseTransitions); err != nil {
		t.Error(err)
	}
}

func TestRealtimeConn_AlreadyConnected(t *testing.T) {
	t.Parallel()
	app, client := ablytest.NewRealtime(&ably.ClientOptions{NoConnect: true})
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
	opts := &ably.ClientOptions{
		AuthOptions: ably.AuthOptions{
			Key:          "abc:abc",
			UseTokenAuth: true,
		},
		NoConnect: true,
	}
	client, err := ably.DeprecatedNewRealtime(opts)
	if err != nil {
		t.Fatalf("NewRealtime()=%v", err)
	}
	if err = ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait(); err == nil {
		t.Fatal("Connect(): want err != nil")
	}
	if state := client.Connection.State(); state != ably.StateConnFailed {
		t.Fatalf("want state=%s; got %s", ably.StateConnFailed, state)
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

	app, client := ablytest.NewRealtime(&ably.ClientOptions{
		Dial:                   ablytest.MessagePipe(in, out),
		RealtimeRequestTimeout: 10 * time.Millisecond,
		NoConnect:              true,
	})
	defer safeclose(t, app)

	states := make(chan ably.State, 10)
	client.Connection.OnState(states, ably.StateConnConnected, ably.StateConnDisconnected)

	client.Connection.Connect()

	var state ably.State

	select {
	case state = <-states:
	case <-time.After(10 * time.Millisecond):
		t.Fatal("didn't receive state change event")
	}

	if expected, got := ably.StateConnConnected, state.State; expected != got {
		t.Fatalf("expected %v, got %v", expected, got)
	}

	leeway := 10 * time.Millisecond
	select {
	case state = <-states:
	case <-time.After(realtimeRequestTimeout + time.Duration(maxIdleInterval)*time.Millisecond + leeway):
		t.Fatal("didn't receive state change event")
	}

	if expected, got := ably.StateConnDisconnected, state.State; expected != got {
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

			app, client := ablytest.NewRealtime(&ably.ClientOptions{
				Dial: ablytest.MessagePipe(in, out),
			})
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
