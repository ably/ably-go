package ably_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ably/proto"
)

func expectMsg(ch <-chan proto.Message, name string, data interface{}, t time.Duration, received bool) error {
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
	if err := channel.Publish(context.Background(), "hello", "world"); err != nil {
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

	if err := channel1.Attach(context.Background()); err != nil {
		t.Fatalf("client1: Attach()=%v", err)
	}
	if err := channel2.Attach(context.Background()); err != nil {
		t.Fatalf("client2: Attach()=%v", err)
	}

	sub1, unsub1, err := ablytest.ReceiveMessages(channel1, "")
	if err != nil {
		t.Fatalf("client1:.Subscribe(context.Background())=%v", err)
	}
	defer unsub1()

	sub2, unsub2, err := ablytest.ReceiveMessages(channel2, "")
	if err != nil {
		t.Fatalf("client2:.Subscribe(context.Background())=%v", err)
	}
	defer unsub2()

	if err := channel1.Publish(context.Background(), "hello", "client1"); err != nil {
		t.Fatalf("client1: Publish()=%v", err)
	}
	if err := channel2.Publish(context.Background(), "hello", "client2"); err != nil {
		t.Fatalf("client2: Publish()=%v", err)
	}

	timeout := 15 * time.Second

	if err := expectMsg(sub1, "hello", "client1", timeout, true); err != nil {
		t.Fatal(err)
	}
	if err := expectMsg(sub1, "hello", "client2", timeout, true); err != nil {
		t.Fatal(err)
	}
	if err := expectMsg(sub2, "hello", "client1", timeout, true); err != nil {
		t.Fatal(err)
	}
	if err := expectMsg(sub2, "hello", "client2", timeout, false); err != nil {
		t.Fatal(err)
	}
}

func TestRealtimeChannel_Detach(t *testing.T) {
	t.Parallel()
	app, client := ablytest.NewRealtime(ably.ClientOptions{})
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	channel := client.Channels.Get("test")
	sub, unsub, err := ablytest.ReceiveMessages(channel, "")
	if err != nil {
		t.Fatalf("ablytest.ReceiveMessages(channel)=%v", err)
	}
	defer unsub()
	if err := channel.Publish(context.Background(), "hello", "world"); err != nil {
		t.Fatalf("channel.Publish()=%v", err)
	}
	done := make(chan error)
	go func() {
		msg, ok := <-sub
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
	if err := channel.Detach(context.Background()); err != nil {
		t.Fatalf("channel.Detach(context.Background())=%v", err)
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

func TestRealtimeChannel_AttachWhileDisconnected(t *testing.T) {
	t.Parallel()

	doEOF := make(chan struct{}, 1)
	allowDial := make(chan struct{}, 1)
	allowDial <- struct{}{}

	app, client := ablytest.NewRealtime(ably.ClientOptions{}.
		AutoConnect(false).
		Dial(func(protocol string, u *url.URL) (proto.Conn, error) {
			<-allowDial
			c, err := ablyutil.DialWebsocket(protocol, u)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	channel := client.Channels.Get("test")

	if err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatal(err)
	}

	// Move to DISCONNECTED.

	disconnected := make(ably.ConnStateChanges, 1)
	off := client.Connection.On(ably.ConnectionEventDisconnected, disconnected.Receive)
	doEOF <- struct{}{}
	off()

	// Attempt ATTACH. It should be blocked until we're CONNECTED again.

	attached := make(ably.ChannelStateChanges, 1)
	channel.On(ably.ChannelEventAttached, attached.Receive)

	res := make(chan ably.Result)
	go func() {
		res <- ablytest.ResultFunc.Go(func() error { return channel.Attach(context.Background()) })
	}()
	ablytest.Before(1*time.Second).NoRecv(t, nil, attached, t.Fatalf)

	// Allow another dial, which should eventually move the connection to
	// CONNECTED, thus allowing the attachment to complete.

	allowDial <- struct{}{}

	ablytest.Soon.Recv(t, nil, attached, t.Fatalf)

	if err := ablytest.Wait(<-res, nil); err != nil {
		t.Fatal(err)
	}
}
