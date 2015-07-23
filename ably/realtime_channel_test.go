package ably_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/proto"
	"github.com/ably/ably-go/ably/testutil"
)

func expectMsg(ch <-chan *proto.Message, name, data string, t time.Duration, received bool) error {
	select {
	case msg := <-ch:
		if !received {
			return fmt.Errorf("received unexpected message name=%q data=%q", msg.Name, msg.Data)
		}
		if msg.Name != name {
			return fmt.Errorf("want msg.Name=%q; got %q", name, msg.Name)
		}
		if msg.Data != data {
			return fmt.Errorf("want msg.Data=%q; got %q", name, msg.Name)
		}
		return nil
	case <-time.After(t):
		if received {
			return fmt.Errorf("waiting for message name=%q data=%q timed out after %v", name, data, t)
		}
		return nil
	}
}

func TestRealtimeChannel_Publish(t *testing.T) {
	t.Parallel()
	app, client := testutil.ProvisionRealtime(nil, nil)
	defer safeclose(t, client, app)

	channel := client.Channels.Get("test")
	if err := ably.Wait(channel.Publish("hello", "world")); err != nil {
		t.Fatalf("Publish()=%v", err)
	}
}

func TestRealtimeChannel_Subscribe(t *testing.T) {
	t.Parallel()
	app, client1 := testutil.ProvisionRealtime(nil, nil)
	defer safeclose(t, client1, app)
	client2, err := ably.NewRealtimeClient(app.Options(&ably.ClientOptions{NoEcho: true}))
	if err != nil {
		t.Fatalf("client2: NewRealtimeClient()=%v", err)
	}
	defer safeclose(t, client2)

	channel1 := client1.Channels.Get("test")
	sub1, err := channel1.Subscribe()
	if err != nil {
		t.Fatalf("client1: Subscribe()=%v", err)
	}
	defer sub1.Close()

	channel2 := client2.Channels.Get("test")
	sub2, err := channel2.Subscribe()
	if err != nil {
		t.Fatalf("client2: Subscribe()=%v", err)
	}
	defer sub2.Close()

	if err := ably.Wait(channel1.Publish("hello", "client1")); err != nil {
		t.Fatalf("client1: Publish()=%v", err)
	}
	if err := ably.Wait(channel2.Publish("hello", "client2")); err != nil {
		t.Fatalf("client2: Publish()=%v", err)
	}

	timeout := 15 * time.Second

	if err := expectMsg(sub1.MessageChannel(), "hello", "client1", timeout, true); err != nil {
		t.Fatal(err)
	}
	if err := expectMsg(sub1.MessageChannel(), "hello", "client2", timeout, true); err != nil {
		t.Fatal(err)
	}
	if err := expectMsg(sub2.MessageChannel(), "hello", "client1", timeout, true); err != nil {
		t.Fatal(err)
	}
	if err := expectMsg(sub2.MessageChannel(), "hello", "client2", timeout, false); err != nil {
		t.Fatal(err)
	}
}

var chanCloseTransitions = [][]ably.StateEnum{{
	ably.StateConnConnecting,
	ably.StateChanAttaching,
	ably.StateConnConnected,
	ably.StateChanAttached,
	ably.StateChanDetaching,
	ably.StateChanDetached,
	ably.StateConnClosing,
	ably.StateConnClosed,
}, {
	ably.StateConnConnecting,
	ably.StateConnConnected,
	ably.StateChanAttaching,
	ably.StateChanAttached,
	ably.StateChanDetaching,
	ably.StateChanDetached,
	ably.StateConnClosing,
	ably.StateConnClosed,
}}

func TestRealtimeChannel_Close(t *testing.T) {
	rec := ably.NewStateRecorder(8)
	app, client := testutil.ProvisionRealtime(nil, &ably.ClientOptions{Listener: rec.Channel()})
	defer safeclose(t, client, app)

	channel := client.Channels.Get("test")
	sub, err := channel.Subscribe()
	if err != nil {
		t.Fatalf("channel.Subscribe()=%v", err)
	}
	defer sub.Close()
	if err := ably.Wait(channel.Publish("hello", "world")); err != nil {
		t.Fatalf("channel.Publish()=%v", err)
	}
	done := make(chan error)
	go func() {
		msg, ok := <-sub.MessageChannel()
		if !ok {
			done <- errors.New("did not receive published message")
		}
		if msg.Name != "hello" || msg.Data != "world" {
			done <- fmt.Errorf(`want name="hello", data="world"; got %s, %s`, msg.Name, msg.Data)
		}
		if _, ok = <-sub.MessageChannel(); ok {
			done <- fmt.Errorf("expected channel to be closed")
		}
		done <- nil
	}()
	if err := channel.Close(); err != nil {
		t.Fatalf("channel.Close()=%v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("client.Close()=%v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("waiting on subscribed channel close failed: err=%s", err)
		}
	case <-time.After(timeout):
		t.Fatalf("waiting on subscribed channel close timed out after %v", timeout)
	}
	rec.Stop()
	states := rec.States()
	for _, expected := range chanCloseTransitions {
		if reflect.DeepEqual(states, expected) {
			return
		}
	}
	t.Fatalf("unexpected state transitions %v", states)
}
