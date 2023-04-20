//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"

	"github.com/stretchr/testify/assert"
)

func expectMsg(ch <-chan *ably.Message, name string, data interface{}, t time.Duration, received bool) error {
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
	app, client := ablytest.NewRealtime(nil...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
	err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)
	channel := client.Channels.Get("test")
	err = channel.Publish(context.Background(), "hello", "world")
	assert.NoError(t, err, "Publish()=%v", err)
}

func TestRealtimeChannel_Subscribe(t *testing.T) {
	app, client1 := ablytest.NewRealtime(nil...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client1), app)
	client2 := app.NewRealtime(ably.WithEchoMessages(false))
	defer safeclose(t, ablytest.FullRealtimeCloser(client2))

	err := ablytest.Wait(ablytest.ConnWaiter(client1, client1.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)
	err = ablytest.Wait(ablytest.ConnWaiter(client2, client2.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	channel1 := client1.Channels.Get("test")
	channel2 := client2.Channels.Get("test")

	err = channel1.Attach(context.Background())
	assert.NoError(t, err,
		"client1: Attach()=%v", err)
	err = channel2.Attach(context.Background())
	assert.NoError(t, err,
		"client2: Attach()=%v", err)

	sub1, unsub1, err := ablytest.ReceiveMessages(channel1, "")
	assert.NoError(t, err, "client1:.Subscribe(context.Background())=%v", err)
	defer unsub1()
	sub2, unsub2, err := ablytest.ReceiveMessages(channel2, "")
	assert.NoError(t, err, "client2:.Subscribe(context.Background())=%v", err)
	defer unsub2()

	err = channel1.Publish(context.Background(), "hello", "client1")
	assert.NoError(t, err, "client1: Publish()=%v", err)
	err = channel2.Publish(context.Background(), "hello", "client2")
	assert.NoError(t, err, "client2: Publish()=%v", err)

	timeout := 15 * time.Second

	err = expectMsg(sub1, "hello", "client1", timeout, true)
	assert.NoError(t, err)
	err = expectMsg(sub1, "hello", "client2", timeout, true)
	assert.NoError(t, err)
	err = expectMsg(sub2, "hello", "client1", timeout, true)
	assert.NoError(t, err)
	err = expectMsg(sub2, "hello", "client2", timeout, false)
	assert.NoError(t, err)
}

func TestRealtimeChannel_SubscriptionFilters(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	assert.NoError(t, err)
	defer app.Close()
	options := app.Options()
	restClient, err := ably.NewREST(options...)
	assert.NoError(t, err)
	msg := []*ably.Message{
		{
			Name: "filtered",
			Data: "This should be filtered",
			Extras: map[string]interface{}{
				"headers": map[string]interface{}{
					"name":   "value one",
					"number": 1234,
					"bool":   true,
				},
			},
		},
		{
			Name: "filtered",
			Data: "A different data",
			Extras: map[string]interface{}{
				"headers": map[string]interface{}{
					"name":   "random value",
					"number": 6789,
					"bool":   true,
				},
			},
		},
		{
			Name: "filtered",
			Data: "No extra data",
		},
	}
	filter := ably.DeriveOptions{
		Filter: "name == `'filtered'` && headers.number == `1234`",
	}

	realtimeClient := app.NewRealtime(ably.WithEchoMessages(false))
	defer safeclose(t, ablytest.FullRealtimeCloser(realtimeClient))

	err = ablytest.Wait(ablytest.ConnWaiter(realtimeClient, realtimeClient.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	restChannel := restClient.Channels.Get("test")
	realtimeChannel, _ := realtimeClient.Channels.GetDerived("test", filter)

	err = realtimeChannel.Attach(context.Background())
	assert.NoError(t, err,
		"realtimeClient: Attach()=%v", err)

	sub, unsub, err := ablytest.ReceiveMessages(realtimeChannel, "filtered")
	assert.NoError(t, err, "realtimeClient:.Subscribe(context.Background())=%v", err)
	defer unsub()

	err = restChannel.PublishMultiple(context.Background(), msg)
	assert.NoError(t, err, "restClient: Publish()=%v", err)

	timeout := 15 * time.Second

	err = expectMsg(sub, "filtered", msg[0].Data, timeout, true)
	assert.NoError(t, err)
}

func TestRealtimeChannel_AttachWhileDisconnected(t *testing.T) {

	doEOF := make(chan struct{}, 1)
	allowDial := make(chan struct{}, 1)
	allowDial <- struct{}{}

	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			<-allowDial
			c, err := ably.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	channel := client.Channels.Get("test")

	err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	// Move to DISCONNECTED.

	disconnected := make(ably.ConnStateChanges, 1)
	off := client.Connection.On(ably.ConnectionEventDisconnected, disconnected.Receive)
	doEOF <- struct{}{}
	off()

	// Attempt ATTACH. It should be blocked until we're CONNECTED again.

	attached := make(ably.ChannelStateChanges, 1)
	channel.On(ably.ChannelEventAttached, attached.Receive)

	res := make(chan Result)
	go func() {
		res <- ablytest.ResultFunc.Go(func(ctx context.Context) error { return channel.Attach(ctx) })
	}()
	ablytest.Before(1*time.Second).NoRecv(t, nil, attached, t.Fatalf)

	// Allow another dial, which should eventually move the connection to
	// CONNECTED, thus allowing the attachment to complete.

	allowDial <- struct{}{}

	ablytest.Soon.Recv(t, nil, attached, t.Fatalf)

	err = ablytest.Wait(<-res, nil)
	assert.NoError(t, err)
}
