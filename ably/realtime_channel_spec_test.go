package ably_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"

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

	setUpWithInterrupt := func() (app *ablytest.Sandbox, client *ably.Realtime, interrupt chan *proto.ProtocolMessage, channel *ably.RealtimeChannel, stateChanges ably.ChannelStateChanges, eof chan struct{}) {
		interrupt = make(chan *proto.ProtocolMessage)
		eof = make(chan struct{})
		app, client = ablytest.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
				c, err := ablyutil.DialWebsocket(protocol, u, timeout)
				return protoConnWithFakeEOF{Conn: c, doEOF: eof, onMessage: func(msg *proto.ProtocolMessage) {
					if msg.Action == proto.ActionClosed {
						close(interrupt)
						return
					}
					if msg.Action == proto.ActionHeartbeat {
						return
					}
					interrupt <- msg
				}}, err
			}))

		channel = client.Channels.Get("test")
		stateChanges = make(ably.ChannelStateChanges, 10)
		channel.OnAll(func(change ably.ChannelStateChange) {
			stateChanges <- change
		})
		return
	}

	t.Run("RTL4a", func(t *testing.T) {
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

	t.Run("RTL4b", func(t *testing.T) {
		_, _, c, channel, _, _ := setup(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		connectionChange := make(chan ably.ConnectionStateChange)
		c.Connection.OnAll(func(change ably.ConnectionStateChange) {
			connectionChange <- change
		})

		c.Close()

		// set connection state to initialized
		c.Connection.SetState(ably.ConnectionStateInitialized, nil, time.Minute)

		ablytest.Instantly.Recv(t, nil, connectionChange, t.Fatalf) // Consume connection state change to closing
		err := channel.Attach(ctx)
		if expected, got := "cannot Attach channel because connection is in INITIALIZED state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}

		// set connection state to closing
		c.Connection.SetState(ably.ConnectionStateClosing, nil, time.Minute)

		ablytest.Instantly.Recv(t, nil, connectionChange, t.Fatalf) // Consume connection state change to closing
		err = channel.Attach(ctx)
		if expected, got := "cannot Attach channel because connection is in CLOSING state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}

		// set connection state to closed
		c.Connection.SetState(ably.ConnectionStateClosed, nil, time.Minute)

		ablytest.Instantly.Recv(t, nil, connectionChange, t.Fatalf) // Consume connection state change to closing
		err = channel.Attach(ctx)
		if expected, got := "cannot Attach channel because connection is in CLOSED state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}

		// set connection state to suspended
		c.Connection.SetState(ably.ConnectionStateSuspended, nil, time.Minute)

		ablytest.Instantly.Recv(t, nil, connectionChange, t.Fatalf) // Consume connection state change to closing
		err = channel.Attach(ctx)
		if expected, got := "cannot Attach channel because connection is in SUSPENDED state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}

		// set connection state to failed
		c.Connection.SetState(ably.ConnectionStateFailed, nil, time.Minute)
		ablytest.Instantly.Recv(t, nil, connectionChange, t.Fatalf) // Consume connection state change to closing
		err = channel.Attach(ctx)
		if expected, got := "cannot Attach channel because connection is in FAILED state", err.Error(); !strings.Contains(got, expected) {
			t.Fatalf("expected error %+v; got %v", expected, got)
		}
	})

	t.Run("RTL4c RTL4d", func(t *testing.T) {
		app, client, interrupt, channel, channelStateChange, _ := setUpWithInterrupt()
		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stateChange := make(ably.ConnStateChanges, 10)
		off := client.Connection.OnAll(stateChange.Receive)
		defer off()

		client.Connect()

		var change ably.ConnectionStateChange

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateConnecting, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		msg := <-interrupt // accept connected message

		if expected, got := proto.ActionConnected, msg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, msg)
		}

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateConnected, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		go func() {
			msg = <-interrupt // accept attached message
			if expected, got := proto.ActionAttached, msg.Action; expected != got {
				t.Fatalf("expected %v; got %v (event: %+v)", expected, got, msg)
			}
		}()

		err := channel.Attach(ctx)

		if err != nil {
			t.Fatal(err)
		}

		var channelChange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelChange, channelStateChange, t.Fatalf) // Consume attaching event state

		if expected, got := ably.ChannelStateAttaching, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		ablytest.Soon.Recv(t, &channelChange, channelStateChange, t.Fatalf) // Consume attached event state

		if expected, got := ably.ChannelStateAttached, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		ablytest.Instantly.NoRecv(t, &channelChange, channelStateChange, t.Fatalf) // Shouldn't change channel state again\
		ablytest.Instantly.NoRecv(t, &change, stateChange, t.Fatalf)
	})

	t.Run("RTL4d", func(t *testing.T) {

	})

	t.Run("RTL4e", func(t *testing.T) {

	})

	t.Run("RTL4f: Channel attach timeout test", func(t *testing.T) {
		_, out, _, channel, stateChanges, _ := setup(t)
		channel.OnAll(stateChanges.Receive)

		var change ably.ChannelStateChange

		var outMsg *proto.ProtocolMessage
		//
		err := channel.Attach(context.Background())

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

	t.Run("RTL4g", func(t *testing.T) {

	})

	t.Run("RTL4h", func(t *testing.T) {
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

		in <- &proto.ProtocolMessage{
			Action:  proto.ActionAttached,
			Channel: channel.Name,
		}

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		channel.Detach(cancelledContext)

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		if expected, got := ably.ChannelStateDetaching, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		ablytest.Instantly.Recv(t, &outMsg, out, t.Fatalf)
		if expected, got := proto.ActionDetach, outMsg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, outMsg.Action)
		}

		go func() {
			time.Sleep(time.Second / 2)
			// Get the channel from ATTACHING to ATTACHED.
			in <- &proto.ProtocolMessage{
				Action:  proto.ActionDetached,
				Channel: channel.Name,
			}

			ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
			if expected, got := ably.ChannelStateDetached, change.Current; expected != got {
				t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
			}

			// sends detach message after channel is attached
			ablytest.Instantly.Recv(t, &outMsg, out, t.Fatalf)
			if expected, got := proto.ActionAttach, outMsg.Action; expected != got {
				t.Fatalf("expected %v; got %v (event: %+v)", expected, got, outMsg.Action)
			}

			ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
			if expected, got := ably.ChannelStateAttaching, change.Current; expected != got {
				t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
			}

			in <- &proto.ProtocolMessage{
				Action:  proto.ActionAttached,
				Channel: channel.Name,
			}

			ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
			if expected, got := ably.ChannelStateAttached, change.Current; expected != got {
				t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
			}
		}()

		err := channel.Attach(ctx)
		if err != nil {
			t.Fatal(err)
		}
		ablytest.Instantly.NoRecv(t, &change, stateChanges, t.Fatalf)
	})

	t.Run("RTL4i : Connecting state", func(t *testing.T) {
		app, client, interrupt, channel, channelStateChange, _ := setUpWithInterrupt()
		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stateChange := make(ably.ConnStateChanges, 10)
		off := client.Connection.OnAll(stateChange.Receive)
		defer off()

		client.Connect()

		var change ably.ConnectionStateChange

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateConnecting, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		channel.Attach(ctx)

		var channelChange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelChange, channelStateChange, t.Fatalf) // Consume attaching event state

		if expected, got := ably.ChannelStateAttaching, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		ablytest.Instantly.Recv(t, &channelChange, channelStateChange, t.Fatalf) // Should timeout and set to suspended

		if expected, got := ably.ChannelStateSuspended, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		ablytest.Instantly.NoRecv(t, &channelChange, channelStateChange, t.Fatalf) // Shouldn't receive attached message, since it's queued

		msg := <-interrupt // accept connected message

		if expected, got := proto.ActionConnected, msg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, msg)
		}

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateConnected, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		msg = <-interrupt // accept attached message

		if expected, got := proto.ActionAttached, msg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, msg)
		}

		ablytest.Soon.Recv(t, &channelChange, channelStateChange, t.Fatalf) // Consume attached event state

		if expected, got := ably.ChannelStateAttached, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		ablytest.Instantly.NoRecv(t, &channelChange, channelStateChange, t.Fatalf) // Shouldn't change channel state again\
		ablytest.Instantly.NoRecv(t, &change, stateChange, t.Fatalf)
	})

	t.Run("RTL4i : Disconnected state", func(t *testing.T) {
		app, client, interrupt, channel, channelStateChange, doEOF := setUpWithInterrupt()
		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stateChange := make(ably.ConnStateChanges, 10)
		off := client.Connection.OnAll(stateChange.Receive)
		defer off()

		client.Connect()

		var change ably.ConnectionStateChange

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateConnecting, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		msg := <-interrupt // accept connected message

		if expected, got := proto.ActionConnected, msg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, msg)
		}

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateConnected, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		doEOF <- struct{}{}

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateDisconnected, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		channel.Attach(ctx)

		var channelChange ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &channelChange, channelStateChange, t.Fatalf) // Consume attaching event state

		if expected, got := ably.ChannelStateAttaching, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		ablytest.Instantly.Recv(t, &channelChange, channelStateChange, t.Fatalf) // Should timeout and set to suspended

		if expected, got := ably.ChannelStateSuspended, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		ablytest.Instantly.NoRecv(t, &channelChange, channelStateChange, t.Fatalf) // Shouldn't receive attached message, since it's queued

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateConnecting, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		msg = <-interrupt // accept connected message

		if expected, got := proto.ActionConnected, msg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, msg)
		}

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateConnected, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		msg = <-interrupt // accept attached message

		if expected, got := proto.ActionAttached, msg.Action; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, msg)
		}

		ablytest.Soon.Recv(t, &channelChange, channelStateChange, t.Fatalf) // Consume attached event state

		if expected, got := ably.ChannelStateAttached, channelChange.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, channelChange)
		}

		ablytest.Instantly.NoRecv(t, &channelChange, channelStateChange, t.Fatalf) // Shouldn't change channel state again\
		ablytest.Instantly.NoRecv(t, &change, stateChange, t.Fatalf)
	})

	t.Run("RTL4j", func(t *testing.T) {

	})

	t.Run("RTL4k", func(t *testing.T) {

	})

	t.Run("RTL4l", func(t *testing.T) {

	})

	t.Run("RTL4m", func(t *testing.T) {

	})

}

func TestRealtimeChannel_RTL6c1_PublishNow(t *testing.T) {
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
