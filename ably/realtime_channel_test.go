package ably_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"
)

func expectMsg(ch <-chan *proto.Message, name string, data interface{}, t time.Duration, received bool) error {
	select {
	case msg := <-ch:
		if !received {
			return fmt.Errorf("received unexpected message name=%q data=%q", msg.Name, msg.Data)
		}
		if msg.Name != name {
			return fmt.Errorf("want msg.Name=%q; got %q", name, msg.Name)
		}
		if !reflect.DeepEqual(msg.Data, data) {
			return fmt.Errorf("want msg.Data=%v; got %v", data, msg.Data)
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
	app, client := ablytest.NewRealtime(nil)
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	channel := client.Channels.Get("test")
	if err := ablytest.Wait(channel.Publish("hello", "world")); err != nil {
		t.Fatalf("Publish()=%v", err)
	}
}

func TestRealtimeChannel_Subscribe(t *testing.T) {
	t.Parallel()
	app, client1 := ablytest.NewRealtime(nil)
	defer safeclose(t, ablytest.FullRealtimeCloser(client1), app)
	client2 := app.NewRealtime(ably.ClientOptions{}.EchoMessages(false))
	defer safeclose(t, ablytest.FullRealtimeCloser(client2))

	channel1 := client1.Channels.Get("test")
	channel2 := client2.Channels.Get("test")

	if err := ablytest.Wait(channel1.Attach()); err != nil {
		t.Fatalf("client1: Attach()=%v", err)
	}
	if err := ablytest.Wait(channel2.Attach()); err != nil {
		t.Fatalf("client2: Attach()=%v", err)
	}

	sub1, err := channel1.Subscribe()
	if err != nil {
		t.Fatalf("client1: Subscribe()=%v", err)
	}
	defer sub1.Close()

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

func TestRealtimeChannel_Detach(t *testing.T) {
	t.Parallel()
	app, client := ablytest.NewRealtime(ably.ClientOptions{})
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

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
		if msg.Name != "hello" || !reflect.DeepEqual(msg.Data, "world") {
			done <- fmt.Errorf(`want name="hello", data="world"; got %s, %v`, msg.Name, msg.Data)
		}
		done <- nil
	}()
	if state := channel.State(); state != ably.ChannelStateAttached {
		t.Fatalf("want state=%v; got %v", ably.ChannelStateAttached, state)
	}
	if err := ablytest.Wait(channel.Detach()); err != nil {
		t.Fatalf("channel.Detach()=%v", err)
	}
	if err := ablytest.FullRealtimeCloser(client).Close(); err != nil {
		t.Fatalf("ablytest.FullRealtimeCloser(client).Close()=%v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("waiting on subscribed channel close failed: err=%s", err)
		}
	case <-time.After(ablytest.Timeout):
		t.Fatalf("waiting on subscribed channel close timed out after %v", ablytest.Timeout)
	}
}
