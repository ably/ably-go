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

	t.Run(fmt.Sprintf("on %s", ably.ChannelEventUpdated), func(t *testing.T) {
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

		err := ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected).Wait()
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

		err := ablytest.ConnWaiter(c, func() {
			close(in)
		}, ably.ConnectionEventDisconnected).Wait()
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
