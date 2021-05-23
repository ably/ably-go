package ably_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"
)

func TestRealtimeChannel_RTL2_ChannelEventForStateChange(t *testing.T) {
	t.Parallel()

	t.Run(fmt.Sprintf("on %s", ably.ChannelStateAttaching), func(t *testing.T) {
		t.Parallel()

		app, realtime := ablytest.NewRealtime(ably.WithAutoConnect(false))
		defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

		connectAndWait(t, realtime)

		changes := make(chan ably.ChannelStateChange)
		defer ablytest.Instantly.NoRecv(t, nil, changes, t.Errorf)

		channel := realtime.Channels.Get("test")

		channel.On(ably.ChannelEventAttaching, func(change ably.ChannelStateChange) {
			changes <- change
		})

		err := channel.Attach(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		ablytest.Soon.Recv(t, nil, changes, t.Fatalf)
	})

	t.Run(fmt.Sprintf("on %s", ably.ChannelStateAttached), func(t *testing.T) {
		t.Parallel()

		app, realtime := ablytest.NewRealtime(ably.WithAutoConnect(false))
		defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

		connectAndWait(t, realtime)

		channel := realtime.Channels.Get("test")

		attachAndWait(t, channel)
	})

	t.Run(fmt.Sprintf("on %s", ably.ChannelStateDetaching), func(t *testing.T) {
		t.Parallel()

		app, realtime := ablytest.NewRealtime(ably.WithAutoConnect(false))
		defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

		connectAndWait(t, realtime)

		channel := realtime.Channels.Get("test")

		attachAndWait(t, channel)

		changes := make(chan ably.ChannelStateChange)
		defer ablytest.Instantly.NoRecv(t, nil, changes, t.Errorf)

		channel.On(ably.ChannelEventDetaching, func(change ably.ChannelStateChange) {
			changes <- change
		})

		err := channel.Detach(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		ablytest.Soon.Recv(t, nil, changes, t.Fatalf)
	})

	t.Run(fmt.Sprintf("on %s", ably.ChannelStateDetached), func(t *testing.T) {
		t.Parallel()

		app, realtime := ablytest.NewRealtime(ably.WithAutoConnect(false))
		defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

		connectAndWait(t, realtime)

		channel := realtime.Channels.Get("test")

		attachAndWait(t, channel)

		changes := make(chan ably.ChannelStateChange)
		defer ablytest.Instantly.NoRecv(t, nil, changes, t.Errorf)

		channel.On(ably.ChannelEventDetached, func(change ably.ChannelStateChange) {
			changes <- change
		})

		err := channel.Detach(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		ablytest.Soon.Recv(t, nil, changes, t.Fatalf)
	})

	t.Run(fmt.Sprintf("on %s", ably.ChannelStateSuspended), func(t *testing.T) {
		t.Parallel()

		t.Skip("SUSPENDED not yet implemented")
	})

	t.Run(fmt.Sprintf("on %s", ably.ChannelEventUpdate), func(t *testing.T) {
		t.Parallel()

		t.Skip("UPDATED not yet implemented")
	})
}

func attachAndWait(t *testing.T, channel *ably.RealtimeChannel) {
	t.Helper()

	changes := make(chan ably.ChannelStateChange, 2)
	defer ablytest.Instantly.NoRecv(t, nil, changes, t.Errorf)

	{
		off := channel.Once(ably.ChannelEventAttached, func(change ably.ChannelStateChange) {
			changes <- change
		})
		defer off()
	}

	{
		off := channel.Once(ably.ChannelEventFailed, func(change ably.ChannelStateChange) {
			changes <- change
		})
		defer off()
	}

	err := channel.Attach(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var change ably.ChannelStateChange
	ablytest.Soon.Recv(t, &change, changes, t.Fatalf)

	if change.Current != ably.ChannelStateAttached {
		t.Fatalf("unexpected FAILED event: %s", change.Reason)
	}
}

func TestRealtimeChannel_RTL4_Attach(t *testing.T) {

	const channelRetryTimeout = 123 * time.Millisecond

	setup := func(t *testing.T) (
		in, out chan *proto.ProtocolMessage,
		c *ably.Realtime,
		channel *ably.RealtimeChannel,
		stateChanges ably.ChannelStateChanges,
		afterCalls chan ablytest.AfterCall,
	) {
		in = make(chan *proto.ProtocolMessage, 1)
		out = make(chan *proto.ProtocolMessage, 16)
		afterCalls = make(chan ablytest.AfterCall, 1)
		now, after := ablytest.TimeFuncs(afterCalls)

		c, _ = ably.NewRealtime(
			ably.WithToken("fake:token"),
			ably.WithAutoConnect(false),
			ably.WithNow(now),
			ably.WithAfter(after),
			ably.WithChannelRetryTimeout(channelRetryTimeout),
			ably.WithDial(ablytest.MessagePipe(in, out)),
		)

		in <- &proto.ProtocolMessage{
			Action:            proto.ActionConnected,
			ConnectionID:      "connection-id",
			ConnectionDetails: &proto.ConnectionDetails{},
		}

		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		if err != nil {
			t.Fatal(err)
		}

		channel = c.Channels.Get("test")
		stateChanges = make(ably.ChannelStateChanges, 10)
		return
	}

	t.Run("RTL4a: If already attached, nothing is done", func(t *testing.T) {
		in, out, _, channel, stateChanges, _ := setup(t)
		ctx, cancel := context.WithCancel(context.Background())

		cancel()
		channel.OnAll(stateChanges.Receive)

		// Get the channel to ATTACHED.

		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}

		var change ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		// Attach the channel again
		err := channel.Attach(ctx)

		if err != nil {
			t.Fatal(err)
		}
		ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)
	})

	t.Run("RTL4b: If connection state is INITIALIZED, CLOSING, CLOSED returns error", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()

		c, close := TransitionConn(t, recorder.Dial, app.Options()...)
		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// connection is initialized
		err = channel.Attach(ctx)

		// Check that the attach message isn't sent
		checkIfAttachSent := recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); attachSent {
			t.Fatalf("Attach message was sent before connection is established")
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Shouldn't change to attaching

		if expected, got := "cannot Attach channel because connection is in INITIALIZED state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}

		close = c.To(
			connecting,
			connected,
			closing,
		)

		defer safeclose(t, close)

		// connection is closing
		err = channel.Attach(ctx)

		// Check that the attach message isn't sent
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); attachSent {
			t.Fatalf("Attach message was sent before connection is established")
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Shouldn't change to attaching

		if expected, got := "cannot Attach channel because connection is in CLOSING state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}

		close = c.To(
			closed,
		)
		defer safeclose(t, close)

		// connection is closed
		err = channel.Attach(ctx)

		// Check that the attach message isn't sent
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); attachSent {
			t.Fatalf("Attach message was sent before connection is established")
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Shouldn't change to attaching

		if expected, got := "cannot Attach channel because connection is in CLOSED state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Shouldn't change to attaching

	})

	t.Run("RTL4b: If connection state is FAILED, returns error", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()

		c, close := TransitionConn(t, recorder.Dial, app.Options()...)

		close = c.To(
			connecting,
			failed,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// connection is failed
		err = channel.Attach(ctx)

		// Check that the attach message isn't sent
		checkIfAttachSent := recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); attachSent {
			t.Fatalf("Attach message was sent before connection is established")
		}

		if expected, got := "cannot Attach channel because connection is in FAILED state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf)
	})

	t.Run("RTL4b: If connection state is SUSPENDED, returns error", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()

		c, close := TransitionConn(t, recorder.Dial, app.Options()...)
		defer safeclose(t, close)

		close = c.To(
			connecting,
			disconnected,
			suspended,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// connection state is suspended
		err = channel.Attach(ctx)

		// Check that the attach message isn't sent
		checkIfAttachSent := recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); attachSent {
			t.Fatalf("Attach message was sent before connection is established")
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // shouldn't send attach message

		if expected, got := "cannot Attach channel because connection is in SUSPENDED state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf)

	})

	t.Run("RTL4c RTL4d: If connected, should get attached successfully", func(t *testing.T) {
		t.Parallel()

		recorder := ablytest.NewMessageRecorder()

		app, client := ablytest.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithDial(recorder.Dial))

		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

		connectionStateChanges := make(ably.ConnStateChanges, 10)
		off := client.Connection.OnAll(connectionStateChanges.Receive)
		defer off()

		client.Connect()

		var connectionChange ably.ConnectionStateChange

		ablytest.Soon.Recv(t, &connectionChange, connectionStateChanges, t.Fatalf)
		if expected, got := ably.ConnectionStateConnecting, connectionChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, connectionChange)
		}

		ablytest.Soon.Recv(t, &connectionChange, connectionStateChanges, t.Fatalf)
		if expected, got := ably.ConnectionStateConnected, connectionChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, connectionChange)
		}

		channel := client.Channels.Get("test")
		channelStateChanges := make(ably.ChannelStateChanges, 10)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		channeloff := channel.OnAll(channelStateChanges.Receive)
		defer channeloff()

		var channelStatechange ably.ChannelStateChange

		err := channel.Attach(ctx)

		// Check that the attach message is sent
		checkIfAttachSent := recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); !attachSent {
			t.Fatalf("Should send attach message, since connection state is connected")
		}

		if err != nil {
			t.Fatal(err)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, connectionStateChanges, t.Fatalf)

	})

	t.Run("RTL4d : should return error on FAILED while attaching channel", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)

		defer safeclose(t, close)

		close = c.To(
			connecting,
			connected,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		channelTransitioner.To(chAttaching, chFailed)

		// Check that the attach message is sent
		checkIfAttachSent := recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); !attachSent {
			t.Fatalf("Should send attach message, since connection state is connected")
		}

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateFailed, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		attachErr := <-channelTransitioner.err[chAttaching]

		if attachErr == nil {
			t.Fatal("attach should return channel failed error")
		}

		if ably.UnwrapErrorCode(attachErr) != 50001 {
			t.Fatalf("want code=50001; got %d", ably.UnwrapErrorCode(err))
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Should not make any change to the channel state
	})

	t.Run("RTL4d : should return error on DETACHED while attaching channel", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)

		defer safeclose(t, close)

		close = c.To(
			connecting,
			connected,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		channelTransitioner.To(chAttaching, chDetached)

		// Check that the attach message is sent
		checkIfAttachSent := recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); !attachSent {
			t.Fatalf("Should send attach message, since connection state is connected")
		}

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		attachErr := <-channelTransitioner.err[chAttaching]

		if attachErr == nil {
			t.Fatal("attach should return channel detached error")
		}

		if ably.UnwrapErrorCode(attachErr) != 50001 {
			t.Fatalf("want code=50001; got %d", ably.UnwrapErrorCode(err))
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Should not make any change to the channel state
	})

	t.Run("RTL4d : should return error on SUSPENDED while attaching channel", func(t *testing.T) {
		t.Parallel()
		t.Skip("Channel SUSPENDED not implemented yet")
		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)

		defer safeclose(t, close)

		close = c.To(
			connecting,
			connected,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		channelTransitioner.To(chAttaching, chSuspended)

		// Check that the attach message is sent
		checkIfAttachSent := recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); !attachSent {
			t.Fatalf("Should send attach message, since connection state is connected")
		}

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateSuspended, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		attachErr := <-channelTransitioner.err[chAttaching]

		if attachErr == nil {
			t.Fatal("attach should return channel failed error")
		}

		if ably.UnwrapErrorCode(attachErr) != 50001 {
			t.Fatalf("want code=50001; got %d", ably.UnwrapErrorCode(err))
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Should not make any change to the channel state
	})

	t.Run("RTL4e: Transition to failed if no attach permission", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		rest, _ := ably.NewREST(app.Options()...)
		var params ably.TokenParams
		params.Capability = `{"foo":["subscribe"]}`
		token, _ := rest.Auth.RequestToken(context.Background(), &params)

		realtime := app.NewRealtime(ably.WithToken(token.Token))
		err = realtime.Channels.Get("nofoo").Attach(context.Background())

		if err == nil {
			t.Fatal("Shouldn't attach channel with server")
		}

		if ably.UnwrapErrorCode(err) != 40160 {
			t.Fatalf("want code=40160; got %d", ably.UnwrapErrorCode(err))
		}

		if ably.UnwrapStatusCode(err) != http.StatusUnauthorized {
			t.Fatalf("error status should be unauthorized")
		}
	})

	t.Run("RTL4f: Channel attach timeout if not received within realtime request timeout", func(t *testing.T) {
		t.Parallel()

		_, out, _, channel, stateChanges, afterCalls := setup(t)
		channel.OnAll(stateChanges.Receive)

		var change ably.ChannelStateChange

		var outMsg *proto.ProtocolMessage

		errCh := asyncAttach(channel)

		// get the timer.
		var afterCall ablytest.AfterCall
		ablytest.Instantly.Recv(t, &afterCall, afterCalls, t.Fatalf)

		if actualTimeout, expectedTimeout := afterCall.D, 10*time.Second; actualTimeout != expectedTimeout {
			t.Fatal("attach request timeout is not equal to 10s")
		}

		// cause a timeout
		afterCall.Fire()

		var err error
		ablytest.Instantly.Recv(t, &err, errCh, t.Fatalf)
		if err == nil {
			t.Fatal("attach should return timeout error")
		}

		if ably.UnwrapErrorCode(err) != 50003 {
			t.Fatalf("want code=50003; got %d", ably.UnwrapErrorCode(err))
		}

		ablytest.Instantly.Recv(t, &outMsg, out, t.Fatalf)
		if expected, got := proto.ActionAttach, outMsg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, outMsg.Action)
		}

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		// setting channelstate to suspended, since channel attach failed
		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateSuspended, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		if expected, got := err, change.Reason; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		ablytest.Instantly.NoRecv(t, &change, stateChanges, t.Fatalf)
	})

	t.Run("RTL4g: If channel in FAILED state, set err to null and proceed with attach", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)
		defer safeclose(t, close)

		close = c.To(
			connecting,
			connected,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		channelTransitioner.To(chAttaching, chFailed)

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		// Check that the attach message is sent
		checkIfAttachSent := recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); !attachSent {
			t.Fatalf("Should send attach message, since connection state is connected")
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateFailed, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		// checking connection state is still connected
		if expected, got := ably.ConnectionStateConnected, c.Realtime.Connection.State(); expected != got {
			t.Fatalf("expected %v; got %v", expected, got)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		recorder.Reset() //reset the recorded messages to zero

		err = channel.Attach(ctx)

		// Check that the attach message is sent
		checkIfAttachSent = recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); !attachSent {
			t.Fatalf("Should send attach message, since connection state is connected")
		}

		if err != nil {
			t.Fatal(err)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // shouldn't receive any channel-state change event

	})

	t.Run("RTL4h: If channel is ATTACHING, listen to the attach event and don't send attach event", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}

		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)
		defer safeclose(t, close)

		close = c.To(
			connecting,
			connected,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		channelTransitioner.To(chAttaching)

		// check if attach message is sent
		checkIfAttachSent := recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); !attachSent {
			t.Fatalf("Should send attach message, since channel is attached")
		}

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		recorder.Reset() //reset the recorded messages to zero

		var rg ablytest.ResultGroup
		defer rg.Wait()
		rg.GoAdd(func(ctx context.Context) error {
			err := channel.Attach(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return err
		})

		// Check that the attach message isn't sent
		checkIfAttachSent = recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); attachSent {
			t.Fatalf("Attach message was sent")
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Shouldn't send attach, waiting for detach

		channelTransitioner.To(chAttached)

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // shouldn't receive any channel-state change event

	})

	t.Run("RTL4h: If channel is DETACHING, do attach after completion of request", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}

		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)
		defer safeclose(t, close)

		close = c.To(
			connecting,
			connected,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		channelTransitioner.To(chAttaching, chAttached, chDetaching)

		// check if attach message is sent
		checkIfAttachSent := recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); !attachSent {
			t.Fatalf("Should send attach message, since channel is attached")
		}

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		recorder.Reset() //reset the recorded messages to zero

		var rg ablytest.ResultGroup
		defer rg.Wait()
		rg.GoAdd(func(ctx context.Context) error {
			err := channel.Attach(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return err
		})

		// Check that the attach message isn't sent
		checkIfAttachSent = recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); attachSent {
			t.Fatalf("Attach message was sent before connection is established")
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Shouldn't send attach, waiting for detach

		channelTransitioner.To(chDetached)

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		// Check that the attach message is sent
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); !attachSent {
			t.Fatalf("Should send attach message, since channel is detached")
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Soon.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // shouldn't receive any channel-state change event

	})

	t.Run("RTL4i : If connection state is CONNECTING, do ATTACH after CONNECTED", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)
		defer safeclose(t, close)

		close = c.To(
			connecting,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		channel.Attach(ctx)

		// Check that the attach message isn't sent
		checkIfAttachSent := recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); attachSent {
			t.Fatalf("Attach message was sent before connection is established")
		}

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Shouldn't receive ATTACH, message queued

		close = c.To(
			connected,
		)

		defer safeclose(t, close)

		// Check that the attach message is sent
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); !attachSent {
			t.Fatalf("Should send attach message, since connected")
		}

		ablytest.Soon.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // shouldn't receive any channel-state change event
	})

	t.Run("RTL4i : If connection state is DISCONNECTED, do ATTACH after CONNECTED", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)
		defer safeclose(t, close)

		close = c.To(
			connecting,
			connected,
			disconnected,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		channel.Attach(ctx)

		// Check that the attach message isn't sent

		checkIfAttachSent := recorder.CheckIfSent(proto.ActionAttach, 1)
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); attachSent {
			t.Fatalf("Attach message was sent before connection is established")
		}

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Shouldn't receive ATTACH, message queued

		close = c.To(
			connecting,
			connected,
		)

		// Check that the attach message is sent
		if attachSent := ablytest.Instantly.IsTrue(checkIfAttachSent); !attachSent {
			t.Fatalf("Should send attach message, since connected")
		}

		defer safeclose(t, close)

		ablytest.Soon.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}
	})

	t.Run("RTL4j RTL13a: If channel attach is not a clean attach, should set ATTACH_RESUME in the ATTACH message", func(t *testing.T) {
		t.Parallel()
		in, out, _, channel, stateChanges, _ := setup(t)

		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel()
		channel.Attach(cancelledCtx)

		ablytest.Instantly.Recv(t, nil, out, t.Fatalf) // Consume ATTACHING
		channel.OnAll(stateChanges.Receive)

		// Get the channel to ATTACHED.

		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}

		var change ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		errInfo := proto.ErrorInfo{
			StatusCode: 500,
			Code:       50500,
			Message:    "fake error",
		}

		in <- &proto.ProtocolMessage{
			Action:  proto.ActionDetached,
			Channel: channel.Name,
			Error:   &errInfo,
		}

		// Expect a transition to ATTACHING with the error.

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		var msg *proto.ProtocolMessage
		ablytest.Instantly.Recv(t, &msg, out, t.Fatalf)
		if expected, got := proto.ActionAttach, msg.Action; expected != got {
			t.Fatalf("expected %v; got %v (message: %+v)", expected, got, msg)
		}

		if !msg.Flags.Has(proto.FlagAttachResume) {
			t.Fatalf("Attach message should have Flag Attach Resume set to true")
		}
	})

	t.Run("RTL4j1: AttachResume should be True when Attached (Clean ATTACH)", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}

		defer safeclose(t, app)

		c, close := TransitionConn(t, nil, app.Options()...)
		defer safeclose(t, close)

		close = c.To(
			connecting,
			connected,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		// channel state is initialized
		if expected, got := ably.ChannelStateInitialized, channel.State(); expected != got {
			t.Fatalf("expected %v; got %v", expected, got)
		}

		if channel.GetAttachResume() != false {
			t.Fatalf("Channel attach resume should be false when channel state is INITIALIZED")
		}

		channelTransitioner.To(chAttaching, chAttached)

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}
		if channel.GetAttachResume() != true {
			t.Fatalf("Channel attach resume should be true when channel state is ATTACHED")
		}

		channelTransitioner.To(chDetaching)

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		if channel.GetAttachResume() != false {
			t.Fatalf("Channel attach resume should be false when channel state is DETACHING")
		}

		channelTransitioner.To(chFailed)

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateFailed, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		if channel.GetAttachResume() != false {
			t.Fatalf("Channel attach resume should be false when channel state is FAILED")
		}
	})

	t.Run("RTL4j2: Rewind flag should allow to receive historic messages", func(t *testing.T) {
		t.Parallel()
		t.Skip("Flaky test, needs to be fixed, sometimes works well in debug mode")
		app, client := ablytest.NewRealtime(
			ably.WithAutoConnect(false))

		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

		client1 := app.NewRealtime(
			ably.WithAutoConnect(false))

		defer safeclose(t, ablytest.FullRealtimeCloser(client1))

		client2 := app.NewRealtime(
			ably.WithAutoConnect(false))

		defer safeclose(t, ablytest.FullRealtimeCloser(client2))

		ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
		ablytest.Wait(ablytest.ConnWaiter(client1, client1.Connect, ably.ConnectionEventConnected), nil)
		ablytest.Wait(ablytest.ConnWaiter(client2, client2.Connect, ably.ConnectionEventConnected), nil)

		channel := client.Channels.Get("persisted:test")
		err := channel.Publish(context.Background(), "test", "testData")
		if err != nil {
			t.Fatalf("error publishing message : %v", err)
		}
		channel1 := client1.Channels.Get("persisted:test",
			ably.ChannelWithRewind("1"))
		channel1.SetAttachResume(true)

		var chan1MsgCount uint64
		unsubscribe1, err := channel1.SubscribeAll(context.Background(), func(message *ably.Message) {
			atomic.AddUint64(&chan1MsgCount, 1)
		})

		defer unsubscribe1()

		if err != nil {
			t.Fatalf("error subscribing channel 1 : %v", err)
		}

		channel2 := client2.Channels.Get("persisted:test",
			ably.ChannelWithRewind("1")) // attach resume is false by default

		var chan2MsgCount uint64
		unsubscribe2, err := channel2.SubscribeAll(context.Background(), func(message *ably.Message) {
			atomic.AddUint64(&chan2MsgCount, 1)
		})

		defer unsubscribe2()

		if err != nil {
			t.Fatalf("error subcribing channel 2 : %v", err)
		}

		err = ablytest.Wait(ablytest.AssertionWaiter(func() bool {
			return atomic.LoadUint64(&chan2MsgCount) == 1
		}), nil)

		if err != nil {
			t.Fatalf("Channel 2 should receive 1 published message")
		}

		err = ablytest.Wait(ablytest.AssertionWaiter(func() bool {
			return atomic.LoadUint64(&chan1MsgCount) == 0
		}), nil)

		if err != nil {
			t.Fatalf("Channel 1 shouldn't receive any messages")
		}
	})

	t.Run("RTL4k: If params given channel options, should be sent in ATTACH message", func(t *testing.T) {
		t.Parallel()

		recorder := ablytest.NewMessageRecorder()
		app, client := ablytest.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithDial(recorder.Dial))

		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

		ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)

		channel := client.Channels.Get("test",
			ably.ChannelWithParams("test", "blah"),
			ably.ChannelWithParams("test2", "blahblah"),
			ably.ChannelWithParams("delta", "vcdiff"))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := channel.Attach(ctx)

		if err != nil {
			t.Fatal(err)
		}

		attachMessage := recorder.FindFirst(proto.ActionAttach)
		params := attachMessage.Params // RTL4k

		if params == nil {
			t.Fatal("Attach message params cannot be null")
		}

		if expected, got := "blah", params["test"]; expected != got {
			t.Fatalf("expected %v; got %v", expected, got)
		}

		if expected, got := "blahblah", params["test2"]; expected != got {
			t.Fatalf("expected %v; got %v", expected, got)
		}

		if expected, got := "vcdiff", params["delta"]; expected != got {
			t.Fatalf("expected %v; got %v", expected, got)
		}
	})

	t.Run("RTL4k1: If params given channel options, should be exposed as readonly field on ATTACHED message", func(t *testing.T) {
		t.Parallel()

		app, client := ablytest.NewRealtime(
			ably.WithAutoConnect(false))

		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

		ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)

		channel := client.Channels.Get("test",
			ably.ChannelWithParams("test", "blah"),
			ably.ChannelWithParams("test2", "blahblah"),
			ably.ChannelWithParams("delta", "vcdiff"))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		err := channel.Attach(ctx)

		if err != nil {
			t.Fatal(err)
		}

		ablytest.Soon.Recv(t, nil, channelStateChanges, t.Fatalf) // CONSUME ATTACHING
		ablytest.Soon.Recv(t, nil, channelStateChanges, t.Fatalf) // CONSUME ATTACHED

		err = ablytest.Wait(ablytest.AssertionWaiter(func() bool {
			return len(channel.Params()) > 0
		}), nil)

		if err != nil {
			t.Fatal("Should receive channel params")
		}

		params := channel.Params() // RTL4k1

		if expected, got := "vcdiff", params["delta"]; expected != got {
			t.Fatalf("expected %v; got %v", expected, got)
		}
	})

	t.Run("RTL4l: If modes provided in channelOptions, should be encoded as bitfield and set as flags field of ATTACH message", func(t *testing.T) {
		t.Parallel()
		recorder := ablytest.NewMessageRecorder()
		app, client := ablytest.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithDial(recorder.Dial))

		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
		ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)

		channelModes := []ably.ChannelMode{proto.ChannelModePresence, proto.ChannelModePublish, proto.ChannelModeSubscribe}

		channel := client.Channels.Get("test",
			ably.ChannelWithModes(channelModes...))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := channel.Attach(ctx)

		if err != nil {
			t.Fatal(err)
		}

		attachMessage := recorder.FindFirst(proto.ActionAttach)
		flags := attachMessage.Flags // RTL4k
		modes := proto.FromFlag(flags)

		if !reflect.DeepEqual(channelModes, modes) {
			t.Fatalf("expected %v; got %v", channelModes, modes)
		}
	})

	t.Run("RTL4m: If modes provides while attach, should receive modes in attached message", func(t *testing.T) {
		t.Parallel()
		app, client := ablytest.NewRealtime(
			ably.WithAutoConnect(false))

		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
		ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)

		channelModes := []ably.ChannelMode{proto.ChannelModePresence, proto.ChannelModePublish, proto.ChannelModeSubscribe}

		channel := client.Channels.Get("test",
			ably.ChannelWithModes(channelModes...))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		err := channel.Attach(ctx)

		if err != nil {
			t.Fatal(err)
		}

		ablytest.Soon.Recv(t, nil, channelStateChanges, t.Fatalf) // CONSUME ATTACHING
		ablytest.Soon.Recv(t, nil, channelStateChanges, t.Fatalf) // CONSUME ATTACHED

		modes := channel.Modes()

		if !reflect.DeepEqual(channelModes, modes) {
			t.Fatalf("expected %v; got %v", channelModes, modes)
		}
	})
}

func TestRealtimeChannel_RTL5_Detach(t *testing.T) {

	const channelRetryTimeout = 123 * time.Millisecond
	const realtimeRequestTimeout = 2 * time.Second

	setup := func(t *testing.T) (
		in, out chan *proto.ProtocolMessage,
		c *ably.Realtime,
		channel *ably.RealtimeChannel,
		stateChanges ably.ChannelStateChanges,
		afterCalls chan ablytest.AfterCall,
	) {
		in = make(chan *proto.ProtocolMessage, 1)
		out = make(chan *proto.ProtocolMessage, 16)
		afterCalls = make(chan ablytest.AfterCall, 1)
		now, after := ablytest.TimeFuncs(afterCalls)

		c, _ = ably.NewRealtime(
			ably.WithToken("fake:token"),
			ably.WithAutoConnect(false),
			ably.WithChannelRetryTimeout(channelRetryTimeout),
			ably.WithRealtimeRequestTimeout(realtimeRequestTimeout),
			ably.WithDial(ablytest.MessagePipe(in, out)),
			ably.WithNow(now),
			ably.WithAfter(after),
		)

		in <- &proto.ProtocolMessage{
			Action:            proto.ActionConnected,
			ConnectionID:      "connection-id",
			ConnectionDetails: &proto.ConnectionDetails{},
		}

		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		if err != nil {
			t.Fatal(err)
		}

		channel = c.Channels.Get("test")
		stateChanges = make(ably.ChannelStateChanges, 10)
		return
	}

	t.Run("RTL5a: If channel is INITIALIZED or DETACHED, do nothing", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)

		close = c.To(
			connecting,
			connected,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		channel.OnAll(channelStateChanges.Receive)

		// Initialized state
		if expected, got := ably.ChannelStateInitialized, channel.State(); expected != got {
			t.Fatalf("expected %v; got %v", expected, got)
		}

		err = channel.Detach(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Check that the detach message isn't sent
		checkIfDetachSent := recorder.CheckIfSent(proto.ActionDetach, 1)
		if detachSent := ablytest.Instantly.IsTrue(checkIfDetachSent); detachSent {
			t.Fatalf("Detach message was sent before connection is established")
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Should not make any change to the channel state

		channelTransitioner.To(chAttaching, chAttached, chDetaching, chDetached)

		// Check that the detach message sent
		checkIfDetachSent = recorder.CheckIfSent(proto.ActionDetach, 1)
		if detachSent := ablytest.Instantly.IsTrue(checkIfDetachSent); !detachSent {
			t.Fatalf("Detach message was not sent")
		}

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		err = channel.Detach(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Check that the detach message isn't sent
		checkIfDetachSent = recorder.CheckIfSent(proto.ActionDetach, 2)
		if detachSent := ablytest.Instantly.IsTrue(checkIfDetachSent); detachSent {
			t.Fatalf("Detach message was sent in channel detached state")
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Should not make any change to the channel state
	})

	t.Run("RTL5b: If channel state is FAILED, return error", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)

		close = c.To(
			connecting,
			connected,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		channelTransitioner.To(chAttaching, chFailed)

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateFailed, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err = channel.Detach(ctx)

		// Check that the detach message isn't sent
		checkIfDetachSent := recorder.CheckIfSent(proto.ActionDetach, 1)
		if detachSent := ablytest.Instantly.IsTrue(checkIfDetachSent); detachSent {
			t.Fatalf("Detach message was sent in channel detached state")
		}

		if expected, got := "cannot Detach channel because it is in FAILED state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}
		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Should not make any change to the channel state

	})

	t.Run("RTL5d RTL5e: If connected, should do successful detach with server", func(t *testing.T) {
		t.Parallel()

		recorder := ablytest.NewMessageRecorder()
		app, client := ablytest.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithDial(recorder.Dial),
		)

		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

		connectionStateChanges := make(ably.ConnStateChanges, 10)
		off := client.Connection.OnAll(connectionStateChanges.Receive)
		defer off()

		client.Connect()

		var connectionChange ably.ConnectionStateChange

		ablytest.Soon.Recv(t, &connectionChange, connectionStateChanges, t.Fatalf)
		if expected, got := ably.ConnectionStateConnecting, connectionChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, connectionChange)
		}

		ablytest.Soon.Recv(t, &connectionChange, connectionStateChanges, t.Fatalf)
		if expected, got := ably.ConnectionStateConnected, connectionChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, connectionChange)
		}

		channel := client.Channels.Get("test")
		channelStateChanges := make(ably.ChannelStateChanges, 10)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		channeloff := channel.OnAll(channelStateChanges.Receive)
		defer channeloff()

		var channelStatechange ably.ChannelStateChange

		err := channel.Attach(ctx)
		if err != nil {
			t.Fatal(err)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		err = channel.Detach(ctx)

		// Check that the detach message sent
		checkIfDetachSent := recorder.CheckIfSent(proto.ActionDetach, 1)
		if detachSent := ablytest.Instantly.IsTrue(checkIfDetachSent); !detachSent {
			t.Fatalf("Detach message was not sent")
		}

		if err != nil {
			t.Fatal(err)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.NoRecv(t, nil, connectionStateChanges, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf)
	})

	t.Run("RTL5e: return error if channel detach fails", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)

		close = c.To(
			connecting,
			connected,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		channelTransitioner.To(chAttaching, chAttached, chDetaching, chFailed)

		// Check that the detach message sent
		checkIfDetachSent := recorder.CheckIfSent(proto.ActionDetach, 1)
		if detachSent := ablytest.Instantly.IsTrue(checkIfDetachSent); !detachSent {
			t.Fatalf("Detach message was not sent")
		}

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateFailed, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		detachErr := <-channelTransitioner.err[chDetaching]

		if detachErr == nil {
			t.Fatal("detach should return channel failed error")
		}

		if ably.UnwrapErrorCode(detachErr) != 50001 {
			t.Fatalf("want code=50001; got %d", ably.UnwrapErrorCode(err))
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Should not make any change to the channel state
	})

	t.Run("RTL5f: return error on request timeout", func(t *testing.T) {
		t.Parallel()

		in, out, _, channel, stateChanges, afterCalls := setup(t)
		channel.OnAll(stateChanges.Receive)

		// Get the channel to ATTACHED.
		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}

		var change ably.ChannelStateChange
		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		var outMsg *proto.ProtocolMessage

		errCh := asyncDetach(channel)

		// Cause a timeout.
		var afterCall ablytest.AfterCall
		ablytest.Instantly.Recv(t, &afterCall, afterCalls, t.Fatalf)
		afterCall.Fire()

		var err error
		ablytest.Instantly.Recv(t, &err, errCh, t.Fatalf)
		if err == nil {
			t.Fatal("detach should return timeout error")
		}

		if ably.UnwrapErrorCode(err) != 50003 {
			t.Fatalf("want code=50003; got %d", ably.UnwrapErrorCode(err))
		}

		ablytest.Instantly.Recv(t, &outMsg, out, t.Fatalf)
		if expected, got := proto.ActionDetach, outMsg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, outMsg.Action)
		}

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetaching, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		// setting channelstate to prevState, since channel detach failed
		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		if expected, got := err, change.Reason; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		ablytest.Instantly.NoRecv(t, &change, stateChanges, t.Fatalf)
	})

	t.Run("RTL5g: If connection state CLOSING or FAILED, should return error", func(t *testing.T) {

		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}

		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)

		close = c.To(
			connecting,
			connected,
		)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		channelTransitioner.To(chAttaching, chAttached)

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		close = c.To(
			closing,
		)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err = channel.Detach(ctx)

		// Check that the detach message isn't sent
		checkIfDetachSent := recorder.CheckIfSent(proto.ActionDetach, 1)
		if detachSent := ablytest.Instantly.IsTrue(checkIfDetachSent); detachSent {
			t.Fatalf("Detach message was sent before connection is established")
		}

		if expected, got := "cannot Detach channel because connection is in CLOSING state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}

		close = c.To(
			failed,
		)

		defer safeclose(t, close)

		err = channel.Detach(ctx)

		// Check that the detach message isn't sent
		if detachSent := ablytest.Instantly.IsTrue(checkIfDetachSent); detachSent {
			t.Fatalf("Detach message was sent before connection is established")
		}

		if expected, got := "cannot Detach channel because connection is in FAILED state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}

	})

	t.Run("RTL5h : If Connection state CONNECTING, queue the DETACH message and send on CONNECTED", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)

		close = c.To(
			connecting,
			connected,
		)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		channelTransitioner.To(chAttaching, chAttached)

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		close = c.To(
			disconnected,
			connecting,
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		channel.Detach(ctx)

		// Check that the detach message isn't sent
		checkIfDetachSent := recorder.CheckIfSent(proto.ActionDetach, 1)
		if detachSent := ablytest.Instantly.IsTrue(checkIfDetachSent); detachSent {
			t.Fatalf("Detach message was sent before connection is established")
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Shouldn't receive detach, detaching message queued

		close = c.To(
			connected,
		)

		defer safeclose(t, close)

		// Check that the detach message sent
		if detachSent := ablytest.Instantly.IsTrue(checkIfDetachSent); !detachSent {
			t.Fatalf("Detach message was not sent")
		}

		ablytest.Soon.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

	})

	t.Run("RTL5h : If Connection state DISCONNECTED, queue the DETACH message and send on CONNECTED", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}
		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)

		close = c.To(
			connecting,
			connected,
		)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		channelTransitioner.To(chAttaching, chAttached)

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		close = c.To(
			disconnected,
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		channel.Detach(ctx)

		// Check that the detach message isn't sent
		checkIfDetachSent := recorder.CheckIfSent(proto.ActionDetach, 1)
		if detachSent := ablytest.Instantly.IsTrue(checkIfDetachSent); detachSent {
			t.Fatalf("Detach message was sent before connection is established")
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Shouldn't receive detach, detaching message queued

		close = c.To(
			connecting,
			connected,
		)

		defer safeclose(t, close)

		// Check that the detach message sent
		if detachSent := ablytest.Instantly.IsTrue(checkIfDetachSent); !detachSent {
			t.Fatalf("Detach message was not sent")
		}

		ablytest.Soon.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}
	})

	t.Run("RTL5i: If channel in DETACHING or ATTACHING state, do detach after completion of operation", func(t *testing.T) {
		t.Parallel()

		app, err := ablytest.NewSandbox(nil)
		if err != nil {
			t.Fatal(err)
		}

		defer safeclose(t, app)

		recorder := ablytest.NewMessageRecorder()
		c, close := TransitionConn(t, recorder.Dial, app.Options()...)

		close = c.To(
			connecting,
			connected,
		)

		defer safeclose(t, close)

		channelTransitioner := c.Channel("test")
		channel := channelTransitioner.Channel

		channelStateChanges := make(ably.ChannelStateChanges, 10)
		channel.OnAll(channelStateChanges.Receive)

		channelTransitioner.To(chAttaching)

		var channelStatechange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		var rg ablytest.ResultGroup
		defer rg.Wait()
		rg.GoAdd(func(ctx context.Context) error {
			err := channel.Detach(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return err
		})

		// Check that the detach message isn't sent
		checkIfDetachSent := recorder.CheckIfSent(proto.ActionDetach, 1)
		if detachSent := ablytest.Instantly.IsTrue(checkIfDetachSent); detachSent {
			t.Fatalf("Detach message was sent before connection is established")
		}

		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf) // Shouldn't send detach waiting to get attached

		channelTransitioner.To(chAttached)

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		// Check that the detach message sent
		if detachSent := ablytest.Instantly.IsTrue(checkIfDetachSent); !detachSent {
			t.Fatalf("Detach message was not sent")
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}

		ablytest.Soon.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelStatechange)
		}
	})

	t.Run("RTL5j: if channel state is SUSPENDED, immediately transition to DETACHED state", func(t *testing.T) {
		t.Parallel()
		t.Skip("Channel SUSPENDED not implemented yet")
		_, _, _, channel, stateChanges, _ := setup(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		channel.OnAll(stateChanges.Receive)

		//channel.SetState(ably.ChannelStateSuspended, nil)
		ablytest.Instantly.Recv(t, nil, stateChanges, t.Fatalf) // State will be changed to suspended

		channel.Detach(ctx)

		var change ably.ChannelStateChange
		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		ablytest.Instantly.NoRecv(t, &change, stateChanges, t.Fatalf)
	})

	t.Run("RTL5k: When receive ATTACH in detaching state, send new DETACH message", func(t *testing.T) {
		t.Parallel()

		in, out, _, channel, stateChanges, _ := setup(t)
		channel.OnAll(stateChanges.Receive)

		var outMsg *proto.ProtocolMessage
		var change ably.ChannelStateChange

		cancelledContext, cancel := context.WithCancel(context.Background())
		cancel()

		channel.Attach(cancelledContext)

		// get channel state to attaching
		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		ablytest.Instantly.Recv(t, &outMsg, out, t.Fatalf)
		if expected, got := proto.ActionAttach, outMsg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, outMsg.Action)
		}

		// Send attach message
		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		channel.Detach(cancelledContext)

		ablytest.Instantly.Recv(t, &outMsg, out, t.Fatalf)
		if expected, got := proto.ActionDetach, outMsg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, outMsg.Action)
		}

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetaching, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		// Send attach message in detaching state
		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}

		// sends detach message instead of attaching the channel
		ablytest.Instantly.Recv(t, &outMsg, out, t.Fatalf)
		if expected, got := proto.ActionDetach, outMsg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, outMsg.Action)
		}

		ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)
	})
}

func TestRealtimeChannel_RTL6c1_PublishNow(t *testing.T) {
	t.Parallel()

	var transition []ably.ChannelState
	for _, state := range []ably.ChannelState{
		ably.ChannelStateInitialized,
		ably.ChannelStateAttaching,
		ably.ChannelStateAttached,
		ably.ChannelStateDetaching,
		ably.ChannelStateDetached,
	} {
		transition = append(transition, state)
		transition := transition // Don't share between test goroutines.
		t.Run(fmt.Sprintf("when %s", state), func(t *testing.T) {
			t.Parallel()

			app, err := ablytest.NewSandbox(nil)
			if err != nil {
				t.Fatal(err)
			}
			defer safeclose(t, app)

			c, close := TransitionConn(t, nil, app.Options()...)
			defer safeclose(t, close)

			close = c.To(
				ably.ConnectionStateConnecting,
				ably.ConnectionStateConnected,
			)
			defer safeclose(t, close)

			chanTransitioner := c.Channel("test")
			channel, close := chanTransitioner.To(transition...)
			defer safeclose(t, close)

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			// Make a second client to subscribe and check that messages are
			// published without interferring with the first client's state.

			subClient, err := ably.NewRealtime(app.Options()...)
			if err != nil {
				t.Fatal(err)
			}
			defer safeclose(t, ablytest.FullRealtimeCloser(subClient))
			err = ablytest.Wait(ablytest.ConnWaiter(subClient, subClient.Connect, ably.ConnectionEventConnected), nil)

			msg := make(messages, 1)

			_, err = subClient.Channels.Get("test").SubscribeAll(context.Background(), msg.Receive)
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}

			err = channel.Publish(ctx, "test", nil)
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}

			ablytest.Soon.Recv(t, nil, msg, t.Fatalf)
		})
	}
}

func TestRealtimeChannel_RTL6c2_PublishEnqueue(t *testing.T) {
	type transitionsCase struct {
		connBefore []ably.ConnectionState
		channel    []ably.ChannelState
		connAfter  []ably.ConnectionState
	}

	var cases []transitionsCase

	// When connection is INITIALIZED, channel can only be INITIALIZED.

	for _, connBefore := range [][]ably.ConnectionState{
		{initialized},
	} {
		for _, channel := range [][]ably.ChannelState{
			{chInitialized},
		} {
			cases = append(cases, transitionsCase{
				connBefore: connBefore,
				channel:    channel,
			})
		}
	}

	// When connection is first CONNECTING, channel can only be INITIALIZED or
	// ATTACHING.

	for _, connBefore := range [][]ably.ConnectionState{
		{connecting},
		{connecting, disconnected},
	} {
		for _, channel := range [][]ably.ChannelState{
			{chInitialized},
			{chAttaching},
		} {
			cases = append(cases, transitionsCase{
				connBefore: connBefore,
				channel:    channel,
			})
		}
	}

	// For a channel to be ATTACHED, DETACHING or DETACHED, we must have had a
	// connection in the past.

	for _, connAfter := range [][]ably.ConnectionState{
		{disconnected},
		{disconnected, connecting},
	} {
		for _, channelAfter := range [][]ably.ChannelState{
			{},
			{chDetaching},
			{chDetaching, chDetached},
		} {
			cases = append(cases, transitionsCase{
				connBefore: []ably.ConnectionState{connecting, connected},
				channel: append([]ably.ChannelState{
					chAttaching,
					chAttached,
				}, channelAfter...),
				connAfter: connAfter,
			})
		}
	}

	for _, trans := range cases {
		trans := trans
		connTarget := trans.connBefore[len(trans.connBefore)-1]
		if len(trans.connAfter) > 0 {
			connTarget = trans.connAfter[len(trans.connAfter)-1]
		}
		chanTarget := trans.channel[len(trans.channel)-1]

		t.Run(fmt.Sprintf("when connection is %v, channel is %v", connTarget, chanTarget), func(t *testing.T) {
			t.Parallel()

			app, err := ablytest.NewSandbox(nil)
			if err != nil {
				t.Fatal(err)
			}
			defer safeclose(t, app)

			recorder := ablytest.NewMessageRecorder()

			c, close := TransitionConn(t, recorder.Dial, app.Options()...)
			defer safeclose(t, close)

			close = c.To(trans.connBefore...)
			defer safeclose(t, close)

			chanTransitioner := c.Channel("test")
			channel, close := chanTransitioner.To(trans.channel...)
			defer safeclose(t, close)

			close = c.To(trans.connAfter...)
			defer safeclose(t, close)

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err = channel.Publish(ctx, "test", nil)
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}

			// Check that the message isn't published.

			published := recorder.CheckIfSent(proto.ActionMessage, 1)

			if published := ablytest.Instantly.IsTrue(published); published {
				t.Fatalf("message was published before connection is established")
			}

			// After moving to CONNECTED, check that message is finally published.

			close = c.To(connecting, connected)
			defer safeclose(t, close)

			if published := ablytest.Soon.IsTrue(published); !published {
				t.Fatalf("message wasn't published once connection is established")
			}
		})
	}
}

func TestRealtimeChannel_RTL6c4_PublishFail(t *testing.T) {
	type transitionsCase struct {
		connBefore []ably.ConnectionState
		channel    []ably.ChannelState
		connAfter  []ably.ConnectionState
	}

	var cases []transitionsCase

	// FAILED and SUSPENDED with no connection ever made.

	for _, connBefore := range [][]ably.ConnectionState{
		// {connecting, failed},
		{connecting, disconnected, suspended},
	} {
		cases = append(cases, transitionsCase{
			connBefore: connBefore,
			channel:    []ably.ChannelState{chInitialized},
		})
	}

	// // FAILED and SUSPENDED after successful connection and attach.

	// for _, connAfter := range [][]ably.ConnectionState{
	// 	{disconnected, suspended},
	// } {
	// 	cases = append(cases, transitionsCase{
	// 		connBefore: []ably.ConnectionState{connecting, connected},
	// 		channel: []ably.ChannelState{
	// 			chAttaching,
	// 			chAttached,
	// 		},
	// 		connAfter: connAfter,
	// 	})
	// }

	// // Connection is OK but channel fails.
	// cases = append(cases, transitionsCase{
	// 	connBefore: []ably.ConnectionState{connecting, connected},
	// 	channel: []ably.ChannelState{
	// 		chAttaching,
	// 		chFailed,
	// 	},
	// })

	for _, trans := range cases {
		trans := trans
		connTarget := trans.connBefore[len(trans.connBefore)-1]
		if len(trans.connAfter) > 0 {
			connTarget = trans.connAfter[len(trans.connAfter)-1]
		}
		chanTarget := trans.channel[len(trans.channel)-1]

		t.Run(fmt.Sprintf("when connection is %v, channel is %v", connTarget, chanTarget), func(t *testing.T) {
			t.Parallel()

			app, err := ablytest.NewSandbox(nil)
			if err != nil {
				t.Fatal(err)
			}
			defer safeclose(t, app)

			recorder := ablytest.NewMessageRecorder()

			c, close := TransitionConn(t, recorder.Dial, app.Options()...)
			defer safeclose(t, close)

			close = c.To(trans.connBefore...)
			defer safeclose(t, close)

			chanTransitioner := c.Channel("test")
			channel, close := chanTransitioner.To(trans.channel...)
			defer safeclose(t, close)

			close = c.To(trans.connAfter...)
			defer safeclose(t, close)

			publishErr := asyncPublish(channel)

			// Check that the message isn't published.

			published := recorder.CheckIfSent(proto.ActionMessage, 1)

			if published := ablytest.Instantly.IsTrue(published); published {
				t.Fatalf("message was published when it shouldn't")
			}

			if err := <-publishErr; err == nil || errors.Is(err, context.Canceled) {
				t.Fatalf("expected publish error")
			}
		})
	}
}

func TestRealtimeChannel_RTL6c5_NoImplicitAttach(t *testing.T) {
	t.Parallel()

	app, c := ablytest.NewRealtime()
	defer safeclose(t, ablytest.FullRealtimeCloser(c), app)

	if err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatal(err)
	}

	channel := c.Channels.Get("test")
	err := channel.Publish(context.Background(), "test", nil)
	if err != nil {
		t.Fatal(err)
	}

	if channel.State() == chAttached {
		t.Fatal("channel implicitly attached")
	}
}

func TestRealtimeChannel_RTL2f_RTL12_HandleResume(t *testing.T) {
	t.Parallel()
	const channelRetryTimeout = 123 * time.Millisecond

	setup := func(t *testing.T) (
		in, out chan *proto.ProtocolMessage,
		c *ably.Realtime,
		channel *ably.RealtimeChannel,
		stateChanges ably.ChannelStateChanges,
		afterCalls chan ablytest.AfterCall,
	) {
		in = make(chan *proto.ProtocolMessage, 1)
		out = make(chan *proto.ProtocolMessage, 16)
		afterCalls = make(chan ablytest.AfterCall, 1)
		now, after := ablytest.TimeFuncs(afterCalls)

		c, _ = ably.NewRealtime(
			ably.WithToken("fake:token"),
			ably.WithAutoConnect(false),
			ably.WithNow(now),
			ably.WithAfter(after),
			ably.WithChannelRetryTimeout(channelRetryTimeout),
			ably.WithDial(ablytest.MessagePipe(in, out)),
		)

		in <- &proto.ProtocolMessage{
			Action:            proto.ActionConnected,
			ConnectionID:      "connection-id",
			ConnectionDetails: &proto.ConnectionDetails{},
		}

		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		if err != nil {
			t.Fatal(err)
		}

		channel = c.Channels.Get("test")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go channel.Attach(ctx)

		ablytest.Instantly.Recv(t, nil, afterCalls, t.Fatalf) // Consume expiry timer for attach
		ablytest.Instantly.Recv(t, nil, out, t.Fatalf)        // Consume ATTACHING
		stateChanges = make(ably.ChannelStateChanges, 10)
		channel.OnAll(stateChanges.Receive)

		return
	}

	flags := make(map[proto.Flag]string)
	flags[proto.FlagHasPresence] = "flag has_presence is provided"
	flags[proto.FlagHasBacklog] = "flag has_backlog is provided"
	flags[proto.FlagResumed] = "flag resumed is provided"

	for flag, flagDescription := range flags {
		t.Run(fmt.Sprintf("RTL2f: when %v, set channelChangeState resume to %v", flagDescription, flag == proto.FlagResumed), func(t *testing.T) {
			t.Parallel()
			isResume := flag == proto.FlagResumed
			in, _, _, channel, stateChanges, afterCalls := setup(t)
			// Get the channel to ATTACHED.

			in <- &proto.ProtocolMessage{
				Action:  proto.ActionAttached,
				Channel: channel.Name,
				Flags:   flag,
			}

			var change ably.ChannelStateChange

			ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
			if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
				t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
			}

			if change.Resumed != isResume {
				t.Fatalf("expected resumed to be %v (event: %+v)", isResume, change)
			}

			// Expect the retry loop to be finished.
			ablytest.Instantly.NoRecv(t, nil, afterCalls, t.Fatalf)
			ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)
		})
	}
	t.Run("RTL12: when RE-ATTACH with error, set ChannelEventUpdated", func(t *testing.T) {
		t.Parallel()
		in, _, _, channel, stateChanges, afterCalls := setup(t)

		// Get the channel to ATTACHED.
		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
			Flags:   proto.FlagResumed,
		}

		var change ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		// Re-attach the channel
		errInfo := proto.ErrorInfo{
			StatusCode: 500,
			Code:       50500,
			Message:    "fake error",
		}

		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
			Flags:   0,
			Error:   &errInfo,
		}

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelEventUpdate, change.Event; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		if expected, got := ably.ChannelStateAttached, change.Previous; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		if change.Resumed {
			t.Fatalf("expected resume to be false")
		}
		if got := fmt.Sprint(change.Reason); !strings.Contains(got, errInfo.Message) {
			t.Fatalf("expected %+v; got %v (error: %+v)", errInfo, got, change.Reason)
		}
		// Expect the retry loop to be finished.
		ablytest.Instantly.NoRecv(t, nil, afterCalls, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)
	})
}

func TestRealtimeChannel_RTL13_HandleDetached(t *testing.T) {
	t.Parallel()

	const channelRetryTimeout = 123 * time.Millisecond

	setup := func(t *testing.T) (
		in, out chan *proto.ProtocolMessage,
		c *ably.Realtime,
		channel *ably.RealtimeChannel,
		stateChanges ably.ChannelStateChanges,
		afterCalls chan ablytest.AfterCall,
	) {
		in = make(chan *proto.ProtocolMessage, 1)
		out = make(chan *proto.ProtocolMessage, 16)
		afterCalls = make(chan ablytest.AfterCall, 1)
		now, after := ablytest.TimeFuncs(afterCalls)

		c, _ = ably.NewRealtime(
			ably.WithToken("fake:token"),
			ably.WithAutoConnect(false),
			ably.WithNow(now),
			ably.WithAfter(after),
			ably.WithChannelRetryTimeout(channelRetryTimeout),
			ably.WithDial(ablytest.MessagePipe(in, out)),
		)

		in <- &proto.ProtocolMessage{
			Action:            proto.ActionConnected,
			ConnectionID:      "connection-id",
			ConnectionDetails: &proto.ConnectionDetails{},
		}

		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		if err != nil {
			t.Fatal(err)
		}

		channel = c.Channels.Get("test")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go channel.Attach(ctx)

		ablytest.Soon.Recv(t, nil, afterCalls, t.Fatalf) // consume TIMER
		ablytest.Instantly.Recv(t, nil, out, t.Fatalf)   // Consume ATTACHING
		stateChanges = make(ably.ChannelStateChanges, 10)
		channel.OnAll(stateChanges.Receive)
		return
	}

	t.Run("RTL13a: when ATTACHED, successful reattach", func(t *testing.T) {
		t.Parallel()

		in, out, _, channel, stateChanges, afterCalls := setup(t)

		// Get the channel to ATTACHED.

		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}

		var change ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		errInfo := proto.ErrorInfo{
			StatusCode: 500,
			Code:       50500,
			Message:    "fake error",
		}

		in <- &proto.ProtocolMessage{
			Action:  proto.ActionDetached,
			Channel: channel.Name,
			Error:   &errInfo,
		}

		// Expect a transition to ATTACHING with the error.

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		if got := fmt.Sprint(change.Reason); !strings.Contains(got, errInfo.Message) {
			t.Fatalf("expected %+v; got %v (error: %+v)", errInfo, got, change.Reason)
		}

		var msg *proto.ProtocolMessage
		ablytest.Instantly.Recv(t, &msg, out, t.Fatalf)
		if expected, got := proto.ActionAttach, msg.Action; expected != got {
			t.Fatalf("expected %v; got %v (message: %+v)", expected, got, msg)
		}

		// TODO: Test attach failure too, which requires RTL4e.

		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}
		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		if change.Reason != nil {
			t.Fatal(change.Reason)
		}

		// Expect the retry loop to be finished.
		ablytest.Instantly.NoRecv(t, nil, afterCalls, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)
	})

	t.Run("RTL13b: when ATTACHING", func(t *testing.T) {
		t.Parallel()

		in, out, _, channel, stateChanges, afterCalls := setup(t)

		errInfo := proto.ErrorInfo{
			StatusCode: 500,
			Code:       50500,
			Message:    "fake error",
		}

		in <- &proto.ProtocolMessage{
			Action:  proto.ActionDetached,
			Channel: channel.Name,
			Error:   &errInfo,
		}

		// Expect a state change with the error.

		var change ably.ChannelStateChange
		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		if got := fmt.Sprint(change.Reason); !strings.Contains(got, errInfo.Message) {
			t.Fatalf("expected %+v; got %v (error: %+v)", errInfo, got, change.Reason)
		}

		// Expect an attempt to attach after channelRetryTimeout.

		var call ablytest.AfterCall
		ablytest.Instantly.Recv(t, &call, afterCalls, t.Fatalf)
		if expected, got := channelRetryTimeout, call.D; expected != got {
			t.Fatalf("expected %v; got %v", expected, got)
		}
		call.Fire()

		// Expect a transition to ATTACHING, and an ATTACH message.

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttaching, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		var msg *proto.ProtocolMessage
		ablytest.Instantly.Recv(t, &msg, out, t.Fatalf)
		if expected, got := proto.ActionAttach, msg.Action; expected != got {
			t.Fatalf("expected %v; got %v (message: %+v)", expected, got, msg)
		}

		// TODO: Test attach failure too, which requires RTL4f.

		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}
		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		if change.Reason != nil {
			t.Fatal(change.Reason)
		}

		// Expect the retry loop to be finished.
		ablytest.Instantly.NoRecv(t, nil, afterCalls, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)
	})

	t.Run("RTL13c: stop on non-CONNECTED", func(t *testing.T) {
		t.Parallel()

		in, out, c, channel, stateChanges, afterCalls := setup(t)

		errInfo := proto.ErrorInfo{
			StatusCode: 500,
			Code:       50500,
			Message:    "fake error",
		}

		in <- &proto.ProtocolMessage{
			Action:  proto.ActionDetached,
			Channel: channel.Name,
			Error:   &errInfo,
		}

		// Expect a state change with the error.

		var change ably.ChannelStateChange
		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		if got := fmt.Sprint(change.Reason); !strings.Contains(got, errInfo.Message) {
			t.Fatalf("expected %+v; got %v (error: %+v)", errInfo, got, change.Reason)
		}

		// Expect an attempt to attach after channelRetryTimeout.

		var call ablytest.AfterCall
		ablytest.Instantly.Recv(t, &call, afterCalls, t.Fatalf)
		if expected, got := channelRetryTimeout, call.D; expected != got {
			t.Fatalf("expected %v; got %v", expected, got)
		}

		// Get the connection to a non-CONNECTED state by closing in.

		err := ablytest.Wait(ablytest.ConnWaiter(c, func() {
			close(in)
		}, ably.ConnectionEventDisconnected), nil)
		if !errors.Is(err, io.EOF) {
			t.Fatal(err)
		}

		// Now trigger the channelRetryTimeout.

		call.Fire()

		// Since the connection isn't CONNECTED, the retry loop should finish.

		ablytest.Instantly.NoRecv(t, nil, afterCalls, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)
	})
}

func TestRealtimeChannel_RTL17_IgnoreMessagesWhenNotAttached(t *testing.T) {
	t.Parallel()

	const channelRetryTimeout = 123 * time.Millisecond

	setup := func(t *testing.T) (
		in, out chan *proto.ProtocolMessage,
		msg chan *proto.Message,
		c *ably.Realtime,
		channel *ably.RealtimeChannel,
		stateChanges ably.ChannelStateChanges,
	) {
		in = make(chan *proto.ProtocolMessage, 1)
		out = make(chan *proto.ProtocolMessage, 16)
		msg = make(chan *proto.Message, 1)
		afterCalls := make(chan ablytest.AfterCall, 1)
		now, after := ablytest.TimeFuncs(afterCalls)

		c, _ = ably.NewRealtime(
			ably.WithToken("fake:token"),
			ably.WithAutoConnect(false),
			ably.WithNow(now),
			ably.WithAfter(after),
			ably.WithChannelRetryTimeout(channelRetryTimeout),
			ably.WithDial(ablytest.MessagePipe(in, out)),
		)

		in <- &proto.ProtocolMessage{
			Action:            proto.ActionConnected,
			ConnectionID:      "connection-id",
			ConnectionDetails: &proto.ConnectionDetails{},
		}

		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		if err != nil {
			t.Fatal(err)
		}

		channel = c.Channels.Get("test")

		stateChanges = make(ably.ChannelStateChanges, 10)
		channel.OnAll(stateChanges.Receive)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		channel.SubscribeAll(ctx, func(message *ably.Message) {
			msg <- message
		})

		channel.Attach(ctx)
		return
	}

	t.Run("Shouldn't receive message when not attached", func(t *testing.T) {
		t.Parallel()

		in, out, msg, _, channel, stateChanges := setup(t)

		receiveMessage := func() {
			message := &ably.Message{
				ID:           "Id",
				ClientID:     "clientId",
				ConnectionID: "connectionId",
				Name:         "Sample Name",
				Data:         "Sample Data",
				Encoding:     "encoding",
				Timestamp:    0,
				Extras:       nil,
			}
			in <- &proto.ProtocolMessage{
				Action:        proto.ActionMessage,
				Channel:       channel.Name,
				ID:            "uniqueId",
				MsgSerial:     3,
				ChannelSerial: "channelSerial",
				Messages:      append(make([]*ably.Message, 0), message),
			}
		}

		var change ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf) // Consume ATTACHING
		if expected, got := ably.ChannelStateAttaching, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		// Shouldn't receive message when state is ATTACHING
		receiveMessage()
		ablytest.Instantly.NoRecv(t, nil, msg, t.Fatalf)

		// Get the channel to ATTACHED.
		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf) // Consume ATTACHED
		if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		channel.SubscribeAll(context.Background(), func(message *ably.Message) {
			msg <- message
		})

		// receive message when state is ATTACHED
		receiveMessage()
		ablytest.Instantly.Recv(t, nil, msg, t.Fatalf)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		channel.Detach(ctx)
		// Get the channel to DETACHED.

		ablytest.Instantly.Recv(t, nil, out, t.Fatalf) // Consume DETACHING

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf) // DETACHING channel state
		if expected, got := ably.ChannelStateDetaching, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		in <- &proto.ProtocolMessage{
			Action:  proto.ActionDetached,
			Channel: channel.Name,
		}

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf) // Consume DETACHED
		if expected, got := ably.ChannelStateDetached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		// Shouldn't receive message when state is DETACHED
		receiveMessage()
		ablytest.Instantly.NoRecv(t, nil, msg, t.Fatalf)

	})
}

func TestRealtimeChannel_RTL14_HandleChannelError(t *testing.T) {
	t.Parallel()

	const channelRetryTimeout = 123 * time.Millisecond

	setup := func(t *testing.T) (
		in, out chan *proto.ProtocolMessage,
		c *ably.Realtime,
		channel *ably.RealtimeChannel,
		stateChanges ably.ChannelStateChanges,
		afterCalls chan ablytest.AfterCall,
	) {
		in = make(chan *proto.ProtocolMessage, 1)
		out = make(chan *proto.ProtocolMessage, 16)
		afterCalls = make(chan ablytest.AfterCall, 1)
		now, after := ablytest.TimeFuncs(afterCalls)

		c, _ = ably.NewRealtime(
			ably.WithToken("fake:token"),
			ably.WithAutoConnect(false),
			ably.WithNow(now),
			ably.WithAfter(after),
			ably.WithChannelRetryTimeout(channelRetryTimeout),
			ably.WithDial(ablytest.MessagePipe(in, out)),
		)

		in <- &proto.ProtocolMessage{
			Action:            proto.ActionConnected,
			ConnectionID:      "connection-id",
			ConnectionDetails: &proto.ConnectionDetails{},
		}

		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		if err != nil {
			t.Fatal(err)
		}

		channel = c.Channels.Get("test")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go channel.Attach(ctx)

		ablytest.Instantly.Recv(t, nil, afterCalls, t.Fatalf) // Consume timer
		ablytest.Instantly.Recv(t, nil, out, t.Fatalf)        // Consume ATTACHING

		stateChanges = make(ably.ChannelStateChanges, 10)
		channel.OnAll(stateChanges.Receive)

		return
	}

	t.Run("RTL14: when Error, should transition to failed state", func(t *testing.T) {
		t.Parallel()
		in, out, _, channel, stateChanges, afterCalls := setup(t)

		errInfo := proto.ErrorInfo{
			StatusCode: 500,
			Code:       50500,
			Message:    "fake error",
		}

		in <- &proto.ProtocolMessage{
			Action:  proto.ActionError,
			Channel: channel.Name,
			Error:   &errInfo,
		}

		// Expect a state change with the error.

		var change ably.ChannelStateChange
		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateFailed, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		if got := fmt.Sprint(change.Reason); !strings.Contains(got, errInfo.Message) {
			t.Fatalf("expected %+v; got %v (error: %+v)", errInfo, got, change.Reason)
		}

		if got := fmt.Sprint(channel.ErrorReason()); !strings.Contains(got, errInfo.Message) {
			t.Fatalf("expected %+v; got %v (error: %+v)", errInfo, got, change.Reason)
		}

		ablytest.Instantly.NoRecv(t, nil, afterCalls, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)
	})
}
