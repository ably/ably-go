package ably_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"
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
	app, client := ablytest.NewRealtimeClient(nil)
	defer safeclose(t, client, app)

	channel := client.Channels.Get("test")
	if err := ablytest.Wait(channel.Publish("hello", "world")); err != nil {
		t.Fatalf("Publish()=%v", err)
	}
}

func TestRealtimeChannel_Failed(t *testing.T) {
	t.Parallel()
	rec := ablytest.NewStateChanRecorder(5)
	opts := &ably.ClientOptions{
		NoConnect:  true,
		NoQueueing: true,
		Listener:   rec.Channel(),
	}
	app, client := ablytest.NewRealtimeClient(opts)
	defer safeclose(t, client, app)

	if err := ablytest.Wait(client.Connection.Connect()); err != nil {
		t.Fatalf("Connect()=%v", err)
	}
	channel := client.Channels.GetAndAttach("test")
	if err := client.Close(); err != nil {
		t.Fatalf("Close()=%v", err)
	}
	want := []ably.StateEnum{
		ably.StateChanAttaching,
		ably.StateChanAttached,
		ably.StateChanClosed,
		ably.StateChanFailed,
	}
	if err := rec.WaitFor(want[:3]); err != nil {
		t.Fatal(err)
	}
	if err := checkError(80000, ablytest.Wait(channel.Publish("im", "closed"))); err != nil {
		t.Fatal(err)
	}
	if err := rec.WaitFor(want); err != nil {
		t.Fatal(err)
	}
	if err := checkError(80000, ablytest.Wait(channel.Detach())); err != nil {
		t.Fatal(err)
	}
	if len(rec.Errors()) == 0 {
		t.Fatal("want len(errors) != 0")
	}
}

func TestRealtimeChannel_Subscribe(t *testing.T) {
	t.Parallel()
	app, client1 := ablytest.NewRealtimeClient(nil)
	defer safeclose(t, client1, app)
	client2 := app.NewRealtimeClient(&ably.ClientOptions{NoEcho: true})
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

	if err := ablytest.Wait(channel1.Publish("hello", "client1")); err != nil {
		t.Fatalf("client1: Publish()=%v", err)
	}
	if err := ablytest.Wait(channel2.Publish("hello", "client2")); err != nil {
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
	t.Parallel()
	rec := ablytest.NewStateRecorder(8)
	app, client := ablytest.NewRealtimeClient(&ably.ClientOptions{Listener: rec.Channel()})
	defer safeclose(t, client, app)

	channel := client.Channels.Get("test")
	sub, err := channel.Subscribe()
	if err != nil {
		t.Fatalf("channel.Subscribe()=%v", err)
	}
	defer sub.Close()
	if err := ablytest.Wait(channel.Publish("hello", "world")); err != nil {
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
	if state := channel.State(); state != ably.StateChanAttached {
		t.Fatalf("want state=%v; got %v", ably.StateChanAttached, state)
	}
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
	case <-time.After(ablytest.Timeout):
		t.Fatalf("waiting on subscribed channel close timed out after %v", ablytest.Timeout)
	}
	rec.Stop()
	for _, expected := range chanCloseTransitions {
		if err = rec.WaitFor(expected); err != nil {
			return
		}
	}
	t.Error(err)
}
