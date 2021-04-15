package ably_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
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

		_, out, _, channel, stateChanges, _ := setup(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		channel.OnAll(stateChanges.Receive)

		// Initialized state
		channel.Detach(ctx)
		ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)          // Should not send detach message
		ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf) // Should not make any change to the state

		channel.SetState(ably.ChannelStateDetached, nil)
		ablytest.Instantly.Recv(t, nil, stateChanges, t.Fatalf) // State will be changed to detached

		// Detached state
		channel.Detach(ctx)
		ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)          // Should not send detach message
		ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf) // Should not make any change to the state
	})

	t.Run("RTL5b: If channel state is FAILED, return error", func(t *testing.T) {
		t.Parallel()

		_, _, _, channel, stateChanges, _ := setup(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		channel.OnAll(stateChanges.Receive)

		channel.SetState(ably.ChannelStateFailed, nil)
		ablytest.Instantly.Recv(t, nil, stateChanges, t.Fatalf) // State will be changed to failed
		err := channel.Detach(ctx)

		if expected, got := "cannot Detach channel because it is in FAILED state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}
	})

	t.Run("RTL5d RTL5e: If connected, should do successful detach with server", func(t *testing.T) {
		t.Parallel()

		app, client := ablytest.NewRealtime(ably.WithAutoConnect(false))
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
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, connectionChange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, connectionChange)
		}

		err = channel.Detach(ctx)
		if err != nil {
			t.Fatal(err)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetaching, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, connectionChange)
		}

		ablytest.Instantly.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetached, channelStatechange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, connectionChange)
		}

		ablytest.Instantly.NoRecv(t, nil, connectionStateChanges, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, channelStateChanges, t.Fatalf)
	})

	t.Run("RTL5e: return error if channel detach fails", func(t *testing.T) {

		in, out, _, channel, stateChanges, _ := setup(t)

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

		go func() {
			time.Sleep(time.Second / 2)
			channel.SetState(ably.ChannelStateFailed, nil)
		}()

		err := channel.Detach(context.Background())

		if err == nil {
			t.Fatal("detach should return timeout error")
		}

		if ably.UnwrapErrorCode(err) != 90000 {
			t.Fatalf("want code=90000; got %d", ably.UnwrapErrorCode(err))
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
		if expected, got := ably.ChannelStateFailed, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		if expected, got := err, change.Reason; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		ablytest.Instantly.NoRecv(t, &change, stateChanges, t.Fatalf)
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

		errCh := make(chan error)
		go func() {
			errCh <- channel.Detach(context.Background())
		}()

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

	t.Run("RTL5g: If connection state is CLOSING or FAILED, should return error", func(t *testing.T) {
		t.Parallel()

		in, _, c, channel, stateChanges, _ := setup(t)
		channel.OnAll(stateChanges.Receive)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		connectionChange := make(chan ably.ConnectionStateChange)
		c.Connection.OnAll(func(change ably.ConnectionStateChange) {
			connectionChange <- change
		})

		// Get the channel to ATTACHED.

		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}

		var change ably.ChannelStateChange

		ablytest.Soon.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		c.Close()
		// set connection state to closing
		c.Connection.SetState(ably.ConnectionStateClosing, nil, time.Minute)

		ablytest.Instantly.Recv(t, nil, connectionChange, t.Fatalf) // Consume connection state change to closing
		err := channel.Detach(ctx)
		if expected, got := "cannot Detach channel because connection is in CLOSING state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}

		// set connection state to failed
		c.Connection.SetState(ably.ConnectionStateFailed, nil, time.Minute)
		ablytest.Instantly.Recv(t, nil, connectionChange, t.Fatalf) // Consume connection state change to closing
		err = channel.Detach(ctx)
		if expected, got := "cannot Detach channel because connection is in FAILED state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}
	})

	t.Run("RTL5h : If Connection state INITIALIZED, queue the DETACH message and send on CONNECTED", func(t *testing.T) {
		t.Parallel()

		in, out, c, channel, _, afterCalls := setup(t)
		connectionChange := make(chan ably.ConnectionStateChange)
		c.Connection.OnAll(func(change ably.ConnectionStateChange) {
			connectionChange <- change
		})

		channelStateChange := make(chan ably.ChannelStateChange)
		channel.OnAll(func(change ably.ChannelStateChange) {
			channelStateChange <- change
		})
		var outMsg *proto.ProtocolMessage

		// Get the channel to ATTACHED.
		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}

		var channelChange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelChange, channelStateChange, t.Fatalf) // Consume attached message

		if expected, got := ably.ChannelStateAttached, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		var change ably.ConnectionStateChange

		// set connection state to ConnectionStateInitialized
		c.Connection.SetState(ably.ConnectionStateInitialized, nil, time.Minute)
		ablytest.Instantly.Recv(t, &change, connectionChange, t.Fatalf) // Consume connection state change to initialized

		if expected, got := ably.ConnectionStateInitialized, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		var rg ablytest.ResultGroup
		defer rg.Wait()
		rg.GoAdd(func(ctx context.Context) error {
			return channel.Detach(ctx)
		})

		ablytest.Instantly.Recv(t, &channelChange, channelStateChange, t.Fatalf) // transition to detaching

		if expected, got := ably.ChannelStateDetaching, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		// Cause a timeout; should reset to attached since detach message is queued
		var afterCall ablytest.AfterCall
		ablytest.Instantly.Recv(t, &afterCall, afterCalls, t.Fatalf)
		afterCall.Fire()

		ablytest.Instantly.Recv(t, &channelChange, channelStateChange, t.Fatalf)

		if expected, got := ably.ChannelStateAttached, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		ablytest.Instantly.NoRecv(t, &channelChange, channelStateChange, t.Fatalf) // Shouldn't receive detached state

		c.Connection.SetState(ably.ConnectionStateConnecting, nil, time.Minute)
		// get state to connecting
		ablytest.Instantly.Recv(t, &change, connectionChange, t.Fatalf) // Consume connection state to connecting

		if expected, got := ably.ConnectionStateConnecting, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		in <- &proto.ProtocolMessage{
			Action:            proto.ActionConnected,
			ConnectionID:      "connection-id",
			ConnectionDetails: &proto.ConnectionDetails{},
		}

		ablytest.Soon.Recv(t, &change, connectionChange, t.Fatalf) // Consume connection state to connected

		if expected, got := ably.ConnectionStateConnected, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		ablytest.Soon.Recv(t, &outMsg, out, t.Fatalf)
		if expected, got := proto.ActionDetach, outMsg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, outMsg.Action)
		}
	})

	t.Run("RTL5h : If Connection state CONNECTING, queue the DETACH message and send on CONNECTED", func(t *testing.T) {
		t.Parallel()

		in, out, c, channel, _, afterCalls := setup(t)
		connectionChange := make(chan ably.ConnectionStateChange)
		c.Connection.OnAll(func(change ably.ConnectionStateChange) {
			connectionChange <- change
		})

		channelStateChange := make(chan ably.ChannelStateChange)
		channel.OnAll(func(change ably.ChannelStateChange) {
			channelStateChange <- change
		})
		var outMsg *proto.ProtocolMessage

		// Get the channel to ATTACHED.
		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}

		var channelChange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelChange, channelStateChange, t.Fatalf) // Consume attached message

		if expected, got := ably.ChannelStateAttached, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		var change ably.ConnectionStateChange

		// set connection state to ConnectionStateConnecting
		c.Connection.SetState(ably.ConnectionStateConnecting, nil, time.Minute)
		ablytest.Instantly.Recv(t, &change, connectionChange, t.Fatalf) // Consume connection state change to initialized

		if expected, got := ably.ConnectionStateConnecting, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		var rg ablytest.ResultGroup
		defer rg.Wait()
		rg.GoAdd(func(ctx context.Context) error {
			return channel.Detach(ctx)
		})

		ablytest.Instantly.Recv(t, &channelChange, channelStateChange, t.Fatalf) // transition to detaching

		if expected, got := ably.ChannelStateDetaching, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		// Cause a timeout; should reset to attached since detach message is queued
		var afterCall ablytest.AfterCall
		ablytest.Instantly.Recv(t, &afterCall, afterCalls, t.Fatalf)
		afterCall.Fire()

		ablytest.Instantly.Recv(t, &channelChange, channelStateChange, t.Fatalf)

		if expected, got := ably.ChannelStateAttached, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		ablytest.Instantly.NoRecv(t, &channelChange, channelStateChange, t.Fatalf) // Shouldn't receive detached state

		in <- &proto.ProtocolMessage{
			Action:            proto.ActionConnected,
			ConnectionID:      "connection-id",
			ConnectionDetails: &proto.ConnectionDetails{},
		}

		ablytest.Soon.Recv(t, &change, connectionChange, t.Fatalf) // Consume connection state to connected

		if expected, got := ably.ConnectionStateConnected, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		ablytest.Soon.Recv(t, &outMsg, out, t.Fatalf)
		if expected, got := proto.ActionDetach, outMsg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, outMsg.Action)
		}
	})

	t.Run("RTL5h : If Connection state DISCONNECTED, queue the DETACH message and send on CONNECTED", func(t *testing.T) {
		t.Parallel()

		in, out, c, channel, _, afterCalls := setup(t)
		connectionChange := make(chan ably.ConnectionStateChange)
		c.Connection.OnAll(func(change ably.ConnectionStateChange) {
			connectionChange <- change
		})

		channelStateChange := make(chan ably.ChannelStateChange)
		channel.OnAll(func(change ably.ChannelStateChange) {
			channelStateChange <- change
		})
		var outMsg *proto.ProtocolMessage

		// Get the channel to ATTACHED.
		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}

		var channelChange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelChange, channelStateChange, t.Fatalf) // Consume attached message

		if expected, got := ably.ChannelStateAttached, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		var change ably.ConnectionStateChange

		// set connection state to ConnectionStateInitialized
		c.Connection.SetState(ably.ConnectionStateDisconnected, nil, time.Minute)
		ablytest.Instantly.Recv(t, &change, connectionChange, t.Fatalf) // Consume connection state change to initialized

		if expected, got := ably.ConnectionStateDisconnected, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		var rg ablytest.ResultGroup
		defer rg.Wait()
		rg.GoAdd(func(ctx context.Context) error {
			return channel.Detach(ctx)
		})

		ablytest.Instantly.Recv(t, &channelChange, channelStateChange, t.Fatalf) // transition to detaching

		if expected, got := ably.ChannelStateDetaching, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		// Cause a timeout; should reset to attached since detach message is queued
		var afterCall ablytest.AfterCall
		ablytest.Instantly.Recv(t, &afterCall, afterCalls, t.Fatalf)
		afterCall.Fire()

		ablytest.Instantly.Recv(t, &channelChange, channelStateChange, t.Fatalf)

		if expected, got := ably.ChannelStateAttached, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		ablytest.Instantly.NoRecv(t, &channelChange, channelStateChange, t.Fatalf) // Shouldn't receive detached state

		c.Connection.SetState(ably.ConnectionStateConnecting, nil, time.Minute)
		// get state to connecting
		ablytest.Instantly.Recv(t, &change, connectionChange, t.Fatalf) // Consume connection state to connecting

		if expected, got := ably.ConnectionStateConnecting, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		in <- &proto.ProtocolMessage{
			Action:            proto.ActionConnected,
			ConnectionID:      "connection-id",
			ConnectionDetails: &proto.ConnectionDetails{},
		}

		ablytest.Soon.Recv(t, &change, connectionChange, t.Fatalf) // Consume connection state to connected

		if expected, got := ably.ConnectionStateConnected, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		ablytest.Soon.Recv(t, &outMsg, out, t.Fatalf)
		if expected, got := proto.ActionDetach, outMsg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, outMsg.Action)
		}
	})

	t.Run("RTL5i: If channel in DETACHING or ATTACHING state, do detach after completion of operation", func(t *testing.T) {
		t.Parallel()

		in, out, _, channel, stateChanges, _ := setup(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		channel.OnAll(stateChanges.Receive)

		var outMsg *proto.ProtocolMessage
		var change ably.ChannelStateChange

		cancelledContext, cancel := context.WithCancel(ctx)
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

		go func() {
			time.Sleep(time.Second / 2)
			// Get the channel from ATTACHING to ATTACHED.
			in <- &proto.ProtocolMessage{
				Action:  proto.ActionAttached,
				Channel: channel.Name,
			}

			ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
			if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
				t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
			}

			// sends detach message after channel is attached
			ablytest.Instantly.Recv(t, &outMsg, out, t.Fatalf)
			if expected, got := proto.ActionDetach, outMsg.Action; expected != got {
				t.Fatalf("expected %v; got %v (event: %+v)", expected, got, outMsg.Action)
			}

			ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
			if expected, got := ably.ChannelStateDetaching, change.Current; expected != got {
				t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
			}

			in <- &proto.ProtocolMessage{
				Action:  proto.ActionDetached,
				Channel: channel.Name,
			}

			ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
			if expected, got := ably.ChannelStateDetached, change.Current; expected != got {
				t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
			}
		}()

		err := channel.Detach(ctx)

		if err != nil {
			t.Fatal(err)
		}
		ablytest.Instantly.NoRecv(t, &change, stateChanges, t.Fatalf)
	})

	t.Run("RTL5j: if channel state is SUSPENDED, immediately transition to DETACHED state", func(t *testing.T) {
		t.Parallel()

		_, _, _, channel, stateChanges, _ := setup(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		channel.OnAll(stateChanges.Receive)

		channel.SetState(ably.ChannelStateSuspended, nil)
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
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		channel.OnAll(stateChanges.Receive)

		var outMsg *proto.ProtocolMessage
		var change ably.ChannelStateChange

		cancelledContext, cancel := context.WithCancel(ctx)
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

		// Get channel state to detaching
		channel.SetState(chDetaching, nil)

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetaching, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		// Send attach message
		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}

		// sends detach message instead of attaching the channel
		ablytest.Instantly.Recv(t, &outMsg, out, t.Fatalf)
		if expected, got := proto.ActionDetach, outMsg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, outMsg.Action)
		}

		ablytest.Instantly.NoRecv(t, &change, stateChanges, t.Fatalf)
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

			channel, close := c.Channel("test").To(transition...)
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

			channel, close := c.Channel("test").To(trans.channel...)
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

			published := func() bool {
				for _, m := range recorder.Sent() {
					if m.Action == proto.ActionMessage {
						return true
					}
				}
				return false
			}

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

			channel, close := c.Channel("test").To(trans.channel...)
			defer safeclose(t, close)

			close = c.To(trans.connAfter...)
			defer safeclose(t, close)

			publishErr := asyncPublish(channel)

			// Check that the message isn't published.

			published := func() bool {
				for _, m := range recorder.Sent() {
					if m.Action == proto.ActionMessage {
						return true
					}
				}
				return false
			}

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
		go channel.Attach(ctx)
		defer cancel()

		ablytest.Instantly.Recv(t, nil, out, t.Fatalf) // Consume ATTACH

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

		// TODO: Test attach failure too, which requires RTL4ef.

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
		call.Time <- time.Time{}

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

		call.Time <- time.Time{}

		// Since the connection isn't CONNECTED, the retry loop should finish.

		ablytest.Instantly.NoRecv(t, nil, afterCalls, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)
	})
}
