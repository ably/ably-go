package ably_test

import (
	"fmt"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/internal/ablyutil"
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
	app, client := ablytest.NewRealtimeClient(&ably.ClientOptions{Listener: rec.Channel()})
	defer safeclose(t, client, app)

	if err := await(client.Connection.State, ably.StateConnConnected); err != nil {
		t.Fatal(err)
	}
	if serial := client.Connection.Serial(); serial != -1 {
		t.Fatalf("want serial=-1; got %d", serial)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("client.Close()=%v", err)
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
	app, client := ablytest.NewRealtimeClient(opts)
	defer safeclose(t, client, app)

	client.Connection.On(rec.Channel())
	if err := ablytest.Wait(client.Connection.Connect()); err != nil {
		t.Fatalf("Connect()=%v", err)
	}
	if serial := client.Connection.Serial(); serial != -1 {
		t.Fatalf("want serial=-1; got %d", serial)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("client.Close()=%v", err)
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
	app, client := ablytest.NewRealtimeClient(&ably.ClientOptions{Listener: rec.Channel()})
	defer safeclose(t, client, app)

	if err := await(client.Connection.State, ably.StateConnConnected); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("client.Close()=%v", err)
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
	app, client := ablytest.NewRealtimeClient(&ably.ClientOptions{NoConnect: true})
	defer safeclose(t, client, app)

	if err := ablytest.Wait(client.Connection.Connect()); err != nil {
		t.Fatalf("Connect=%s", err)
	}
	if err := ablytest.Wait(client.Connection.Connect()); err != nil {
		t.Fatalf("Connect=%s", err)
	}
}

func TestRealtimeConn_AuthError(t *testing.T) {
	opts := &ably.ClientOptions{
		AuthOptions: ably.AuthOptions{
			Key:          "abc:abc",
			UseTokenAuth: true,
		},
		NoConnect: true,
	}
	client, err := ably.NewRealtimeClient(opts)
	if err != nil {
		t.Fatalf("NewRealtimeClient()=%v", err)
	}
	defer safeclose(t, client)
	if err = ablytest.Wait(client.Connection.Connect()); err == nil {
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

	app, client := ablytest.NewRealtimeClient(&ably.ClientOptions{
		Dial:                   ablytest.MessagePipe(in, out),
		RealtimeRequestTimeout: 10 * time.Millisecond,
		NoConnect:              true,
	})
	defer safeclose(t, app)

	states := make(chan ably.State, 10)
	client.Connection.On(states, ably.StateConnConnected, ably.StateConnFailed)

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

	// TODO: Should be Disconnected, not Failed
	// Part of https://ably.atlassian.net/browse/FEA-391
	if expected, got := ably.StateConnFailed, state.State; expected != got {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestRealtimeConn_ReconnectOnEOF(t *testing.T) {
	t.Parallel()

	doEOF := make(chan struct{}, 1)

	app, client := ablytest.NewRealtimeClient(&ably.ClientOptions{
		NoConnect: true,
		Dial: func(protocol string, u *url.URL) (proto.Conn, error) {
			c, err := ablyutil.DialWebsocket(protocol, u)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF}, err
		},
	})
	defer safeclose(t, client, app)

	if err := ablytest.Wait(client.Connection.Connect()); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	stateChanges := make(chan ably.State, 16)
	client.Connection.On(stateChanges)

	doEOF <- struct{}{}

	var state ably.State

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't transition on EOF")
	}

	if expected, got := ably.StateConnDisconnected, state; expected != got.State {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't reconnect")
	}

	if expected, got := ably.StateConnConnecting, state; expected != got.State {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}

	select {
	case state = <-stateChanges:
	case <-time.After(ablytest.Timeout):
		t.Fatal("didn't transition from CONNECTING")
	}

	if expected, got := ably.StateConnConnected, state; expected != got.State {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}
}

type protoConnWithFakeEOF struct {
	proto.Conn
	doEOF <-chan struct{}
}

func (c protoConnWithFakeEOF) Receive(deadline time.Time) (*proto.ProtocolMessage, error) {
	type result struct {
		msg *proto.ProtocolMessage
		err error
	}

	received := make(chan result, 1)

	go func() {
		msg, err := c.Conn.Receive(deadline)
		received <- result{msg: msg, err: err}

	}()

	select {
	case r := <-received:
		return r.msg, r.err
	case <-c.doEOF:
		return nil, io.EOF
	}
}
