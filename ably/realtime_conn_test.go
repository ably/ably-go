package ably_test

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/testutil"
)

func record() (*[]int, chan<- ably.State, *sync.WaitGroup) {
	listen := make(chan ably.State, 16)
	states := make([]int, 0, 16)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func(states *[]int) {
		defer wg.Done()
		for state := range listen {
			*states = append(*states, state.State)
		}
	}(&states)
	return &states, listen, wg
}

func await(fn func() int, state int) error {
	t := time.After(timeout)
	for {
		select {
		case <-t:
			return fmt.Errorf("waiting for %s state has timed out after %v",
				ably.StateText(state), timeout)
		default:
			if fn() == state {
				return nil
			}
		}
	}
}

var connTransitions = []int{
	ably.StateConnConnecting,
	ably.StateConnConnected,
	ably.StateConnClosing,
	ably.StateConnClosed,
}

func TestRealtimeConn_Connect(t *testing.T) {
	states, listen, wg := record()
	app, client := testutil.ProvisionRealtime(nil, &ably.ClientOptions{Listener: listen})
	defer multiclose(client, app)

	if err := await(client.Connection.State, ably.StateConnConnected); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("client.Close()=%v", err)
	}
	close(listen)
	wg.Wait()
	if !reflect.DeepEqual(*states, connTransitions) {
		t.Errorf("expected states=%v; got %v", connTransitions, *states)
	}
}

func TestRealtimeConn_NoConnect(t *testing.T) {
	states, listen, wg := record()
	app, client := testutil.ProvisionRealtime(nil, &ably.ClientOptions{NoConnect: true})
	defer multiclose(client, app)

	client.Connection.On(listen)
	if err := ably.Wait(client.Connection.Connect()); err != nil {
		t.Fatalf("Connect()=%v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("client.Close()=%v", err)
	}
	close(listen)
	wg.Wait()
	if !reflect.DeepEqual(*states, connTransitions) {
		t.Errorf("expected states=%v; got %v", connTransitions, *states)
	}
}

var connCloseTransitions = []int{
	ably.StateConnConnecting,
	ably.StateConnConnected,
	ably.StateConnClosing,
	ably.StateConnClosed,
}

func TestRealtimeConn_ConnectClose(t *testing.T) {
	states, listen, wg := record()
	app, client := testutil.ProvisionRealtime(nil, &ably.ClientOptions{Listener: listen})
	defer multiclose(client, app)

	if err := await(client.Connection.State, ably.StateConnConnected); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("client.Close()=%v", err)
	}
	if err := await(client.Connection.State, ably.StateConnClosed); err != nil {
		t.Fatal(err)
	}
	close(listen)
	wg.Wait()
	if !reflect.DeepEqual(*states, connCloseTransitions) {
		t.Errorf("expected states=%v; got %v", connCloseTransitions, *states)
	}
}
