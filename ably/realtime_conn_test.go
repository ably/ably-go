package ably_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
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
	app, client := ablytest.NewRealtimeClient(opts)
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	client.Connection.On(rec.Channel())
	if err := ablytest.Wait(client.Connection.Connect()); err != nil {
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
	app, client := ablytest.NewRealtimeClient(&ably.ClientOptions{Listener: rec.Channel()})
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
	app, client := ablytest.NewRealtimeClient(&ably.ClientOptions{NoConnect: true})
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

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
	if err = ablytest.Wait(client.Connection.Connect()); err == nil {
		t.Fatal("Connect(): want err != nil")
	}
	if state := client.Connection.State(); state != ably.StateConnFailed {
		t.Fatalf("want state=%s; got %s", ably.StateConnFailed, state)
	}
	if err = ablytest.FullRealtimeCloser(client).Close(); err == nil {
		t.Fatal("Close(): want err != nil")
	}
}
