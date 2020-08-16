package ably_test

import (
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

func TestRealtimeChannel_RTL13_HandleDetached(t *testing.T) {
	t.Parallel()

	const channelRetryTimeout = 123 * time.Millisecond

	type afterCall struct {
		d   time.Duration
		ret chan<- time.Time
	}

	setup := func(t *testing.T) (
		in, out chan *proto.ProtocolMessage,
		c *ably.Realtime,
		channel *ably.RealtimeChannel,
		stateChanges chan ably.State,
		afterCalls chan afterCall,
	) {
		in = make(chan *proto.ProtocolMessage, 1)
		out = make(chan *proto.ProtocolMessage, 16)
		afterCalls = make(chan afterCall, 1)

		c, _ = ably.NewRealtime(ably.ClientOptions{}.
			Token("fake:token").
			AutoConnect(false).
			After(func(d time.Duration) <-chan time.Time {
				ch := make(chan time.Time)
				afterCalls <- afterCall{d: d, ret: ch}
				return ch
			}).
			ChannelRetryTimeout(channelRetryTimeout).
			Dial(ablytest.MessagePipe(in, out)))

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

		_, err = channel.Attach()
		if err != nil {
			t.Fatal(err)
		}

		ablytest.Instantly.Recv(t, nil, out, t.Fatalf) // Consume ATTACH

		stateChanges = make(chan ably.State, 10)
		channel.On(stateChanges)

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

		var change ably.State

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.StateChanAttached, change.State; expected != got {
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
		if expected, got := ably.StateChanAttaching, change.State; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		if got := fmt.Sprint(change.Err); !strings.Contains(got, errInfo.Message) {
			t.Fatalf("expected %+v; got %v (error: %+v)", errInfo, got, change.Err)
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
		if expected, got := ably.StateChanAttached, change.State; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		if change.Err != nil {
			t.Fatal(change.Err)
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

		var change ably.State
		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.StateChanDetached, change.State; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		if got := fmt.Sprint(change.Err); !strings.Contains(got, errInfo.Message) {
			t.Fatalf("expected %+v; got %v (error: %+v)", errInfo, got, change.Err)
		}

		// Expect an attempt to attach after channelRetryTimeout.

		var call afterCall
		ablytest.Instantly.Recv(t, &call, afterCalls, t.Fatalf)
		if expected, got := channelRetryTimeout, call.d; expected != got {
			t.Fatalf("expected %v; got %v", expected, got)
		}
		call.ret <- time.Time{}

		// Expect a transition to ATTACHING, and an ATTACH message.

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.StateChanAttaching, change.State; expected != got {
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
		if expected, got := ably.StateChanAttached, change.State; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		if change.Err != nil {
			t.Fatal(change.Err)
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

		var change ably.State
		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.StateChanDetached, change.State; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}
		if got := fmt.Sprint(change.Err); !strings.Contains(got, errInfo.Message) {
			t.Fatalf("expected %+v; got %v (error: %+v)", errInfo, got, change.Err)
		}

		// Expect an attempt to attach after channelRetryTimeout.

		var call afterCall
		ablytest.Instantly.Recv(t, &call, afterCalls, t.Fatalf)
		if expected, got := channelRetryTimeout, call.d; expected != got {
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

		call.ret <- time.Time{}

		// Since the connection isn't CONNECTED, the retry loop should finish.

		ablytest.Instantly.NoRecv(t, nil, afterCalls, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)
	})
}
