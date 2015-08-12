package ably_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/testutil"
)

func await(fn func() ably.StateEnum, state ably.StateEnum) error {
	t := time.After(timeout)
	for {
		select {
		case <-t:
			return fmt.Errorf("waiting for %s state has timed out after %v", state, timeout)
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
	rec := ably.NewStateRecorder(4)
	app, client := testutil.ProvisionRealtime(nil, &ably.ClientOptions{Listener: rec.Channel()})
	defer safeclose(t, client, app)

	if err := await(client.Connection.State, ably.StateConnConnected); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("client.Close()=%v", err)
	}
	rec.Stop()
	if states := rec.States(); !reflect.DeepEqual(states, connTransitions) {
		t.Errorf("expected states=%v; got %v", connTransitions, states)
	}
}

func TestRealtimeConn_NoConnect(t *testing.T) {
	rec := ably.NewStateRecorder(4)
	app, client := testutil.ProvisionRealtime(nil, &ably.ClientOptions{NoConnect: true})
	defer safeclose(t, client, app)

	client.Connection.On(rec.Channel())
	if err := ably.Wait(client.Connection.Connect()); err != nil {
		t.Fatalf("Connect()=%v", err)
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
	rec := ably.NewStateRecorder(4)
	app, client := testutil.ProvisionRealtime(nil, &ably.ClientOptions{Listener: rec.Channel()})
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
	app, client := testutil.ProvisionRealtime(nil, &ably.ClientOptions{NoConnect: true})
	defer safeclose(t, client, app)

	if err := ably.Wait(client.Connection.Connect()); err != nil {
		t.Fatalf("Connect=%s", err)
	}
	if err := ably.Wait(client.Connection.Connect()); err != nil {
		t.Fatalf("Connect=%s", err)
	}
}
