package ably_test

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/testutil"
)

func record(t *testing.T, spy map[int]int, done func() bool) (chan<- ably.State, func()) {
	ch := make(chan ably.State, 64)
	stop := time.After(timeout)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				t.Fatalf("record timed out after %v (spy=%v)", timeout, spy)
			case s := <-ch:
				spy[s.State]++
			default:
				if done() {
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()
	return ch, func() { wg.Wait() }
}

func TestRealtimeConn_Connect(t *testing.T) {
	client, err := ably.NewRealtimeClient(testutil.Options(t, nil))
	if err != nil {
		t.Fatalf("ably.NewRealtimeClient=%v", err)
	}
	defer client.Close()
	record(t, nil, func() bool { return client.Connection.State() == ably.StateConnConnected })
}

func TestRealtimeConn_NoConnect(t *testing.T) {
	spy := map[int]int{}
	expected := map[int]int{
		ably.StateConnConnecting: 1,
		ably.StateConnConnected:  1,
	}
	opts := testutil.Options(t, nil)
	opts.NoConnect = true

	client, err := ably.NewRealtimeClient(opts)
	if err != nil {
		t.Fatalf("NewRealtimeClient(%v)=%v", opts, err)
	}
	defer client.Close()

	ch, wait := record(t, spy, func() bool { return client.Connection.State() == ably.StateConnConnected })
	client.Connection.On(ch)
	if err := client.Connection.Connect(); err != nil {
		t.Fatalf("Connect()=%v", err)
	}
	wait()
	if !reflect.DeepEqual(spy, expected) {
		t.Errorf("expected spy=%v; got %v", expected, spy)
	}
}

func TestRealtimeConn_Queueing(t *testing.T) {
	t.Skip("TODO")
}
