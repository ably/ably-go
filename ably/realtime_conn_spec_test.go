package ably_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ably/proto"
)

func Test_RTN3_ConnectionAutoConnect(t *testing.T) {
	t.Parallel()

	recorder := ablytest.NewMessageRecorder()

	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(true),
		ably.WithDial(recorder.Dial))

	connectionStateChanges := make(ably.ConnStateChanges, 10)
	off := client.Connection.OnAll(connectionStateChanges.Receive)
	defer off()

	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	var connectionChange ably.ConnectionStateChange

	// Transition to connected without needing to explicitly connect
	ablytest.Soon.Recv(t, &connectionChange, connectionStateChanges, t.Fatalf)
	if expected, got := ably.ConnectionStateConnected, connectionChange.Current; expected != got {
		t.Fatalf("expected %v; got %v (event: %+v)", expected, got, connectionChange)
	}
	ablytest.Instantly.NoRecv(t, nil, connectionStateChanges, t.Fatalf)
}

func Test_RTN4a_ConnectionEventForStateChange(t *testing.T) {
	t.Run(fmt.Sprintf("on %s", ably.ConnectionStateConnecting), func(t *testing.T) {
		t.Parallel()

		app, realtime := ablytest.NewRealtime(ably.WithAutoConnect(false))
		defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

		changes := make(chan ably.ConnectionStateChange)
		defer ablytest.Instantly.NoRecv(t, nil, changes, t.Errorf)

		realtime.Connection.On(ably.ConnectionEventConnecting, func(change ably.ConnectionStateChange) {
			changes <- change
		})

		realtime.Connect()

		ablytest.Soon.Recv(t, nil, changes, t.Fatalf)
	})

	t.Run(fmt.Sprintf("on %s", ably.ConnectionStateConnected), func(t *testing.T) {
		t.Parallel()

		app, realtime := ablytest.NewRealtime(ably.WithAutoConnect(false))
		defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

		connectAndWait(t, realtime)
	})

	t.Run(fmt.Sprintf("on %s", ably.ConnectionStateDisconnected), func(t *testing.T) {
		t.Parallel()

		dial, disconnect := ablytest.DialFakeDisconnect(nil)
		options := []ably.ClientOption{
			ably.WithAutoConnect(false),
			ably.WithDial(dial),
		}
		app, realtime := ablytest.NewRealtime(options...)
		defer safeclose(t, app)
		defer realtime.Close()

		connectAndWait(t, realtime)

		changes := make(chan ably.ConnectionStateChange)
		defer ablytest.Instantly.NoRecv(t, nil, changes, t.Errorf)

		realtime.Connection.On(ably.ConnectionEventDisconnected, func(change ably.ConnectionStateChange) {
			changes <- change
		})

		err := disconnect()
		if err != nil {
			t.Fatalf("fake disconnection failed: %v", err)
		}

		ablytest.Soon.Recv(t, nil, changes, t.Fatalf)

	})

	t.Run(fmt.Sprintf("on %s", ably.ConnectionStateSuspended), func(t *testing.T) {
		t.Parallel()

		t.Skip("SUSPENDED not yet implemented")
	})

	t.Run(fmt.Sprintf("on %s", ably.ConnectionStateClosing), func(t *testing.T) {
		t.Parallel()

		app, realtime := ablytest.NewRealtime(ably.WithAutoConnect(false))
		defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

		connectAndWait(t, realtime)

		changes := make(chan ably.ConnectionStateChange)
		defer ablytest.Instantly.NoRecv(t, nil, changes, t.Errorf)

		realtime.Connection.On(ably.ConnectionEventClosing, func(change ably.ConnectionStateChange) {
			changes <- change
		})

		realtime.Close()
		ablytest.Soon.Recv(t, nil, changes, t.Fatalf)
	})

	t.Run(fmt.Sprintf("on %s", ably.ConnectionStateClosed), func(t *testing.T) {
		t.Parallel()

		app, realtime := ablytest.NewRealtime(ably.WithAutoConnect(false))
		defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

		connectAndWait(t, realtime)

		changes := make(chan ably.ConnectionStateChange)
		defer ablytest.Instantly.NoRecv(t, nil, changes, t.Errorf)

		realtime.Connection.On(ably.ConnectionEventClosed, func(change ably.ConnectionStateChange) {
			changes <- change
		})

		realtime.Close()
		ablytest.Soon.Recv(t, nil, changes, t.Fatalf)
	})

	t.Run(fmt.Sprintf("on %s", ably.ConnectionStateFailed), func(t *testing.T) {
		t.Parallel()

		options := []ably.ClientOption{
			ably.WithEnvironment("sandbox"),
			ably.WithAutoConnect(false),
			ably.WithKey("made:up"),
		}

		realtime, err := ably.NewRealtime(options...)
		if err != nil {
			t.Fatalf("unexpected err: %s", err)
		}

		changes := make(chan ably.ConnectionStateChange)
		defer ablytest.Instantly.NoRecv(t, nil, changes, t.Errorf)

		realtime.Connection.On(ably.ConnectionEventFailed, func(change ably.ConnectionStateChange) {
			changes <- change
		})

		realtime.Connect()
		ablytest.Soon.Recv(t, nil, changes, t.Fatalf)
	})
}

func connectAndWait(t *testing.T, realtime *ably.Realtime) {
	t.Helper()

	changes := make(chan ably.ConnectionStateChange, 2)
	defer ablytest.Instantly.NoRecv(t, nil, changes, t.Errorf)

	{
		off := realtime.Connection.Once(ably.ConnectionEventConnected, func(change ably.ConnectionStateChange) {
			changes <- change
		})
		defer off()
	}

	{
		off := realtime.Connection.Once(ably.ConnectionEventFailed, func(change ably.ConnectionStateChange) {
			changes <- change
		})
		defer off()
	}

	realtime.Connect()

	var change ably.ConnectionStateChange
	ablytest.Soon.Recv(t, &change, changes, t.Fatalf)

	if change.Current != ably.ConnectionStateConnected {
		t.Fatalf("unexpected FAILED event: %s", change.Reason)
	}
}

type connectionStateChanges chan ably.ConnectionStateChange

func (c connectionStateChanges) Receive(change ably.ConnectionStateChange) {
	c <- change
}

func TestRealtimeConn_RTN10_ConnectionSerial(t *testing.T) {
	t.Run("RTN10a: Should be unset until connected, should set after connected", func(t *testing.T) {
		connDetails := proto.ConnectionDetails{
			ConnectionKey:      "foo",
			ConnectionStateTTL: proto.DurationFromMsecs(time.Minute * 20),
			MaxIdleInterval:    proto.DurationFromMsecs(time.Minute * 5),
		}

		in := make(chan *proto.ProtocolMessage, 1)
		out := make(chan *proto.ProtocolMessage, 16)

		c, _ := ably.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithToken("fake:token"),
			ably.WithDial(ablytest.MessagePipe(in, out)))

		stateChange := make(connectionStateChanges, 2)
		c.Connection.OnAll(stateChange.Receive)

		if expected, got := ably.ConnectionStateInitialized, c.Connection.State(); expected != got {
			t.Fatalf("expected %v; got %v", expected, got)
		}

		serial := c.Connection.Serial()

		if serial != nil {
			t.Fatal("Connection serial should be nil when initialized/not connected")
		}

		c.Connect()

		var change ably.ConnectionStateChange

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateConnecting, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}

		serial = c.Connection.Serial()

		if serial != nil {
			t.Fatal("Connection serial should be nil when connecting/not connected")
		}

		in <- &proto.ProtocolMessage{
			Action:            proto.ActionConnected,
			ConnectionID:      "connection",
			ConnectionSerial:  2,
			ConnectionDetails: &connDetails,
		}

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateConnected, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}

		err := ablytest.Wait(ablytest.AssertionWaiter(func() bool {
			return *c.Connection.Serial() == 2
		}), nil)

		if err != nil {
			t.Fatalf("Expected %v, Received %v", 2, *c.Connection.Serial())
		}
	})

	t.Run("RTN10b: Should be set everytime message with connection-serial is received", func(t *testing.T) {
		connDetails := proto.ConnectionDetails{
			ConnectionKey:      "foo",
			ConnectionStateTTL: proto.DurationFromMsecs(time.Minute * 20),
			MaxIdleInterval:    proto.DurationFromMsecs(time.Minute * 5),
		}

		in := make(chan *proto.ProtocolMessage, 1)
		out := make(chan *proto.ProtocolMessage, 16)

		in <- &proto.ProtocolMessage{
			Action:            proto.ActionConnected,
			ConnectionID:      "connection",
			ConnectionSerial:  2,
			ConnectionDetails: &connDetails,
		}

		c, _ := ably.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithToken("fake:token"),
			ably.WithDial(ablytest.MessagePipe(in, out)))

		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		if err != nil {
			t.Fatal(err)
		}

		serial := *c.Connection.Serial()
		if serial != 2 {
			t.Fatal("Connection serial should be set to 2")
		}

		in <- &proto.ProtocolMessage{
			Action:            proto.ActionAttached,
			ConnectionID:      "connection",
			ConnectionSerial:  4,
			ConnectionDetails: &connDetails,
		}

		err = ablytest.Wait(ablytest.AssertionWaiter(func() bool {
			return *c.Connection.Serial() == 4
		}), nil)

		if err != nil {
			t.Fatalf("Expected %v, Received %v", 4, *c.Connection.Serial())
		}

		in <- &proto.ProtocolMessage{
			Action:            proto.ActionMessage,
			ConnectionID:      "connection",
			ConnectionSerial:  5,
			ConnectionDetails: &connDetails,
		}

		err = ablytest.Wait(ablytest.AssertionWaiter(func() bool {
			return *c.Connection.Serial() == 5
		}), nil)

		if err != nil {
			t.Fatalf("Expected %v, Received %v", 5, *c.Connection.Serial())
		}

		in <- &proto.ProtocolMessage{
			Action:            proto.ActionHeartbeat,
			ConnectionID:      "connection",
			ConnectionSerial:  6,
			ConnectionDetails: &connDetails,
		}

		err = ablytest.Wait(ablytest.AssertionWaiter(func() bool {
			return *c.Connection.Serial() == 6
		}), nil)

		if err != nil {
			t.Fatalf("Expected %v, Received %v", 6, *c.Connection.Serial())
		}
	})
}

func TestRealtimeConn_RTN12_Connection_Close(t *testing.T) {

	setUpWithEOF := func() (app *ablytest.Sandbox, client *ably.Realtime, doEOF chan struct{}) {
		doEOF = make(chan struct{}, 1)

		app, client = ablytest.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
				c, err := ablyutil.DialWebsocket(protocol, u, timeout)
				return protoConnWithFakeEOF{Conn: c, doEOF: doEOF}, err
			}))

		err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
		if err != nil {
			t.Fatal(err)
		}
		connectionState := client.Connection.State()
		if connectionState != ably.ConnectionStateConnected {
			t.Fatalf("expected %v; got %v", ably.ConnectionStateConnected, connectionState)
		}
		return
	}

	setUpWithConnectingInterrupt := func() (app *ablytest.Sandbox, client *ably.Realtime, dialErr chan error, waitTillDial chan error) {
		dialErr = make(chan error, 1)
		waitTillDial = make(chan error)
		app, client = ablytest.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
				waitTillDial <- nil
				if err := <-dialErr; err != nil {
					return nil, err
				}
				return ablyutil.DialWebsocket(protocol, u, timeout)
			}))
		return
	}

	t.Run("RTN12a: transition to closed on connection close", func(t *testing.T) {
		t.Parallel()
		app, client, _ := setUpWithEOF()
		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

		stateChange := make(connectionStateChanges, 2)
		client.Connection.OnAll(stateChange.Receive)

		client.Close()

		var change ably.ConnectionStateChange
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)

		if expected, got := ably.ConnectionStateClosing, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)

		if expected, got := ably.ConnectionStateClosed, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}
		ablytest.Instantly.NoRecv(t, nil, stateChange, t.Fatalf)

	})

	t.Run("RTN12b: transition to closed on close request timeout", func(t *testing.T) {
		t.Parallel()
		connDetails := proto.ConnectionDetails{
			ConnectionKey:      "foo",
			ConnectionStateTTL: proto.DurationFromMsecs(time.Minute * 20),
			MaxIdleInterval:    proto.DurationFromMsecs(time.Minute * 5),
		}

		afterCalls := make(chan ablytest.AfterCall)
		now, after := ablytest.TimeFuncs(afterCalls)

		var in chan *proto.ProtocolMessage
		realtimeRequestTimeout := time.Minute
		c, _ := ably.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithToken("fake:token"),
			ably.WithRealtimeRequestTimeout(realtimeRequestTimeout),
			ably.WithNow(now),
			ably.WithAfter(after),
			ably.WithDial(func(p string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
				in = make(chan *proto.ProtocolMessage, 1)
				out := make(chan *proto.ProtocolMessage, 16)
				in <- &proto.ProtocolMessage{
					Action:            proto.ActionConnected,
					ConnectionID:      "connection",
					ConnectionDetails: &connDetails,
				}
				return ablytest.MessagePipe(in, out,
					ablytest.MessagePipeWithNowFunc(now),
					ablytest.MessagePipeWithAfterFunc(after),
				)(p, u, timeout)
			}))

		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		if err != nil {
			t.Fatal(err)
		}

		stateChange := make(connectionStateChanges, 2)
		c.Connection.OnAll(stateChange.Receive)

		maxIdleInterval := time.Duration(connDetails.MaxIdleInterval)
		receiveTimeout := realtimeRequestTimeout + maxIdleInterval

		c.Close()
		// RTN23a The connection should be closed due to lack of activity past
		// receiveTimeout
		var change ably.ConnectionStateChange
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)

		if expected, got := ably.ConnectionStateClosing, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}

		// Expect timer for a message receive.
		var timer ablytest.AfterCall

		ablytest.Instantly.Recv(t, &timer, afterCalls, t.Fatalf)
		if expected, got := receiveTimeout, timer.D; expected != got {
			t.Fatalf("expected %v, got %v", expected, got)
		}

		// Let the deadline pass without a message; expect a disconnection.
		timer.Fire()

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateClosed, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}

		errReason := change.Reason
		// make sure the reason is timeout
		if !strings.Contains(errReason.Error(), "timeout") {
			t.Errorf("expected %q to contain timeout", errReason.Error())
		}

		ablytest.Instantly.NoRecv(t, nil, stateChange, t.Fatalf)
	})

	t.Run("RTN12c: transition to closed on transport error", func(t *testing.T) {
		t.Parallel()
		app, client, doEOF := setUpWithEOF()
		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

		stateChange := make(connectionStateChanges, 2)
		client.Connection.OnAll(stateChange.Receive)

		client.Close()

		var change ably.ConnectionStateChange
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)

		if expected, got := ably.ConnectionStateClosing, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}

		doEOF <- struct{}{}

		select {
		case change = <-stateChange:
		case <-time.After(50 * time.Millisecond):
			t.Fatal("didn't transition on EOF")
		}

		if expected, got := ably.ConnectionStateClosed, change; expected != got.Current {
			t.Fatalf("expected transition to %v, got %v", expected, got)
		}

		ablytest.Instantly.NoRecv(t, nil, stateChange, t.Fatalf)
	})

	t.Run("RTN12d : should abort reconnection timer while disconnected on closed", func(t *testing.T) {
		t.Parallel()
		connDetails := proto.ConnectionDetails{
			ConnectionKey:      "foo",
			ConnectionStateTTL: proto.DurationFromMsecs(time.Minute * 20),
			MaxIdleInterval:    proto.DurationFromMsecs(time.Minute * 5),
		}

		afterCalls := make(chan ablytest.AfterCall)
		now, after := ablytest.TimeFuncs(afterCalls)

		var in chan *proto.ProtocolMessage

		dialErr := make(chan error, 1)

		realtimeRequestTimeout := time.Minute
		c, _ := ably.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithToken("fake:token"),
			ably.WithRealtimeRequestTimeout(realtimeRequestTimeout),
			ably.WithNow(now),
			ably.WithAfter(after),
			ably.WithDial(func(p string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
				if err := <-dialErr; err != nil {
					return nil, err
				}
				in = make(chan *proto.ProtocolMessage, 1)
				out := make(chan *proto.ProtocolMessage, 16)
				in <- &proto.ProtocolMessage{
					Action:            proto.ActionConnected,
					ConnectionID:      "connection",
					ConnectionDetails: &connDetails,
				}
				return ablytest.MessagePipe(in, out,
					ablytest.MessagePipeWithNowFunc(now),
					ablytest.MessagePipeWithAfterFunc(after),
				)(p, u, timeout)
			}))

		dialErr <- nil
		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		if err != nil {
			t.Fatal(err)
		}

		stateChange := make(connectionStateChanges, 2)
		c.Connection.OnAll(stateChange.Receive)

		maxIdleInterval := time.Duration(connDetails.MaxIdleInterval)
		receiveTimeout := realtimeRequestTimeout + maxIdleInterval

		// Expect timer for a message receive.
		var timer ablytest.AfterCall

		// receive message request timeout inside eventloop
		ablytest.Instantly.Recv(t, &timer, afterCalls, t.Fatalf)
		if expected, got := receiveTimeout, timer.D; expected != got {
			t.Fatalf("expected %v, got %v", expected, got)
		}

		// Let the deadline pass without a message; expect a disconnection.
		timer.Fire()
		dialErr <- errors.New("can't reconnect once disconnected")

		var change ably.ConnectionStateChange
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)

		// received disconnect on first receive
		if expected, got := ably.ConnectionStateDisconnected, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}

		// retry connection with ably server
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateConnecting, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}

		dialErr <- errors.New("can't reconnect once disconnected")
		// first reconnect failed (raw connection can't be established, outside retry loop), so returned disconnect
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateDisconnected, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}

		// consume parent suspend timer with timeout connectionState TTL
		ablytest.Instantly.Recv(t, nil, afterCalls, t.Fatalf)
		// Went inside retry loop for connecting with server again
		// consume and wait for disconnect retry timer with disconnectedRetryTimeout
		ablytest.Instantly.Recv(t, nil, afterCalls, t.Fatalf)

		c.Close()
		// state change should triggger disconnect timer to trigger and stop the loop due to ConnectionStateClosed
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateClosed, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}
		ablytest.Before(time.Second).NoRecv(t, nil, stateChange, t.Fatalf)
	})

	t.Run("RTN12d: should abort reconnection timer while suspended on closed", func(t *testing.T) {
		t.Parallel()

		connDetails := proto.ConnectionDetails{
			ConnectionKey:      "foo",
			ConnectionStateTTL: proto.DurationFromMsecs(time.Minute * 20),
			MaxIdleInterval:    proto.DurationFromMsecs(time.Minute * 5),
		}

		afterCalls := make(chan ablytest.AfterCall)
		now, after := ablytest.TimeFuncs(afterCalls)

		var in chan *proto.ProtocolMessage

		dialErr := make(chan error, 1)
		realtimeRequestTimeout := time.Minute
		c, _ := ably.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithToken("fake:token"),
			ably.WithRealtimeRequestTimeout(realtimeRequestTimeout),
			ably.WithNow(now),
			ably.WithAfter(after),
			ably.WithDial(func(p string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
				if err := <-dialErr; err != nil {
					return nil, err
				}
				in = make(chan *proto.ProtocolMessage, 1)
				out := make(chan *proto.ProtocolMessage, 16)
				in <- &proto.ProtocolMessage{
					Action:            proto.ActionConnected,
					ConnectionID:      "connection",
					ConnectionDetails: &connDetails,
				}
				return ablytest.MessagePipe(in, out,
					ablytest.MessagePipeWithNowFunc(now),
					ablytest.MessagePipeWithAfterFunc(after),
				)(p, u, timeout)
			}))

		dialErr <- nil
		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		if err != nil {
			t.Fatal(err)
		}

		stateChange := make(connectionStateChanges, 2)
		c.Connection.OnAll(stateChange.Receive)

		maxIdleInterval := time.Duration(connDetails.MaxIdleInterval)
		receiveTimeout := realtimeRequestTimeout + maxIdleInterval

		// Expect timer for a message receive.
		var timer ablytest.AfterCall

		// receive message request timeout inside eventloop
		ablytest.Instantly.Recv(t, &timer, afterCalls, t.Fatalf)
		if expected, got := receiveTimeout, timer.D; expected != got {
			t.Fatalf("expected %v, got %v", expected, got)
		}

		// Let the deadline pass without a message; expect a disconnection.
		timer.Fire()
		dialErr <- errors.New("can't reconnect once disconnected")

		var change ably.ConnectionStateChange
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)

		// received disconnect on first receive
		if expected, got := ably.ConnectionStateDisconnected, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}
		// retry connection with ably server
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateConnecting, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}

		dialErr <- errors.New("can't reconnect once disconnected")
		// first connect failed (raw connection can't be established), so returned disconnect
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateDisconnected, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}

		// consume parent suspend timer with timeout connectionStateTTL
		reconnect, suspend := ablytest.GetReconnectionTimersFrom(t, afterCalls)
		suspend.Fire()                                       // fire suspend timer
		ablytest.Wait(ablytest.AssertionWaiter(func() bool { // make sure suspend timer is triggered
			return suspend.IsTriggered()
		}), nil)
		// Went inside retry loop for connecting with server again
		// consume and wait for disconnect retry timer with disconnectedRetryTimeout
		reconnect.Fire()                                     // fire disconnect timer
		ablytest.Wait(ablytest.AssertionWaiter(func() bool { // make sure disconnect timer is triggered
			return reconnect.IsTriggered()
		}), nil)

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateSuspended, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}

		// consume and wait for suspend retry timer with suspendedRetryTimeout
		ablytest.Instantly.Recv(t, nil, afterCalls, t.Fatalf)

		c.Close()

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateClosed, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change.Current)
		}

		ablytest.Before(time.Second).NoRecv(t, nil, stateChange, t.Fatalf)
	})

	t.Run("RTN12f: transition to closed when close is called intermittently", func(t *testing.T) {
		t.Parallel()
		app, client, interrupt, waitTillConnecting := setUpWithConnectingInterrupt()
		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

		stateChange := make(ably.ConnStateChanges, 10)
		off := client.Connection.OnAll(stateChange.Receive)
		defer off()

		client.Connect()

		var change ably.ConnectionStateChange

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateConnecting, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		<-waitTillConnecting

		client.Close()

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateClosing, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

		interrupt <- nil // continue to connected

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		if expected, got := ably.ConnectionStateClosed, change.Current; expected != got {
			t.Fatalf("expected %v; got %v (event: %+v)", expected, got, change)
		}

	})
}

func TestRealtimeConn_RTN15a_ReconnectOnEOF(t *testing.T) {
	t.Parallel()

	doEOF := make(chan struct{}, 1)

	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			c, err := ablyutil.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	channel := client.Channels.Get("channel")

	if err := channel.Attach(context.Background()); err != nil {
		t.Fatal(err)
	}

	sub, unsub, err := ablytest.ReceiveMessages(channel, "")
	if err != nil {
		t.Fatal(err)
	}
	defer unsub()

	stateChanges := make(chan ably.ConnectionStateChange, 16)
	client.Connection.OnAll(func(c ably.ConnectionStateChange) {
		stateChanges <- c
	})

	doEOF <- struct{}{}

	var state ably.ConnectionStateChange

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't transition on EOF")
	}

	if expected, got := ably.ConnectionStateDisconnected, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}

	// Publish a message to the channel through REST. If connection recovery
	// succeeds, we should then receive it without reattaching.

	rest, err := ably.NewREST(app.Options()...)
	if err != nil {
		t.Fatal(err)
	}
	err = rest.Channels.Get("channel").Publish(context.Background(), "name", "data")
	if err != nil {
		t.Fatal(err)
	}

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't reconnect")
	}

	if expected, got := ably.ConnectionStateConnecting, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}

	select {
	case state = <-stateChanges:
	case <-time.After(ablytest.Timeout):
		t.Fatal("didn't transition from CONNECTING")
	}

	if expected, got := ably.ConnectionStateConnected, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}

	select {
	case msg := <-sub:
		if expected, got := "data", msg.Data; expected != got {
			t.Fatalf("expected message with data %v, got %v", expected, got)
		}
	case <-time.After(ablytest.Timeout):
		t.Fatal("expected message after connection recovery; got none")
	}
}

type protoConnWithFakeEOF struct {
	proto.Conn
	doEOF     <-chan struct{}
	onMessage func(msg *proto.ProtocolMessage)
}

func (c protoConnWithFakeEOF) Receive(deadline time.Time) (*proto.ProtocolMessage, error) {
	type result struct {
		msg *proto.ProtocolMessage
		err error
	}

	received := make(chan result, 1)

	go func() {
		msg, err := c.Conn.Receive(deadline)
		received <- result{msg: msg, err: err}
	}()

	select {
	case r := <-received:
		if c.onMessage != nil {
			c.onMessage(r.msg)
		}
		return r.msg, r.err
	case <-c.doEOF:
		return nil, io.EOF
	}
}

func TestRealtimeConn_RTN15b(t *testing.T) {
	t.Parallel()

	doEOF := make(chan struct{}, 1)

	type meta struct {
		dial     *url.URL
		messages []*proto.ProtocolMessage
	}

	var metaList []*meta
	gotDial := make(chan chan struct{})
	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			m := &meta{dial: u}
			metaList = append(metaList, m)
			if len(metaList) > 1 {
				goOn := make(chan struct{})
				gotDial <- goOn
				<-goOn
			}
			c, err := ablyutil.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF, onMessage: func(msg *proto.ProtocolMessage) {
				m.messages = append(m.messages, msg)
			}}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	channel := client.Channels.Get("channel")

	if err := channel.Attach(context.Background()); err != nil {
		t.Fatal(err)
	}

	stateChanges := make(chan ably.ConnectionStateChange, 16)
	client.Connection.OnAll(func(c ably.ConnectionStateChange) {
		stateChanges <- c
	})

	doEOF <- struct{}{}

	var state ably.ConnectionStateChange

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't transition on EOF")
	}

	if expected, got := ably.ConnectionStateDisconnected, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}

	// Publish a message to the channel through REST. If connection recovery
	// succeeds, we should then receive it without reattaching.

	rest, err := ably.NewREST(app.Options()...)
	if err != nil {
		t.Fatal(err)
	}
	goOn := <-gotDial
	err = rest.Channels.Get("channel").Publish(context.Background(), "name", "data")
	if err != nil {
		t.Fatal(err)
	}
	close(goOn)

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't reconnect")
	}

	if expected, got := ably.ConnectionStateConnecting, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}

	select {
	case state = <-stateChanges:
	case <-time.After(ablytest.Timeout):
		t.Fatal("didn't transition from CONNECTING")
	}

	if expected, got := ably.ConnectionStateConnected, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}
	if len(metaList) != 2 {
		t.Fatalf("expected 2 connection dialing got %d", len(metaList))
	}

	{ //(RTN15b1)
		u := metaList[1].dial
		resume := u.Query().Get("resume")
		connKey := recent(metaList[0].messages, proto.ActionConnected).ConnectionDetails.ConnectionKey
		if resume == "" {
			t.Fatal("expected resume query param to be set")
		}
		if resume != connKey {
			t.Errorf("resume: expected %q got %q", connKey, resume)
		}
	}

	{ //(RTN15b2)
		u := metaList[1].dial
		serial := u.Query().Get("connectionSerial")
		connSerial := fmt.Sprint(metaList[0].messages[0].ConnectionSerial)
		if serial == "" {
			t.Fatal("expected connectionSerial query param to be set")
		}
		if serial != connSerial {
			t.Errorf("connectionSerial: expected %q got %q", connSerial, serial)
		}
	}
}

func recent(msgs []*proto.ProtocolMessage, action proto.Action) *proto.ProtocolMessage {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Action == action {
			return msgs[i]
		}
	}
	return nil
}

func TestRealtimeConn_RTN15c1(t *testing.T) {
	t.Parallel()

	doEOF := make(chan struct{}, 1)

	type meta struct {
		dial     *url.URL
		messages []*proto.ProtocolMessage
	}

	var metaList []*meta
	gotDial := make(chan chan struct{})

	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			m := &meta{dial: u}
			metaList = append(metaList, m)
			if len(metaList) > 1 {
				goOn := make(chan struct{})
				gotDial <- goOn
				<-goOn
			}
			c, err := ablyutil.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF, onMessage: func(msg *proto.ProtocolMessage) {
				m.messages = append(m.messages, msg)
			}}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	channel := client.Channels.Get("channel")
	if err := channel.Attach(context.Background()); err != nil {
		t.Fatal(err)
	}

	sub, unsub, err := ablytest.ReceiveMessages(channel, "")
	if err != nil {
		t.Fatal(err)
	}
	defer unsub()

	chanStateChanges := make(ably.ChannelStateChanges)
	off := channel.OnAll(chanStateChanges.Receive)
	defer off()

	stateChanges := make(chan ably.ConnectionStateChange, 16)
	client.Connection.OnAll(func(c ably.ConnectionStateChange) {
		stateChanges <- c
	})

	doEOF <- struct{}{}

	var state ably.ConnectionStateChange

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't transition on EOF")
	}
	if expected, got := ably.ConnectionStateDisconnected, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}
	rest, err := ably.NewREST(app.Options()...)
	if err != nil {
		t.Fatal(err)
	}
	goOn := <-gotDial
	err = rest.Channels.Get("channel").Publish(context.Background(), "name", "data")
	if err != nil {
		t.Fatal(err)
	}
	close(goOn)

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't reconnect")
	}
	if expected, got := ably.ConnectionStateConnecting, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}
	select {
	case msg := <-sub:
		if expected, got := "data", msg.Data; expected != got {
			t.Fatalf("expected message with data %v, got %v", expected, got)
		}
	case <-time.After(ablytest.Timeout):
		t.Fatal("expected message after connection recovery; got none")
	}

	// (RTN15c1)
	//
	// - current connectionId == resume message connectionId
	// - resume message has no error
	// - no channel state changes happened.

	var change ably.ChannelStateChange
	ablytest.Soon.Recv(t, &change, chanStateChanges, t.Fatalf)
	if change.Previous != change.Current {
		t.Errorf("expected no state change; got %+v", change)
	}
	if client.Connection.ID() != metaList[1].messages[0].ConnectionID {
		t.Errorf("expected %q to equal %q", client.Connection.ID(), metaList[1].messages[0].ConnectionID)
	}
	if metaList[1].messages[0].Error != nil {
		t.Error("expected resume error to be nil")
	}
}

func TestRealtimeConn_RTN15c2(t *testing.T) {
	t.Parallel()

	doEOF := make(chan struct{}, 1)

	type meta struct {
		dial     *url.URL
		messages []*proto.ProtocolMessage
	}

	var metaList []*meta
	errInfo := &proto.ErrorInfo{
		StatusCode: 401,
	}

	gotDial := make(chan chan struct{})
	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			m := &meta{dial: u}
			metaList = append(metaList, m)
			if len(metaList) > 1 {
				goOn := make(chan struct{})
				gotDial <- goOn
				<-goOn
			}
			c, err := ablyutil.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF, onMessage: func(msg *proto.ProtocolMessage) {
				if len(metaList) == 2 && len(m.messages) == 0 {
					msg.Error = errInfo
				}
				m.messages = append(m.messages, msg)
			}}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	channel := client.Channels.Get("channel")
	if err := channel.Attach(context.Background()); err != nil {
		t.Fatal(err)
	}
	chanStateChanges := make(ably.ChannelStateChanges)
	off := channel.OnAll(chanStateChanges.Receive)
	defer off()

	sub, unsub, err := ablytest.ReceiveMessages(channel, "")
	if err != nil {
		t.Fatal(err)
	}
	defer unsub()

	stateChanges := make(chan ably.ConnectionStateChange, 16)
	client.Connection.OnAll(func(c ably.ConnectionStateChange) {
		stateChanges <- c
	})

	doEOF <- struct{}{}

	var state ably.ConnectionStateChange

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't transition on EOF")
	}
	if expected, got := ably.ConnectionStateDisconnected, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}
	rest, err := ably.NewREST(app.Options()...)
	if err != nil {
		t.Fatal(err)
	}
	goOn := <-gotDial
	err = rest.Channels.Get("channel").Publish(context.Background(), "name", "data")
	if err != nil {
		t.Fatal(err)
	}
	close(goOn)

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't reconnect")
	}
	if expected, got := ably.ConnectionStateConnecting, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}

	select {
	case msg := <-sub:
		if expected, got := "data", msg.Data; expected != got {
			t.Fatalf("expected message with data %v, got %v", expected, got)
		}
	case <-time.After(ablytest.Timeout):
		t.Fatal("expected message after connection recovery; got none")
	}

	// (RTN15c2)
	//
	// - current connectionId == resume message connectionId
	// - resume message has an error
	// - Conn.Reqson == message resume error
	// - no channel state changes happened.

	var change ably.ChannelStateChange
	ablytest.Soon.Recv(t, &change, chanStateChanges, t.Fatalf)
	if change.Previous != change.Current {
		t.Errorf("expected no state change; got %+v", change)
	}
	if client.Connection.ID() != metaList[1].messages[0].ConnectionID {
		t.Errorf("expected %q to equal %q", client.Connection.ID(), metaList[1].messages[0].ConnectionID)
	}
	if metaList[1].messages[0].Error == nil {
		t.Error("expected resume error")
	}
	err = client.Connection.ErrorReason()
	if err == nil {
		t.Fatal("expected reason to be set")
	}
	reason := err.(*ably.ErrorInfo)
	if reason.StatusCode != errInfo.StatusCode {
		t.Errorf("expected %d got %d", errInfo.StatusCode, reason.StatusCode)
	}
}

func TestRealtimeConn_RTN15c3_attached(t *testing.T) {
	t.Parallel()

	doEOF := make(chan struct{}, 1)

	type meta struct {
		dial     *url.URL
		messages []*proto.ProtocolMessage
	}

	var metaList []*meta
	errInfo := &proto.ErrorInfo{
		StatusCode: 401,
	}
	connID := "new-conn-id"
	gotDial := make(chan chan struct{})
	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			m := &meta{dial: u}
			metaList = append(metaList, m)
			if len(metaList) > 1 {
				goOn := make(chan struct{})
				gotDial <- goOn
				<-goOn
			}
			c, err := ablyutil.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF, onMessage: func(msg *proto.ProtocolMessage) {
				if len(metaList) == 2 && len(m.messages) == 0 {
					msg.Error = errInfo
					msg.ConnectionID = connID
				}
				m.messages = append(m.messages, msg)
			}}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	channel := client.Channels.Get("channel")
	if err := channel.Attach(context.Background()); err != nil {
		t.Fatal(err)
	}
	chanStateChanges := make(ably.ChannelStateChanges, 18)
	off := channel.On(ably.ChannelEventAttaching, chanStateChanges.Receive)
	defer off()

	stateChanges := make(chan ably.ConnectionStateChange, 16)
	client.Connection.OnAll(func(c ably.ConnectionStateChange) {
		stateChanges <- c
	})

	doEOF <- struct{}{}

	var state ably.ConnectionStateChange

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't transition on EOF")
	}
	if expected, got := ably.ConnectionStateDisconnected, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}
	rest, err := ably.NewREST(app.Options()...)
	if err != nil {
		t.Fatal(err)
	}
	goOn := <-gotDial
	err = rest.Channels.Get("channel").Publish(context.Background(), "name", "data")
	if err != nil {
		t.Fatal(err)
	}
	close(goOn)

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't reconnect")
	}
	if expected, got := ably.ConnectionStateConnecting, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}

	<-stateChanges

	var chanState ably.ChannelStateChange
	select {
	case chanState = <-chanStateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't change state")
	}
	// we are testing to make sure we have initiated a new attach for channels
	// in ATTACHED state.
	if expected, got := ably.ChannelStateAttaching, chanState; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}
	reason := client.Connection.ErrorReason()
	if reason == nil {
		t.Fatal("expected reason to be set")
	}
	if reason.StatusCode != errInfo.StatusCode {
		t.Errorf("expected %d got %d", errInfo.StatusCode, reason.StatusCode)
	}
}

func TestRealtimeConn_RTN15c3_attaching(t *testing.T) {
	t.Parallel()

	doEOF := make(chan struct{}, 1)

	type meta struct {
		dial     *url.URL
		messages []*proto.ProtocolMessage
	}

	var metaList []*meta
	errInfo := &proto.ErrorInfo{
		StatusCode: 401,
	}
	connID := "new-conn-id"
	gotDial := make(chan chan struct{})
	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			m := &meta{dial: u}
			metaList = append(metaList, m)
			if len(metaList) > 1 {
				goOn := make(chan struct{})
				gotDial <- goOn
				<-goOn
			}
			c, err := ablyutil.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF, onMessage: func(msg *proto.ProtocolMessage) {
				if len(metaList) == 2 && len(m.messages) == 0 {
					msg.Error = errInfo
					msg.ConnectionID = connID
				}
				if msg.Action == proto.ActionAttached {
					msg.Action = proto.ActionHeartbeat
				}
				m.messages = append(m.messages, msg)
			}}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	channel := client.Channels.Get("channel")
	attaching := make(ably.ChannelStateChanges, 1)
	off := channel.On(ably.ChannelEventAttaching, attaching.Receive)
	defer off()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		channel.Attach(ctx)
	}()

	ablytest.Soon.Recv(t, nil, attaching, t.Fatalf)

	stateChanges := make(chan ably.ConnectionStateChange, 16)
	client.Connection.OnAll(func(c ably.ConnectionStateChange) {
		stateChanges <- c
	})

	doEOF <- struct{}{}

	var state ably.ConnectionStateChange

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't transition on EOF")
	}
	if expected, got := ably.ConnectionStateDisconnected, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}
	rest, err := ably.NewREST(app.Options()...)
	if err != nil {
		t.Fatal(err)
	}
	goOn := <-gotDial
	err = rest.Channels.Get("channel").Publish(ctx, "name", "data")
	if err != nil {
		t.Fatal(err)
	}
	close(goOn)

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't reconnect")
	}
	if expected, got := ably.ConnectionStateConnecting, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}

	<-stateChanges

	// we are testing to make sure we have initiated a new attach for channels
	// in ATTACHING  state.
	if expected, got := ably.ChannelStateAttaching, channel.State(); expected != got {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}
	reason := client.Connection.ErrorReason()
	if reason == nil {
		t.Fatal("expected reason to be set")
	}
	if reason.StatusCode != errInfo.StatusCode {
		t.Errorf("expected %d got %d", errInfo.StatusCode, reason.StatusCode)
	}
}

func TestRealtimeConn_RTN15c4(t *testing.T) {
	t.Parallel()

	doEOF := make(chan struct{}, 1)

	type meta struct {
		dial     *url.URL
		messages []*proto.ProtocolMessage
	}

	var metaList []*meta
	errInfo := &proto.ErrorInfo{
		StatusCode: http.StatusBadRequest,
	}
	gotDial := make(chan chan struct{})
	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			m := &meta{dial: u}
			metaList = append(metaList, m)
			if len(metaList) > 1 {
				goOn := make(chan struct{})
				gotDial <- goOn
				<-goOn
			}
			c, err := ablyutil.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF, onMessage: func(msg *proto.ProtocolMessage) {
				if len(metaList) == 2 && len(m.messages) == 0 {
					msg.Action = proto.ActionError
					msg.Error = errInfo
				}
				m.messages = append(m.messages, msg)
			}}, err
		}))
	defer safeclose(t, &closeClient{Closer: ablytest.FullRealtimeCloser(client), skip: []int{http.StatusBadRequest}}, app)

	if err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	channel := client.Channels.Get("channel")
	if err := channel.Attach(context.Background()); err != nil {
		t.Fatal(err)
	}
	chanStateChanges := make(ably.ChannelStateChanges, 1)
	off := channel.On(ably.ChannelEventFailed, chanStateChanges.Receive)
	defer off()

	stateChanges := make(chan ably.ConnectionStateChange, 16)
	client.Connection.OnAll(func(c ably.ConnectionStateChange) {
		stateChanges <- c
	})

	doEOF <- struct{}{}

	var state ably.ConnectionStateChange

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't transition on EOF")
	}
	if expected, got := ably.ConnectionStateDisconnected, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}
	rest, err := ably.NewREST(app.Options()...)
	if err != nil {
		t.Fatal(err)
	}
	goOn := <-gotDial
	err = rest.Channels.Get("channel").Publish(context.Background(), "name", "data")
	if err != nil {
		t.Fatal(err)
	}
	close(goOn)

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't reconnect")
	}
	if expected, got := ably.ConnectionStateConnecting, state; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}
	<-stateChanges
	var chanState ably.ChannelStateChange
	select {
	case chanState = <-chanStateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't change state")
	}
	if expected, got := ably.ChannelStateFailed, chanState; expected != got.Current {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}
	reason := client.Connection.ErrorReason()
	if reason == nil {
		t.Fatal("expected reason to be set")
	}
	if reason.StatusCode != errInfo.StatusCode {
		t.Errorf("expected %d got %d", errInfo.StatusCode, reason.StatusCode)
	}
	// The client should transition to the FAILED state
	if expected, got := ably.ConnectionStateFailed, client.Connection.State(); expected != got {
		t.Fatalf("expected transition to %v, got %v", expected, got)
	}
}

func TestRealtimeConn_RTN15d_MessageRecovery(t *testing.T) {
	t.Parallel()

	doEOF := make(chan struct{}, 1)
	allowDial := make(chan struct{}, 1)

	allowDial <- struct{}{}

	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			<-allowDial
			c, err := ablyutil.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatal(err)
	}

	channel := client.Channels.Get("test")
	err := channel.Attach(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	sub, unsub, err := ablytest.ReceiveMessages(channel, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer unsub()

	if err := ablytest.Wait(ablytest.ConnWaiter(client, func() {
		doEOF <- struct{}{}
	}, ably.ConnectionEventDisconnected), nil); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}

	// While we're DISCONNECTED, publish a few messages to the channel through
	// REST. If we then successfully recover connection state, the channel will
	// still be attached and the messages will arrive.

	rest, err := ably.NewREST(app.Options()...)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		err := rest.Channels.Get("test").Publish(context.Background(), "test", fmt.Sprintf("msg %d", i))
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
	}

	// Now let the connection reconnect.

	if err := ablytest.Wait(ablytest.ConnWaiter(client, func() {
		allowDial <- struct{}{}
	}, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatal(err)
	}

	// And expect the messages in the same channel.
	for i := 0; i < 3; i++ {
		fatalf := ablytest.FmtFunc(t.Fatalf).Wrap(t, "%d: %s", i)

		var msg *ably.Message
		ablytest.Soon.Recv(t, &msg, sub, fatalf)

		if expected, got := fmt.Sprintf("msg %d", i), msg.Data; expected != got {
			fatalf("expected %v, got %v", expected, got)
		}
	}
}

func TestRealtimeConn_RTN15e_ConnKeyUpdatedOnReconnect(t *testing.T) {
	t.Parallel()

	doEOF := make(chan struct{}, 1)

	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			c, err := ablyutil.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatal(err)
	}

	key1 := client.Connection.Key()

	connected := make(chan struct{}, 1)
	off := client.Connection.Once(ably.ConnectionEventConnected, func(ably.ConnectionStateChange) {
		connected <- struct{}{}
	})
	defer off()

	// Break the connection to cause a reconnection.

	doEOF <- struct{}{}

	// Once reconnected, the key should be different to the old one.

	ablytest.Soon.Recv(t, nil, connected, t.Fatalf)
	key2 := client.Connection.Key()

	if key2 == "" {
		t.Fatalf("the new key is empty")
	}
	if key1 == key2 {
		t.Fatalf("expected new key %q to be different from first one", key1)
	}
}

func TestRealtimeConn_RTN15g_NewConnectionOnStateLost(t *testing.T) {
	t.Parallel()

	out := make(chan *proto.ProtocolMessage, 16)

	now, setNow := func() (func() time.Time, func(time.Time)) {
		now := time.Now()

		var mtx sync.Mutex
		return func() time.Time {
				mtx.Lock()
				defer mtx.Unlock()
				return now
			}, func(t time.Time) {
				mtx.Lock()
				defer mtx.Unlock()
				now = t
			}
	}()

	connDetails := proto.ConnectionDetails{
		ConnectionKey:      "foo",
		ConnectionStateTTL: proto.DurationFromMsecs(time.Minute * 2),
		MaxIdleInterval:    proto.DurationFromMsecs(time.Second),
	}

	dials := make(chan *url.URL, 1)
	connIDs := make(chan string, 1)
	var breakConn func()
	var in chan *proto.ProtocolMessage

	c, _ := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
		ably.WithNow(now),
		ably.WithDial(func(p string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			in = make(chan *proto.ProtocolMessage, 1)
			in <- &proto.ProtocolMessage{
				Action:            proto.ActionConnected,
				ConnectionID:      <-connIDs,
				ConnectionDetails: &connDetails,
			}
			breakConn = func() { close(in) }
			dials <- u
			return ablytest.MessagePipe(in, out)(p, u, timeout)
		}))

	connIDs <- "conn-1"
	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	if err != nil {
		t.Fatal(err)
	}

	ablytest.Instantly.Recv(t, nil, dials, t.Fatalf) // discard first URL; we're interested in reconnections

	// Get channels to ATTACHING, ATTACHED and DETACHED. (TODO: SUSPENDED)

	attaching := c.Channels.Get("attaching")
	_ = ablytest.ResultFunc.Go(func(ctx context.Context) error { return attaching.Attach(ctx) })
	if msg := <-out; msg.Action != proto.ActionAttach {
		t.Fatalf("expected ATTACH, got %v", msg)
	}

	attached := c.Channels.Get("attached")
	attachWaiter := ablytest.ResultFunc.Go(func(ctx context.Context) error { return attached.Attach(ctx) })
	if msg := <-out; msg.Action != proto.ActionAttach {
		t.Fatalf("expected ATTACH, got %v", msg)
	}
	in <- &proto.ProtocolMessage{
		Action:  proto.ActionAttached,
		Channel: "attached",
	}
	ablytest.Wait(attachWaiter, err)

	detached := c.Channels.Get("detached")
	attachWaiter = ablytest.ResultFunc.Go(func(ctx context.Context) error { return detached.Attach(ctx) })
	if msg := <-out; msg.Action != proto.ActionAttach {
		t.Fatalf("expected ATTACH, got %v", msg)
	}
	in <- &proto.ProtocolMessage{
		Action:  proto.ActionAttached,
		Channel: "detached",
	}
	ablytest.Wait(attachWaiter, err)
	detachWaiter := ablytest.ResultFunc.Go(func(ctx context.Context) error { return detached.Detach(ctx) })
	if msg := <-out; msg.Action != proto.ActionDetach {
		t.Fatalf("expected DETACH, got %v", msg)
	}
	in <- &proto.ProtocolMessage{
		Action:  proto.ActionDetached,
		Channel: "detached",
	}
	ablytest.Wait(detachWaiter, err)

	// Simulate a broken connection. Before that, set the current time to a point
	// in the future just before connectionStateTTL + maxIdleInterval. So we still
	// attempt a resume.

	discardStateTTL := time.Duration(connDetails.ConnectionStateTTL + connDetails.MaxIdleInterval)

	setNow(now().Add(discardStateTTL - 1))

	connected := make(chan struct{})
	c.Connection.Once(ably.ConnectionEventConnected, func(ably.ConnectionStateChange) {
		close(connected)
	})

	breakConn()

	connIDs <- "conn-1" // Same connection ID so the resume "succeeds".
	var dialed *url.URL
	ablytest.Instantly.Recv(t, &dialed, dials, t.Fatalf)
	if resume := dialed.Query().Get("resume"); resume == "" {
		t.Fatalf("expected a resume key; got %v", dialed)
	}

	// Now do the same, but past connectionStateTTL + maxIdleInterval. This
	// should make a fresh connection.

	ablytest.Instantly.Recv(t, nil, connected, t.Fatalf) // wait for CONNECTED before disconnecting again

	setNow(now().Add(discardStateTTL + 1))
	breakConn()

	connIDs <- "conn-2"
	ablytest.Instantly.Recv(t, &dialed, dials, t.Fatalf)
	if resume := dialed.Query().Get("resume"); resume != "" {
		t.Fatalf("didn't expect a resume key; got %v", dialed)
	}
	if recover := dialed.Query().Get("recover"); recover != "" {
		t.Fatalf("didn't expect a recover key; got %v", dialed)
	}

	// RTN15g3: Expect the previously attaching and attached channels to be
	// attached again.
	attachExpected := map[string]struct{}{
		"attaching": {},
		"attached":  {},
	}
	for len(attachExpected) > 0 {
		var msg *proto.ProtocolMessage
		ablytest.Instantly.Recv(t, &msg, out, t.Fatalf)
		_, ok := attachExpected[msg.Channel]
		if !ok {
			t.Fatalf("ATTACH sent for unexpected or already attaching channel %q", msg.Channel)
		}
		delete(attachExpected, msg.Channel)
	}
	ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)
}

func TestRealtimeConn_RTN15h1_OnDisconnectedCannotRenewToken(t *testing.T) {
	t.Parallel()

	in := make(chan *proto.ProtocolMessage, 1)
	out := make(chan *proto.ProtocolMessage, 16)

	c, _ := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
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

	tokenErr := proto.ErrorInfo{
		StatusCode: 401,
		Code:       40141,
		Message:    "fake token error",
	}

	err = ablytest.Wait(ablytest.ConnWaiter(c, func() {
		in <- &proto.ProtocolMessage{
			Action: proto.ActionDisconnected,
			Error:  &tokenErr,
		}
	}, ably.ConnectionEventFailed), nil)

	var errInfo *ably.ErrorInfo
	if !errors.As(err, &errInfo) {
		t.Fatal(err)
	}

	if errInfo.StatusCode != tokenErr.StatusCode || int(errInfo.Code) != tokenErr.Code {
		t.Fatalf("expected the token error as FAILED state change reason; got: %s", err)
	}
}

func TestRealtimeConn_RTN15h2_ReauthFails(t *testing.T) {
	t.Parallel()

	authErr := fmt.Errorf("reauth error")

	in := make(chan *proto.ProtocolMessage, 1)
	out := make(chan *proto.ProtocolMessage, 16)

	authCallbackCalled := false

	c, _ := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
		ably.WithAuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
			if authCallbackCalled {
				t.Errorf("expected a single attempt to reauth")
			}
			authCallbackCalled = true
			return nil, authErr
		}),
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

	tokenErr := proto.ErrorInfo{
		StatusCode: 401,
		Code:       40141,
		Message:    "fake token error",
	}

	err = ablytest.Wait(ablytest.ConnWaiter(c, func() {
		in <- &proto.ProtocolMessage{
			Action: proto.ActionDisconnected,
			Error:  &tokenErr,
		}
	}, ably.ConnectionEventDisconnected), nil)

	if !errors.Is(err, authErr) {
		t.Fatalf("expected the auth error as DISCONNECTED state change reason; got: %s", err)
	}
}

func TestRealtimeConn_RTN15h2_ReauthWithBadToken(t *testing.T) {
	t.Parallel()

	in := make(chan *proto.ProtocolMessage, 1)
	out := make(chan *proto.ProtocolMessage, 16)

	dials := make(chan *url.URL, 1)

	c, _ := ably.NewRealtime(
		ably.WithToken("fake:token"),
		ably.WithAutoConnect(false),
		ably.WithAuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
			return ably.TokenString("bad:token"), nil
		}),
		ably.WithDial(func(proto string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			dials <- u
			return ablytest.MessagePipe(in, out)(proto, u, timeout)
		}))

	in <- &proto.ProtocolMessage{
		Action:            proto.ActionConnected,
		ConnectionID:      "connection-id",
		ConnectionDetails: &proto.ConnectionDetails{},
	}

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	if err != nil {
		t.Fatal(err)
	}

	ablytest.Instantly.Recv(t, nil, dials, t.Fatalf)

	tokenErr := proto.ErrorInfo{
		StatusCode: 401,
		Code:       40141,
		Message:    "fake token error",
	}

	stateChanges := make(chan ably.ConnectionStateChange, 1)

	off := c.Connection.OnAll(func(change ably.ConnectionStateChange) {
		stateChanges <- change
	})
	defer off()

	// Remove the buffer from dials, so that we can make the library wait for
	// us to receive it before a state change.
	dials = make(chan *url.URL)

	in <- &proto.ProtocolMessage{
		Action: proto.ActionDisconnected,
		Error:  &tokenErr,
	}

	// No state change expected before a reauthorization and reconnection
	// attempt.
	ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)

	// The DISCONNECTED causes a reauth, and dial again with the new
	// token.
	var dialURL *url.URL
	ablytest.Instantly.Recv(t, &dialURL, dials, t.Fatalf)

	if expected, got := "bad:token", dialURL.Query().Get("access_token"); expected != got {
		t.Errorf("expected reauthorization with token returned by the authCallback; got %q", got)
	}

	// After a token error response, we finally get to our expected
	// DISCONNECTED state.

	in <- &proto.ProtocolMessage{
		Action: proto.ActionError,
		Error:  &tokenErr,
	}

	var change ably.ConnectionStateChange
	ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)

	if change.Current != ably.ConnectionStateDisconnected {
		t.Fatalf("expected DISCONNECTED event; got %v", change)
	}

	if change.Reason.StatusCode != tokenErr.StatusCode || int(change.Reason.Code) != tokenErr.Code {
		t.Fatalf("expected the token error as FAILED state change reason; got: %s", change.Reason)
	}
}

func TestRealtimeConn_RTN15h2_Success(t *testing.T) {
	t.Parallel()

	in := make(chan *proto.ProtocolMessage, 1)
	out := make(chan *proto.ProtocolMessage, 16)

	dials := make(chan *url.URL, 1)

	c, _ := ably.NewRealtime(
		ably.WithToken("fake:token"),
		ably.WithAutoConnect(false),
		ably.WithAuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
			return ably.TokenString("good:token"), nil
		}),
		ably.WithDial(func(proto string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			dials <- u
			return ablytest.MessagePipe(in, out)(proto, u, timeout)
		}))

	in <- &proto.ProtocolMessage{
		Action:            proto.ActionConnected,
		ConnectionID:      "connection-id",
		ConnectionDetails: &proto.ConnectionDetails{},
	}

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	if err != nil {
		t.Fatal(err)
	}

	ablytest.Instantly.Recv(t, nil, dials, t.Fatalf)

	tokenErr := proto.ErrorInfo{
		StatusCode: 401,
		Code:       40141,
		Message:    "fake token error",
	}

	stateChanges := make(chan ably.ConnectionStateChange, 1)

	off := c.Connection.OnAll(func(change ably.ConnectionStateChange) {
		stateChanges <- change
	})
	defer off()

	in <- &proto.ProtocolMessage{
		Action: proto.ActionDisconnected,
		Error:  &tokenErr,
	}

	// The DISCONNECTED causes a reauth, and dial again with the new
	// token.
	var dialURL *url.URL
	ablytest.Instantly.Recv(t, &dialURL, dials, t.Fatalf)

	if expected, got := "good:token", dialURL.Query().Get("access_token"); expected != got {
		t.Errorf("expected reauthorization with token returned by the authCallback; got %q", got)
	}

	// Simulate a successful reconnection.
	in <- &proto.ProtocolMessage{
		Action:            proto.ActionConnected,
		ConnectionID:      "new-connection-id",
		ConnectionDetails: &proto.ConnectionDetails{},
	}

	// Expect a UPDATED event.

	var change ably.ConnectionStateChange
	ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)

	if change.Event != ably.ConnectionEventUpdate {
		t.Fatalf("expected UPDATED event; got %v", change)
	}

	// Expect no further events.break
	ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)
}

func TestRealtimeConn_RTN15i_OnErrorWhenConnected(t *testing.T) {
	t.Parallel()

	in := make(chan *proto.ProtocolMessage, 1)
	out := make(chan *proto.ProtocolMessage, 16)

	c, _ := ably.NewRealtime(
		ably.WithToken("fake:token"),
		ably.WithAutoConnect(false),
		ably.WithDial(ablytest.MessagePipe(in, out)),
	)

	// Get the connection to CONNECTED.

	in <- &proto.ProtocolMessage{
		Action:            proto.ActionConnected,
		ConnectionID:      "connection-id",
		ConnectionDetails: &proto.ConnectionDetails{},
	}

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Get a channel to ATTACHED.

	channel := c.Channels.Get("test")
	attachWaiter := ablytest.ResultFunc.Go(func(ctx context.Context) error { return channel.Attach(ctx) })

	_ = <-out // consume ATTACH

	in <- &proto.ProtocolMessage{
		Action:  proto.ActionAttached,
		Channel: "test",
	}

	ablytest.Wait(attachWaiter, err)

	// Send a fake ERROR; expect a transition to FAILED.

	errorCode := 50123

	channelFailed := make(ably.ChannelStateChanges, 1)
	off := channel.On(ably.ChannelEventFailed, channelFailed.Receive)
	defer off()

	err = ablytest.Wait(ablytest.ConnWaiter(c, func() {
		in <- &proto.ProtocolMessage{
			Action: proto.ActionError,
			Error: &proto.ErrorInfo{
				StatusCode: 500,
				Code:       errorCode,
				Message:    "fake error",
			},
		}
	}, ably.ConnectionEventFailed), nil)

	var errorInfo *ably.ErrorInfo
	if !errors.As(err, &errorInfo) {
		t.Fatal(err)
	}
	if expected, got := errorCode, int(errorInfo.Code); expected != got {
		t.Fatalf("expected error code %d; got %v", expected, errorInfo)
	}

	if expected, got := errorInfo, c.Connection.ErrorReason(); got == nil || *expected != *got {
		t.Fatalf("expected %v; got %v", expected, got)
	}

	// The channel should be moved to FAILED too.

	ablytest.Instantly.Recv(t, nil, channelFailed, t.Fatalf)

	if expected, got := ably.ChannelStateFailed, channel.State(); expected != got {
		t.Fatalf("expected channel in state %v; got %v", expected, got)
	}
}

func TestRealtimeConn_RTN16(t *testing.T) {
	t.Parallel()
	app, c := ablytest.NewRealtime()
	defer safeclose(t, ablytest.FullRealtimeCloser(c), app)

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	if err != nil {
		t.Fatal(err)
	}
	channel := c.Channels.Get("channel")
	if err := channel.Attach(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := channel.Publish(context.Background(), "name", "data"); err != nil {
		t.Fatal(err)
	}
	prevMsgSerial := c.Connection.MsgSerial()

	client := app.NewRealtime(
		ably.WithRecover(c.Connection.RecoveryKey()),
	)
	defer safeclose(t, ablytest.FullRealtimeCloser(client))

	err = ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	if err != nil {
		t.Fatal(err)
	}
	{ //RTN16b, RTN16f
		if !sameConnection(client.Connection.Key(), c.Connection.Key()) {
			t.Errorf("expected the same connection")
		}

		if expected, got := prevMsgSerial, client.Connection.MsgSerial(); expected != got {
			t.Errorf("expected %d got %d", expected, got)
		}
	}

	{ //(RTN16c)
		err := ablytest.Wait(ablytest.ConnWaiter(client, client.Close, ably.ConnectionEventClosed), nil)
		if err != nil {
			t.Fatal(err)
		}

		if key := client.Connection.Key(); key != "" {
			t.Errorf("expected key to be empty got %q instead", key)
		}

		if recover := client.Connection.RecoveryKey(); recover != "" {
			t.Errorf("expected recovery key to be empty got %q instead", recover)
		}
		if id := client.Connection.ID(); id != "" {
			t.Errorf("expected id to be empty got %q instead", id)
		}
	}
	{ //(RTN16e)
		// This test was adopted from the ably-js project
		// https://github.com/ably/ably-js/blob/340e5ce31dc9d7434a06ae4e1eec32bdacc9c6c5/spec/realtime/connection.test.js#L119
		var query url.Values
		fakeRecoveryKey := "_____!ablygo_test_fake-key____:5:3"
		client2 := app.NewRealtime(
			ably.WithRecover(fakeRecoveryKey),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
				query = u.Query()
				return ablyutil.DialWebsocket(protocol, u, timeout)
			}))
		defer safeclose(t, ablytest.FullRealtimeCloser(client2))
		err = ablytest.Wait(ablytest.ConnWaiter(client2, client2.Connect, ably.ConnectionEventConnected), nil)
		if err == nil {
			t.Fatal("expected reason to be set")
		}
		{ // (RTN16a)
			recover := query.Get("recover")
			if recover == "" {
				t.Fatal("expected resume query param to be set")
			}
			parts := strings.Split(fakeRecoveryKey, ":")
			if recover != parts[0] {
				t.Errorf("resume: expected %q got %q", parts[0], recover)
			}
			serial := query.Get("connectionSerial")
			if serial == "" {
				t.Fatal("expected connectionSerial query param to be set")
			}
			if serial != parts[1] {
				t.Errorf("connectionSerial: expected %q got %q", parts[1], serial)
			}
		}
		{ //(RTN16e)
			info := err.(*ably.ErrorInfo)
			code := 80008
			if int(info.Code) != code {
				// verify unrecoverable-connection error set in stateChange.reason
				t.Errorf("expected 80000 got %d", info.Code)
			}
			reason := client2.Connection.ErrorReason()
			if int(reason.Code) != code {
				// verify unrecoverable-connection error set in connection.errorReason
				t.Errorf("expected 80000 got %d", reason.Code)
			}
			if serial := client2.Connection.Serial(); *serial != -1 {
				// verify serial is -1 (new connection), not 5
				t.Errorf("expected -1 got %d", serial)
			}
			if serial := client2.Connection.MsgSerial(); serial != 0 {
				// (RTN16f)
				// verify msgSerial is 0 (new connection), not 3
				t.Errorf("expected 0 got %d", serial)
			}
			fake := "ablygo_test_fake"
			if key := client2.Connection.Key(); strings.Contains(key, fake) {
				// verify connection using a new connectionkey
				t.Errorf("expected %q not to contain %q", key, fake)
			}
		}
	}
}

func sameConnection(a, b string) bool {
	return strings.Split(a, "-")[0] == strings.Split(b, "-")[0]
}

func TestRealtimeConn_RTN23(t *testing.T) {
	t.Parallel()

	connDetails := proto.ConnectionDetails{
		ConnectionKey:      "foo",
		ConnectionStateTTL: proto.DurationFromMsecs(time.Minute * 20),
		MaxIdleInterval:    proto.DurationFromMsecs(time.Minute * 5),
	}

	afterCalls := make(chan ablytest.AfterCall)
	now, after := ablytest.TimeFuncs(afterCalls)

	dials := make(chan *url.URL, 1)
	var in chan *proto.ProtocolMessage
	realtimeRequestTimeout := time.Minute
	c, _ := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
		ably.WithRealtimeRequestTimeout(realtimeRequestTimeout),
		ably.WithNow(now),
		ably.WithAfter(after),
		ably.WithDial(func(p string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			in = make(chan *proto.ProtocolMessage, 1)
			in <- &proto.ProtocolMessage{
				Action:            proto.ActionConnected,
				ConnectionID:      "connection",
				ConnectionDetails: &connDetails,
			}
			dials <- u
			return ablytest.MessagePipe(in, nil,
				ablytest.MessagePipeWithNowFunc(now),
				ablytest.MessagePipeWithAfterFunc(after),
			)(p, u, timeout)
		}))
	disconnected := make(chan *ably.ErrorInfo, 1)
	c.Connection.Once(ably.ConnectionEventDisconnected, func(e ably.ConnectionStateChange) {
		disconnected <- e.Reason
	})
	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	if err != nil {
		t.Fatal(err)
	}

	var dialed *url.URL
	ablytest.Instantly.Recv(t, &dialed, dials, t.Fatalf)
	// RTN23b
	h := dialed.Query().Get("heartbeats")
	if h != "true" {
		t.Errorf("expected heartbeats query param to be true got %q", h)
	}

	maxIdleInterval := time.Duration(connDetails.MaxIdleInterval)
	receiveTimeout := realtimeRequestTimeout + maxIdleInterval

	// Expect a timer for a message receive.
	var timer ablytest.AfterCall
	ablytest.Instantly.Recv(t, &timer, afterCalls, t.Fatalf)
	if expected, got := receiveTimeout, timer.D; expected != got {
		t.Fatalf("expected %v, got %v", expected, got)
	}

	in <- &proto.ProtocolMessage{
		Action: proto.ActionHeartbeat,
	}

	// An incoming message should cancel the timer and prevent a disconnection.
	ablytest.Instantly.Recv(t, nil, timer.Ctx.Done(), t.Fatalf)
	ablytest.Instantly.NoRecv(t, nil, disconnected, t.Fatalf)

	// Expect another timer for a message receive.
	ablytest.Instantly.Recv(t, &timer, afterCalls, t.Fatalf)
	if expected, got := receiveTimeout, timer.D; expected != got {
		t.Fatalf("expected %v, got %v", expected, got)
	}

	// Let the deadline pass without a message; expect a disconnection.
	timer.Fire()

	// RTN23a The connection should be disconnected due to lack of activity past
	// receiveTimeout
	var reason *ably.ErrorInfo
	ablytest.Instantly.Recv(t, &reason, disconnected, t.Fatalf)

	// make sure the reason is timeout
	if !strings.Contains(reason.Error(), "timeout") {
		t.Errorf("expected %q to contain timeout", reason.Error())
	}
}

type writerLogger struct {
	w io.Writer
}

func (w *writerLogger) Printf(level ably.LogLevel, format string, v ...interface{}) {
	fmt.Fprintf(w.w, format, v...)
}

func TestRealtimeConn_RTN14c_ConnectedTimeout(t *testing.T) {
	t.Parallel()

	afterCalls := make(chan ablytest.AfterCall)
	now, after := ablytest.TimeFuncs(afterCalls)

	in := make(chan *proto.ProtocolMessage, 10)
	out := make(chan *proto.ProtocolMessage, 10)

	c, err := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
		ably.WithNow(now),
		ably.WithAfter(after),
		ably.WithDial(ablytest.MessagePipe(in, out,
			ablytest.MessagePipeWithNowFunc(now),
			ablytest.MessagePipeWithAfterFunc(after),
		)),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	stateChanges := make(ably.ConnStateChanges, 10)
	off := c.Connection.OnAll(stateChanges.Receive)
	defer off()

	c.Connect()

	var change ably.ConnectionStateChange
	ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
	if expected, got := connecting, change.Current; expected != got {
		t.Fatalf("expected %v; got %v", expected, got)
	}

	// Once dialed, expect a timer for realtimeRequestTimeout.

	const realtimeRequestTimeout = 10 * time.Second // DF1b

	var receiveTimer ablytest.AfterCall
	ablytest.Instantly.Recv(t, &receiveTimer, afterCalls, t.Fatalf)

	if expected, got := realtimeRequestTimeout, receiveTimer.D; expected != got {
		t.Fatalf("expected %v; got %v", expected, got)
	}

	// Expect a state change to DISCONNECTED when the timer expires.
	ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)

	receiveTimer.Fire()

	ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
	if expected, got := disconnected, change.Current; expected != got {
		t.Fatalf("expected %v; got %v", expected, got)
	}
}

func TestRealtimeConn_RTN14a(t *testing.T) {
	t.Skip(`
	Currently its impossible to implement/test this.
	See https://github.com/ably/docs/issues/984 for details
	`)
}

func TestRealtimeConn_RTN14b(t *testing.T) {
	t.Parallel()
	t.Run("renewable token that fails to renew with token error", func(t *testing.T) {
		t.Parallel()
		in := make(chan *proto.ProtocolMessage, 1)
		out := make(chan *proto.ProtocolMessage, 16)
		var reauth atomic.Value
		reauth.Store(int(0))
		bad := "bad token request"
		c, _ := ably.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithAuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
				reauth.Store(reauth.Load().(int) + 1)
				if reauth.Load().(int) > 1 {
					return nil, errors.New(bad)
				}
				return ably.TokenString("fake:token"), nil
			}),
			ably.WithDial(ablytest.MessagePipe(in, out)))
		// Get the connection to CONNECTED.
		tokenError := &proto.ErrorInfo{
			StatusCode: http.StatusUnauthorized,
			Code:       40140,
		}
		in <- &proto.ProtocolMessage{
			Action:            proto.ActionError,
			ConnectionDetails: &proto.ConnectionDetails{},
			Error:             tokenError,
		}
		change := make(ably.ConnStateChanges, 1)
		c.Connection.OnAll(change.Receive)
		c.Connect()
		var state ably.ConnectionStateChange
		ablytest.Instantly.Recv(t, nil, change, t.Fatalf) // Skip CONNECTING
		ablytest.Instantly.Recv(t, &state, change, t.Fatalf)
		if expect, got := ably.ConnectionStateDisconnected, state.Current; expect != got {
			t.Errorf("expected %v got %v", expect, got)
		}
		if expect, got := bad, c.Connection.ErrorReason(); !strings.Contains(got.Error(), expect) {
			t.Errorf("expected %v to contain %q", got, expect)
		}
		n := reauth.Load().(int) - 1 // we remove the first attempt
		if n != 1 {
			t.Errorf("expected re authorization to happen once but happened %d", n)
		}
	})
	t.Run("renewable token, consecutive token errors", func(t *testing.T) {
		t.Parallel()
		in := make(chan *proto.ProtocolMessage, 1)
		out := make(chan *proto.ProtocolMessage, 16)
		var reauth atomic.Value
		reauth.Store(int(0))
		var dials atomic.Value
		dials.Store(int(0))
		c, _ := ably.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithAuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
				reauth.Store(reauth.Load().(int) + 1)
				return ably.TokenString("fake:token"), nil
			}),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
				dials.Store(dials.Load().(int) + 1)
				return ablytest.MessagePipe(in, out)(protocol, u, timeout)
			}))
		// Get the connection to CONNECTED.
		tokenError := &proto.ErrorInfo{
			StatusCode: http.StatusUnauthorized,
			Code:       40140,
		}
		in <- &proto.ProtocolMessage{
			Action:            proto.ActionError,
			ConnectionDetails: &proto.ConnectionDetails{},
			Error:             tokenError,
		}
		change := make(ably.ConnStateChanges, 1)
		c.Connection.OnAll(change.Receive)
		c.Connect()
		bad := &proto.ErrorInfo{
			StatusCode: http.StatusUnauthorized,
			Code:       40140,
			Message:    "bad token request",
		}
		in <- &proto.ProtocolMessage{
			Action:            proto.ActionError,
			ConnectionDetails: &proto.ConnectionDetails{},
			Error:             bad,
		}
		var state ably.ConnectionStateChange
		ablytest.Instantly.Recv(t, nil, change, t.Fatalf) // skip CONNECTING
		ablytest.Instantly.Recv(t, &state, change, t.Fatalf)
		if expect, got := ably.ConnectionStateDisconnected, state.Current; expect != got {
			t.Errorf("expected %v got %v", expect, got)
		}
		if expect, got := bad.Message, c.Connection.ErrorReason(); !strings.Contains(got.Error(), expect) {
			t.Errorf("expected %v to contain %q", got, expect)
		}
		n := reauth.Load().(int)
		if n != 2 {
			t.Errorf("expected re authorization to happen twice got %d", n)
		}

		// We should only try try to reconnect once after token error.
		if expect, got := 2, dials.Load().(int); got != expect {
			t.Errorf("expected %v got %v", expect, got)
		}
	})

}

type closeConn struct {
	proto.Conn
	closed int
}

func (c *closeConn) Close() error {
	c.closed++
	return c.Conn.Close()
}

type noopConn struct {
	ch chan struct{}
}

func (noopConn) Send(*proto.ProtocolMessage) error {
	return nil
}

func (n *noopConn) Receive(deadline time.Time) (*proto.ProtocolMessage, error) {
	n.ch <- struct{}{}
	return &proto.ProtocolMessage{}, nil
}
func (noopConn) Close() error { return nil }

func TestRealtimeConn_RTN14g(t *testing.T) {
	t.Parallel()
	t.Run("Non RTN14b error", func(t *testing.T) {
		in := make(chan *proto.ProtocolMessage, 1)
		out := make(chan *proto.ProtocolMessage, 16)
		var ls *closeConn
		c, _ := ably.NewRealtime(
			ably.WithToken("fake:token"),
			ably.WithAutoConnect(false),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
				w, err := ablytest.MessagePipe(in, out)(protocol, u, timeout)
				if err != nil {
					return nil, err
				}
				ls = &closeConn{Conn: w}
				return ls, nil
			}))
		badReqErr := &proto.ErrorInfo{
			StatusCode: http.StatusBadRequest,
		}
		in <- &proto.ProtocolMessage{
			Action:            proto.ActionError,
			ConnectionDetails: &proto.ConnectionDetails{},
			Error:             badReqErr,
		}
		change := make(ably.ConnStateChanges, 1)
		c.Connection.OnAll(change.Receive)
		c.Connect()
		var state ably.ConnectionStateChange
		ablytest.Instantly.Recv(t, nil, change, t.Fatalf) // Skip CONNECTING
		ablytest.Instantly.Recv(t, &state, change, t.Fatalf)
		if expect, got := ably.ConnectionStateFailed, state.Current; expect != got {
			t.Errorf("expected %v got %v", expect, got)
		}
		if expect, got := badReqErr, c.Connection.ErrorReason(); got.StatusCode != expect.StatusCode {
			t.Errorf("expected %v got %v", expect, got)
		}
		// we make sure the connection is closed
		if expect, got := 1, ls.closed; got != expect {
			t.Errorf("expected %v got %v", expect, got)
		}
	})
}

func TestRealtimeConn_RTN14e(t *testing.T) {
	t.Parallel()
	ttl := 10 * time.Millisecond
	disconnTTL := ttl * 2
	suspendTTL := ttl / 2
	c, _ := ably.NewRealtime(
		ably.WithToken("fake:token"),
		ably.WithAutoConnect(false),
		ably.WithConnectionStateTTL(ttl),
		ably.WithSuspendedRetryTimeout(suspendTTL),
		ably.WithDisconnectedRetryTimeout(disconnTTL),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			return nil, context.DeadlineExceeded
		}))
	defer c.Close()
	changes := make(ably.ConnStateChanges, 2)
	off := c.Connection.On(ably.ConnectionEventSuspended, changes.Receive)
	defer off()
	c.Connect()
	var state ably.ConnectionStateChange
	ablytest.Soon.Recv(t, &state, changes, t.Fatalf)
	if state.RetryIn != suspendTTL {
		t.Fatalf("expected retry to be in %v got %v ", suspendTTL, state.RetryIn)
	}
	reason := fmt.Sprintf(ably.ConnectionStateTTLErrFmt, ttl)
	if !strings.Contains(state.Reason.Error(), reason) {
		t.Errorf("expected %v\n to contain %s", state.Reason, reason)
	}
	// make sure we are from DISCONNECTED => SUSPENDED
	if expect, got := ably.ConnectionStateDisconnected, state.Previous; got != expect {
		t.Fatalf("expected transitioning from %v got %v", expect, got)
	}
	state = ably.ConnectionStateChange{}
	ablytest.Soon.Recv(t, &state, changes, t.Fatalf)

	// in suspend retries we move from
	//  SUSPENDED => CONNECTING => SUSPENDED ...
	if expect, got := ably.ConnectionStateConnecting, state.Previous; got != expect {
		t.Fatalf("expected transitioning from %v got %v", expect, got)
	}
}

func TestRealtimeConn_RTN2g(t *testing.T) {
	t.Parallel()
	uri := make(chan url.URL, 1)
	_, err := ably.NewRealtime(
		ably.WithKey("xxx:xxx"),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			uri <- *u
			return nil, io.EOF
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	var connURL url.URL
	ablytest.Soon.Recv(t, &connURL, uri, t.Fatalf)
	lib := connURL.Query().Get("lib")
	if lib != proto.LibraryString {
		t.Errorf("expected %q got %q", proto.LibraryString, lib)
	}
}

func TestRealtimeConn_RTN19b(t *testing.T) {
	t.Parallel()
	connIDs := make(chan string)
	var breakConn func()
	var out, in chan *proto.ProtocolMessage
	c, err := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithKey("fake:key"),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			in = make(chan *proto.ProtocolMessage, 1)
			in <- &proto.ProtocolMessage{
				Action:       proto.ActionConnected,
				ConnectionID: <-connIDs,
				ConnectionDetails: &proto.ConnectionDetails{
					ConnectionKey: "key",
				},
			}
			out = make(chan *proto.ProtocolMessage, 16)
			breakConn = func() { close(in) }
			return ablytest.MessagePipe(in, out)(protocol, u, timeout)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	changes := make(ably.ConnStateChanges, 2)
	off := c.Connection.On(ably.ConnectionEventConnected, changes.Receive)
	defer off()
	c.Connect()
	connIDs <- "1"
	ablytest.Soon.Recv(t, nil, changes, t.Fatalf)
	attaching := c.Channels.Get("attaching")
	_ = ablytest.ResultFunc.Go(func(ctx context.Context) error { return attaching.Attach(ctx) })

	attachChanges := make(ably.ChannelStateChanges, 1)
	off = attaching.OnAll(attachChanges.Receive)
	defer off()
	ablytest.Soon.Recv(t, nil, attachChanges, t.Fatalf)

	detaching := c.Channels.Get("detaching")
	detachChange := make(ably.ChannelStateChanges, 1)
	off = detaching.OnAll(detachChange.Receive)
	defer off()

	wait := ablytest.ResultFunc.Go(func(ctx context.Context) error { return detaching.Attach(ctx) })
	var state ably.ChannelStateChange
	ablytest.Soon.Recv(t, &state, detachChange, t.Fatalf)
	if expect, got := ably.ChannelStateAttaching, state.Current; expect != got {
		t.Fatalf("expected %v got %v", expect, got)
	}
	in <- &proto.ProtocolMessage{
		Action:  proto.ActionAttached,
		Channel: "detaching",
	}
	ablytest.Wait(wait, nil)
	state = ably.ChannelStateChange{}
	ablytest.Soon.Recv(t, &state, detachChange, t.Fatalf)
	if expect, got := ably.ChannelStateAttached, state.Current; expect != got {
		t.Fatalf("expected %v got %v", expect, got)
	}

	_ = ablytest.ResultFunc.Go(func(ctx context.Context) error { return detaching.Detach(ctx) })
	state = ably.ChannelStateChange{}
	ablytest.Soon.Recv(t, &state, detachChange, t.Fatalf)
	if expect, got := ably.ChannelStateDetaching, state.Current; expect != got {
		t.Fatalf("expected %v got %v", expect, got)
	}

	msgs := []proto.ProtocolMessage{
		{
			Channel: "attaching",
			Action:  proto.ActionAttach,
		},
		{
			Channel: "detaching",
			Action:  proto.ActionAttach,
		},
		{
			Channel: "detaching",
			Action:  proto.ActionDetach,
		},
	}
	for _, expect := range msgs {
		var got *proto.ProtocolMessage
		ablytest.Instantly.Recv(t, &got, out, t.Fatalf)
		if expect.Action != got.Action {
			t.Errorf("expected %v got %v", expect.Action, got.Action)
		}
		if expect.Channel != got.Channel {
			t.Errorf("expected %v got %v", expect.Channel, got.Channel)
		}
	}
	breakConn()
	connIDs <- "2"
	ablytest.Soon.Recv(t, nil, changes, t.Fatalf)
	if c.Connection.ID() != "2" {
		t.Fatal("expected new connection")
	}
	msgs = []proto.ProtocolMessage{
		{
			Channel: "attaching",
			Action:  proto.ActionAttach,
		},
		{
			Channel: "detaching",
			Action:  proto.ActionDetach,
		},
	}
	for _, expect := range msgs {
		var got *proto.ProtocolMessage
		ablytest.Instantly.Recv(t, &got, out, t.Fatalf)
		if expect.Action != got.Action {
			t.Errorf("expected %v got %v", expect.Action, got.Action)
		}
		if expect.Channel != got.Channel {
			t.Errorf("expected %v got %v", expect.Channel, got.Channel)
		}
	}
}

func TestRealtimeConn_RTN19a(t *testing.T) {
	t.Parallel()
	connIDs := make(chan string)
	var breakConn func()
	var out, in chan *proto.ProtocolMessage
	c, err := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithKey("fake:key"),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			in = make(chan *proto.ProtocolMessage, 1)
			in <- &proto.ProtocolMessage{
				Action:       proto.ActionConnected,
				ConnectionID: <-connIDs,
				ConnectionDetails: &proto.ConnectionDetails{
					ConnectionKey: "key",
				},
			}
			out = make(chan *proto.ProtocolMessage, 16)
			breakConn = func() { close(in) }
			return ablytest.MessagePipe(in, out)(protocol, u, timeout)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	changes := make(ably.ConnStateChanges, 2)
	off := c.Connection.On(ably.ConnectionEventConnected, changes.Receive)
	defer off()
	c.Connect()
	connIDs <- "1"
	ablytest.Soon.Recv(t, nil, changes, t.Fatalf)

	name := "channel"
	channel := c.Channels.Get(name)
	chanChange := make(ably.ChannelStateChanges, 1)
	off = channel.OnAll(chanChange.Receive)
	defer off()

	wait := ablytest.ResultFunc.Go(func(ctx context.Context) error { return channel.Attach(ctx) })
	var state ably.ChannelStateChange
	ablytest.Soon.Recv(t, &state, chanChange, t.Fatalf)
	if expect, got := ably.ChannelStateAttaching, state.Current; got != expect {
		t.Fatalf("expected %v got %v", expect, got)
	}
	in <- &proto.ProtocolMessage{
		Action:  proto.ActionAttached,
		Channel: name,
	}
	ablytest.Wait(wait, nil)
	state = ably.ChannelStateChange{}
	ablytest.Soon.Recv(t, &state, chanChange, t.Fatalf)
	if expect, got := ably.ChannelStateAttached, state.Current; got != expect {
		t.Fatalf("expected %v got %v", expect, got)
	}
	ctx, cancel := context.WithTimeout(context.TODO(), time.Millisecond)
	defer cancel()
	err = channel.Publish(ctx, "ack", "ack")
	if err != nil {
		if err != context.DeadlineExceeded {
			t.Fatal(err)
		}
	}
	ablytest.Soon.Recv(t, nil, out, t.Fatalf) // attach

	var msg *proto.ProtocolMessage
	ablytest.Soon.Recv(t, &msg, out, t.Fatalf)
	if expect, got := proto.ActionMessage, msg.Action; got != expect {
		t.Fatalf("expected %v got %v", expect, got)
	}

	if expect, got := 1, c.Connection.PendingItems(); got != expect {
		t.Fatalf("expected %v got %v", expect, got)
	}
	breakConn()
	connIDs <- "2"
	ablytest.Soon.Recv(t, nil, changes, t.Fatalf)
	if expect, got := "2", c.Connection.ID(); got != expect {
		t.Fatalf("expected %v got %v", expect, got)
	}

	ablytest.Instantly.Recv(t, nil, out, t.Fatalf) // attach
	msg = nil
	ablytest.Instantly.Recv(t, &msg, out, t.Fatalf)
	if expect, got := proto.ActionMessage, msg.Action; got != expect {
		t.Fatalf("expected %v got %v", expect, got)
	}
	if expect, got := name, msg.Channel; got != expect {
		t.Fatalf("expected %v got %v", expect, got)
	}
	if expect, got := 1, c.Connection.PendingItems(); got != expect {
		t.Fatalf("expected %v got %v", expect, got)
	}
}

func TestRealtimeConn_RTN24_RTN21_RTC8a_RTN4h_Override_ConnectionDetails_On_Connected(t *testing.T) {
	t.Parallel()

	in := make(chan *proto.ProtocolMessage, 1)
	out := make(chan *proto.ProtocolMessage, 16)

	c, _ := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
		ably.WithDial(ablytest.MessagePipe(in, out)),
	)

	connDetails := proto.ConnectionDetails{
		ClientID:           "id1",
		ConnectionKey:      "foo",
		MaxFrameSize:       12,
		MaxInboundRate:     14,
		MaxMessageSize:     67,
		ConnectionStateTTL: proto.DurationFromMsecs(time.Minute * 2),
		MaxIdleInterval:    proto.DurationFromMsecs(time.Second),
	}

	in <- &proto.ProtocolMessage{
		Action:            proto.ActionConnected,
		ConnectionID:      "connection-id-1",
		ConnectionDetails: &connDetails,
	}

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	if err != nil {
		t.Fatal(err)
	}

	newConnDetails := proto.ConnectionDetails{
		ClientID:           "id2",
		ConnectionKey:      "bar",
		MaxFrameSize:       13,
		MaxInboundRate:     15,
		MaxMessageSize:     70,
		ConnectionStateTTL: proto.DurationFromMsecs(time.Minute * 3),
		MaxIdleInterval:    proto.DurationFromMsecs(time.Second),
	}

	errInfo := proto.ErrorInfo{
		StatusCode: 500,
		Code:       50500,
		Message:    "fake error",
	}

	changes := make(ably.ConnStateChanges, 3)
	off := c.Connection.OnAll(changes.Receive)
	defer off()

	//  Send new connection details
	in <- &proto.ProtocolMessage{
		Action:            proto.ActionConnected,
		ConnectionID:      "connection-id-2",
		ConnectionDetails: &newConnDetails,
		Error:             &errInfo,
	}

	var newConnectionState ably.ConnectionStateChange
	ablytest.Instantly.Recv(t, &newConnectionState, changes, t.Fatalf)

	// RTN4h - can emit UPDATE event
	if expected, got := ably.ConnectionEventUpdate, newConnectionState.Event; expected != got {
		t.Fatalf("expected %v; got %v (event: %+v)", expected, got, newConnectionState)
	}
	if expected, got := ably.ConnectionStateConnected, newConnectionState.Current; expected != got {
		t.Fatalf("expected %v; got %v (event: %+v)", expected, got, newConnectionState)
	}
	if expected, got := ably.ConnectionStateConnected, newConnectionState.Previous; expected != got {
		t.Fatalf("expected %v; got %v (event: %+v)", expected, got, newConnectionState)
	}
	if got := fmt.Sprint(newConnectionState.Reason); !strings.Contains(got, errInfo.Message) {
		t.Fatalf("expected %+v; got %v (error: %+v)", errInfo, got, newConnectionState.Reason)
	}
	if got := c.Connection.ErrorReason().Message(); !strings.Contains(got, errInfo.Message) {
		t.Fatalf("expected %+v; got %v (error: %+v)", errInfo, got, c.Connection.ErrorReason().Message())
	}
	// RTN21 - new connection details over write old values
	if c.Connection.Key() != newConnDetails.ConnectionKey {
		t.Fatalf("expected %v; got %v", newConnDetails.ConnectionKey, c.Connection.Key())
	}

	if c.Auth.ClientID() != newConnDetails.ClientID {
		t.Fatalf("expected %v; got %v", newConnDetails.ClientID, c.Auth.ClientID())
	}

	if c.Connection.ConnectionStateTTL() != time.Duration(newConnDetails.ConnectionStateTTL) {
		t.Fatalf("expected %v; got %v", newConnDetails.ConnectionStateTTL, c.Connection.ConnectionStateTTL())
	}

	if c.Connection.ID() != "connection-id-2" {
		t.Fatalf("expected %v; got %v", "connection-id-2", c.Connection.ID())
	}
}
