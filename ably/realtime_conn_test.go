package ably_test

import (
	"fmt"
	"reflect"
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
	if err := rec.WaitFor(connTransitions, time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestRealtimeConn_NoConnect(t *testing.T) {
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
	if states := rec.States(); !reflect.DeepEqual(states, connTransitions) {
		t.Errorf("expected states=%v; got %v", connTransitions, states)
	}
}

var connCloseTransitions = []ably.StateEnum{
	ably.StateConnConnecting,
	ably.StateConnConnected,
	ably.StateConnClosing,
	ably.StateConnClosed,
}

func TestRealtimeConn_ConnectClose(t *testing.T) {
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
	if states := rec.States(); !reflect.DeepEqual(states, connCloseTransitions) {
		t.Errorf("expected states=%v; got %v", connCloseTransitions, states)
	}
}

func TestRealtimeConn_AlreadyConnected(t *testing.T) {
	app, client := ablytest.NewRealtimeClient(&ably.ClientOptions{NoConnect: true})
	defer safeclose(t, client, app)

	if err := ablytest.Wait(client.Connection.Connect()); err != nil {
		t.Fatalf("Connect=%s", err)
	}
	if err := ablytest.Wait(client.Connection.Connect()); err != nil {
		t.Fatalf("Connect=%s", err)
	}
}
