//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"
	"github.com/stretchr/testify/assert"
)

func Test_RTN2_WebsocketQueryParams(t *testing.T) {
	setup := func(options ...ably.ClientOption) (requestParams url.Values) {
		in := make(chan *ably.ProtocolMessage, 1)
		out := make(chan *ably.ProtocolMessage, 16)
		var urls []url.URL
		defaultOptions := []ably.ClientOption{
			ably.WithToken("fake:token"),
			ably.WithAutoConnect(false),
			ably.WithDial(func(proto string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				urls = append(urls, *u)
				return MessagePipe(in, out)(proto, u, timeout)
			}),
		}
		options = append(defaultOptions, options...)
		c, _ := ably.NewRealtime(options...)
		in <- &ably.ProtocolMessage{
			Action:            ably.ActionConnected,
			ConnectionID:      "connection-id",
			ConnectionDetails: &ably.ConnectionDetails{},
		}

		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		assert.NoError(t, err)
		requestParams = urls[0].Query()
		return
	}

	t.Run("RTN2a: format should be msgPack or json", func(t *testing.T) {
		requestParams := setup(ably.WithUseBinaryProtocol(false)) // default protocol is false
		protocol := requestParams["format"]
		assert.Equal(t, []string{"json"}, protocol)

		requestParams = setup(ably.WithUseBinaryProtocol(true))
		protocol = requestParams["format"]
		assert.Equal(t, []string{"msgpack"}, protocol)
	})

	t.Run("RTN2b: echo should be true by default", func(t *testing.T) {
		requestParams := setup() // default echo value is true
		echo := requestParams["echo"]
		assert.Equal(t, []string{"true"}, echo)

		requestParams = setup(ably.WithEchoMessages(false))
		echo = requestParams["echo"]
		assert.Equal(t, []string{"false"}, echo)
	})

	t.Run("RTN2d: clientId contains provided clientId", func(t *testing.T) {
		requestParams := setup()
		clientId := requestParams["clientId"]
		assert.Nil(t, clientId)

		// todo - Need to verify if clientId is only valid for Basic Auth Mode
		clientIdParam := "123"
		key := "fake:key"
		requestParams = setup(ably.WithToken(""), ably.WithKey(key), ably.WithClientID(clientIdParam)) // Client Id is only enabled for basic auth
		clientId = requestParams["clientId"]
		assert.Equal(t, []string{clientIdParam}, clientId)
	})

	t.Run("RTN2e: depending on the auth scheme, accessToken contains token string or key contains api key", func(t *testing.T) {
		token := "fake:clientToken"
		requestParams := setup(ably.WithToken(token))
		actualToken := requestParams["access_token"]
		assert.Equal(t, []string{token}, actualToken)

		key := "fake:key"
		requestParams = setup(ably.WithToken(""), ably.WithKey(key)) // disable token, use key instead
		actualKey := requestParams["key"]
		assert.Equal(t, []string{key}, actualKey)
	})

	t.Run("RTN2f: api version v should be the API version", func(t *testing.T) {
		requestParams := setup()
		libVersion := requestParams["v"]
		assert.Equal(t, []string{ably.AblyProtocolVersion}, libVersion)
	})
}

func Test_RTN3_ConnectionAutoConnect(t *testing.T) {

	recorder := NewMessageRecorder()

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
	assert.Equal(t, ably.ConnectionStateConnected, connectionChange.Current,
		"expected %v; got %v (event: %+v)", ably.ConnectionStateConnected, connectionChange.Current, connectionChange)
	ablytest.Instantly.NoRecv(t, nil, connectionStateChanges, t.Fatalf)
}

func Test_RTN4a_ConnectionEventForStateChange(t *testing.T) {
	t.Run(fmt.Sprintf("on %s", ably.ConnectionStateConnecting), func(t *testing.T) {

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

		app, realtime := ablytest.NewRealtime(ably.WithAutoConnect(false))
		defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

		connectAndWait(t, realtime)
	})

	t.Run(fmt.Sprintf("on %s", ably.ConnectionStateDisconnected), func(t *testing.T) {

		dial, disconnect := DialFakeDisconnect(nil)
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
		assert.NoError(t, err,
			"fake disconnection failed: %v", err)

		ablytest.Soon.Recv(t, nil, changes, t.Fatalf)

	})

	t.Run(fmt.Sprintf("on %s", ably.ConnectionStateSuspended), func(t *testing.T) {

		t.Skip("SUSPENDED not yet implemented")
	})

	t.Run(fmt.Sprintf("on %s", ably.ConnectionStateClosing), func(t *testing.T) {

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

		options := []ably.ClientOption{
			ably.WithEnvironment("sandbox"),
			ably.WithAutoConnect(false),
			ably.WithKey("made:up"),
		}

		realtime, err := ably.NewRealtime(options...)
		assert.NoError(t, err,
			"unexpected err: %s", err)

		changes := make(chan ably.ConnectionStateChange)
		defer ablytest.Instantly.NoRecv(t, nil, changes, t.Errorf)

		realtime.Connection.On(ably.ConnectionEventFailed, func(change ably.ConnectionStateChange) {
			changes <- change
		})

		realtime.Connect()
		ablytest.Soon.Recv(t, nil, changes, t.Fatalf)
	})
}

func Test_RTN6_Connected_When_CONNECTED_Msg_Received(t *testing.T) {
	// Check that the connection state changes to connected upon receipt
	// of a connected message.

	in := make(chan *ably.ProtocolMessage, 1)
	out := make(chan *ably.ProtocolMessage, 16)

	client, _ := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
		ably.WithDial(MessagePipe(in, out)),
	)

	stateChange := make(ably.ConnStateChanges, 10)
	off := client.Connection.OnAll(stateChange.Receive)
	defer off()

	client.Connect()

	var change ably.ConnectionStateChange

	ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
	assert.Equal(t, ably.ConnectionStateConnecting, change.Current,
		"expected %v; got %v (event: %+v)", ably.ConnectionStateConnecting, change.Current, change)
	ablytest.Instantly.NoRecv(t, nil, stateChange, t.Fatalf) // Shouldn't receive connected state change until CONNECTED MSG is received

	// send connected message
	in <- &ably.ProtocolMessage{
		Action:            ably.ActionConnected,
		ConnectionID:      "connection-id",
		ConnectionDetails: &ably.ConnectionDetails{},
	}

	ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
	assert.Equal(t, ably.ConnectionStateConnected, change.Current,
		"expected %v; got %v (event: %+v)", ably.ConnectionStateConnected, change.Current, change)
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

	assert.Equal(t, ably.ConnectionStateConnected, change.Current,
		"unexpected FAILED event: %s", change.Reason)
}

type connectionStateChanges chan ably.ConnectionStateChange

func (c connectionStateChanges) Receive(change ably.ConnectionStateChange) {
	c <- change
}

func TestRealtimeConn_RTN12_Connection_Close(t *testing.T) {

	setUpWithEOF := func() (app *ablytest.Sandbox, client *ably.Realtime, doEOF chan struct{}) {
		doEOF = make(chan struct{}, 1)

		app, client = ablytest.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				c, err := ably.DialWebsocket(protocol, u, timeout)
				return protoConnWithFakeEOF{Conn: c, doEOF: doEOF}, err
			}))

		err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
		assert.NoError(t, err)
		assert.Equal(t, ably.ConnectionStateConnected, client.Connection.State(),
			"expected %v; got %v", ably.ConnectionStateConnected, client.Connection.State())
		return
	}

	setUpWithConnectingInterrupt := func() (app *ablytest.Sandbox, client *ably.Realtime, dialErr chan error, waitTillDial chan error) {
		dialErr = make(chan error, 1)
		waitTillDial = make(chan error)
		app, client = ablytest.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				waitTillDial <- nil
				if err := <-dialErr; err != nil {
					return nil, err
				}
				return ably.DialWebsocket(protocol, u, timeout)
			}))
		return
	}

	t.Run("RTN12a: transition to closed on connection close", func(t *testing.T) {
		app, client, _ := setUpWithEOF()
		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

		stateChange := make(connectionStateChanges, 2)
		client.Connection.OnAll(stateChange.Receive)

		client.Close()

		var change ably.ConnectionStateChange
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)

		assert.Equal(t, ably.ConnectionStateClosing, change.Current,
			"expected %v; got %v", ably.ConnectionStateClosing, change.Current)

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)

		assert.Equal(t, ably.ConnectionStateClosed, change.Current,
			"expected %v; got %v", ably.ConnectionStateClosed, change.Current)

		ablytest.Instantly.NoRecv(t, nil, stateChange, t.Fatalf)
	})

	t.Run("RTN12b: transition to closed on close request timeout", func(t *testing.T) {
		connDetails := ably.ConnectionDetails{
			ConnectionKey:      "foo",
			ConnectionStateTTL: ably.DurationFromMsecs(time.Minute * 20),
			MaxIdleInterval:    ably.DurationFromMsecs(time.Minute * 5),
		}

		afterCalls := make(chan ablytest.AfterCall)
		now, after := ablytest.TimeFuncs(afterCalls)

		var in chan *ably.ProtocolMessage
		realtimeRequestTimeout := time.Minute
		c, _ := ably.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithToken("fake:token"),
			ably.WithRealtimeRequestTimeout(realtimeRequestTimeout),
			ably.WithNow(now),
			ably.WithAfter(after),
			ably.WithDial(func(p string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				in = make(chan *ably.ProtocolMessage, 1)
				out := make(chan *ably.ProtocolMessage, 16)
				in <- &ably.ProtocolMessage{
					Action:            ably.ActionConnected,
					ConnectionID:      "connection",
					ConnectionDetails: &connDetails,
				}
				return MessagePipe(in, out,
					MessagePipeWithNowFunc(now),
					MessagePipeWithAfterFunc(after),
				)(p, u, timeout)
			}))

		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		assert.NoError(t, err)

		stateChange := make(connectionStateChanges, 2)
		c.Connection.OnAll(stateChange.Receive)

		maxIdleInterval := time.Duration(connDetails.MaxIdleInterval)
		receiveTimeout := realtimeRequestTimeout + maxIdleInterval

		c.Close()
		// RTN23a The connection should be closed due to lack of activity past
		// receiveTimeout
		var change ably.ConnectionStateChange
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)

		assert.Equal(t, ably.ConnectionStateClosing, change.Current,
			"expected %v; got %v", ably.ConnectionStateClosing, change.Current)

		// Expect timer for a message receive.
		var timer ablytest.AfterCall

		ablytest.Instantly.Recv(t, &timer, afterCalls, t.Fatalf)
		assert.Equal(t, receiveTimeout, timer.D,
			"expected %v, got %v", receiveTimeout, timer.D)

		// Let the deadline pass without a message; expect a disconnection.
		timer.Fire()

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		assert.Equal(t, ably.ConnectionStateClosed, change.Current,
			"expected %v; got %v ", ably.ConnectionStateClosed, change.Current)

		errReason := change.Reason
		// make sure the reason is timeout
		assert.Contains(t, errReason.Error(), "timeout",
			"expected %q to contain timeout", errReason.Error())

		ablytest.Instantly.NoRecv(t, nil, stateChange, t.Fatalf)
	})

	t.Run("RTN12c: transition to closed on transport error", func(t *testing.T) {
		app, client, doEOF := setUpWithEOF()
		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

		stateChange := make(connectionStateChanges, 2)
		client.Connection.OnAll(stateChange.Receive)

		client.Close()

		var change ably.ConnectionStateChange
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)

		assert.Equal(t, ably.ConnectionStateClosing, change.Current,
			"expected %v; got %v", ably.ConnectionStateClosing, change.Current)

		doEOF <- struct{}{}

		select {
		case change = <-stateChange:
		case <-time.After(50 * time.Millisecond):
			t.Fatal("didn't transition on EOF")
		}

		assert.Equal(t, ably.ConnectionStateClosed, change.Current,
			"expected transition to %v, got %v", ably.ConnectionStateClosed, change.Current)

		ablytest.Instantly.NoRecv(t, nil, stateChange, t.Fatalf)
	})

	t.Run("RTN12d : should abort reconnection timer while disconnected on closed", func(t *testing.T) {
		connDetails := ably.ConnectionDetails{
			ConnectionKey:      "foo",
			ConnectionStateTTL: ably.DurationFromMsecs(time.Minute * 20),
			MaxIdleInterval:    ably.DurationFromMsecs(time.Minute * 5),
		}

		afterCalls := make(chan ablytest.AfterCall)
		now, after := ablytest.TimeFuncs(afterCalls)

		var in chan *ably.ProtocolMessage

		dialErr := make(chan error, 1)

		realtimeRequestTimeout := time.Minute
		c, _ := ably.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithToken("fake:token"),
			ably.WithRealtimeRequestTimeout(realtimeRequestTimeout),
			ably.WithNow(now),
			ably.WithAfter(after),
			ably.WithDial(func(p string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				if err := <-dialErr; err != nil {
					return nil, err
				}
				in = make(chan *ably.ProtocolMessage, 1)
				out := make(chan *ably.ProtocolMessage, 16)
				in <- &ably.ProtocolMessage{
					Action:            ably.ActionConnected,
					ConnectionID:      "connection",
					ConnectionDetails: &connDetails,
				}
				return MessagePipe(in, out,
					MessagePipeWithNowFunc(now),
					MessagePipeWithAfterFunc(after),
				)(p, u, timeout)
			}))

		dialErr <- nil
		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		assert.NoError(t, err)

		stateChange := make(connectionStateChanges, 2)
		c.Connection.OnAll(stateChange.Receive)

		maxIdleInterval := time.Duration(connDetails.MaxIdleInterval)
		receiveTimeout := realtimeRequestTimeout + maxIdleInterval

		// Expect timer for a message receive.
		var timer ablytest.AfterCall

		// receive message request timeout inside eventloop
		ablytest.Instantly.Recv(t, &timer, afterCalls, t.Fatalf)
		assert.LessOrEqual(t, timer.D, receiveTimeout, "timer was not within expected receive timeout")

		// Let the deadline pass without a message; expect a disconnection.
		timer.Fire()
		dialErr <- errors.New("can't reconnect once disconnected")

		var change ably.ConnectionStateChange
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)

		// received disconnect on first receive
		assert.Equal(t, ably.ConnectionStateDisconnected, change.Current,
			"expected %v; got %v", ably.ConnectionStateDisconnected, change.Current)

		// retry connection with ably server
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		assert.Equal(t, ably.ConnectionStateConnecting, change.Current,
			"expected %v; got %v", ably.ConnectionStateConnecting, change.Current)

		dialErr <- errors.New("can't reconnect once disconnected")
		// first reconnect failed (raw connection can't be established, outside retry loop), so returned disconnect
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		assert.Equal(t, ably.ConnectionStateDisconnected, change.Current,
			"expected %v; got %v", ably.ConnectionStateDisconnected, change.Current)

		// consume parent suspend timer with timeout connectionState TTL
		ablytest.Instantly.Recv(t, nil, afterCalls, t.Fatalf)
		// Went inside retry loop for connecting with server again
		// consume and wait for disconnect retry timer with disconnectedRetryTimeout
		ablytest.Instantly.Recv(t, nil, afterCalls, t.Fatalf)

		c.Close()
		// state change should triggger disconnect timer to trigger and stop the loop due to ConnectionStateClosed
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		assert.Equal(t, ably.ConnectionStateClosed, change.Current,
			"expected %v; got %v", ably.ConnectionStateClosed, change.Current)
		ablytest.Before(time.Second).NoRecv(t, nil, stateChange, t.Fatalf)
	})

	t.Run("RTN12d: should abort reconnection timer while suspended on closed", func(t *testing.T) {
		connDetails := ably.ConnectionDetails{
			ConnectionKey:      "foo",
			ConnectionStateTTL: ably.DurationFromMsecs(time.Minute * 20),
			MaxIdleInterval:    ably.DurationFromMsecs(time.Minute * 5),
		}

		afterCalls := make(chan ablytest.AfterCall)
		now, after := ablytest.TimeFuncs(afterCalls)

		var in chan *ably.ProtocolMessage

		dialErr := make(chan error, 1)
		realtimeRequestTimeout := time.Minute
		c, _ := ably.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithToken("fake:token"),
			ably.WithRealtimeRequestTimeout(realtimeRequestTimeout),
			ably.WithNow(now),
			ably.WithAfter(after),
			ably.WithDial(func(p string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				if err := <-dialErr; err != nil {
					return nil, err
				}
				in = make(chan *ably.ProtocolMessage, 1)
				out := make(chan *ably.ProtocolMessage, 16)
				in <- &ably.ProtocolMessage{
					Action:            ably.ActionConnected,
					ConnectionID:      "connection",
					ConnectionDetails: &connDetails,
				}
				return MessagePipe(in, out,
					MessagePipeWithNowFunc(now),
					MessagePipeWithAfterFunc(after),
				)(p, u, timeout)
			}))

		dialErr <- nil
		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		assert.NoError(t, err)

		stateChange := make(connectionStateChanges, 2)
		c.Connection.OnAll(stateChange.Receive)

		maxIdleInterval := time.Duration(connDetails.MaxIdleInterval)
		receiveTimeout := realtimeRequestTimeout + maxIdleInterval

		// Expect timer for a message receive.
		var timer ablytest.AfterCall

		// receive message request timeout inside eventloop
		ablytest.Instantly.Recv(t, &timer, afterCalls, t.Fatalf)
		assert.LessOrEqual(t, timer.D, receiveTimeout, "timer was not within expected receive timeout")

		// Let the deadline pass without a message; expect a disconnection.
		timer.Fire()
		dialErr <- errors.New("can't reconnect once disconnected")

		var change ably.ConnectionStateChange
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)

		// received disconnect on first receive
		assert.Equal(t, ably.ConnectionStateDisconnected, change.Current,
			"expected %v; got %v", ably.ConnectionStateDisconnected, change.Current)

		// retry connection with ably server
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		assert.Equal(t, ably.ConnectionStateConnecting, change.Current,
			"expected %v; got %v", ably.ConnectionStateConnecting, change.Current)

		dialErr <- errors.New("can't reconnect once disconnected")
		// first connect failed (raw connection can't be established), so returned disconnect
		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		assert.Equal(t, ably.ConnectionStateDisconnected, change.Current,
			"expected %v; got %v", ably.ConnectionStateDisconnected, change.Current)

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
		assert.Equal(t, ably.ConnectionStateSuspended, change.Current,
			"expected %v; got %v", ably.ConnectionStateSuspended, change.Current)

		// consume and wait for suspend retry timer with suspendedRetryTimeout
		ablytest.Instantly.Recv(t, nil, afterCalls, t.Fatalf)

		c.Close()

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		assert.Equal(t, ably.ConnectionStateClosed, change.Current,
			"expected %v; got %v", ably.ConnectionStateClosed, change.Current)

		ablytest.Before(time.Second).NoRecv(t, nil, stateChange, t.Fatalf)
	})

	t.Run("RTN12f: transition to closed when close is called intermittently", func(t *testing.T) {
		app, client, interrupt, waitTillConnecting := setUpWithConnectingInterrupt()
		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

		stateChange := make(ably.ConnStateChanges, 10)
		off := client.Connection.OnAll(stateChange.Receive)
		defer off()

		client.Connect()

		var change ably.ConnectionStateChange

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		assert.Equal(t, ably.ConnectionStateConnecting, change.Current,
			"expected %v; got %v", ably.ConnectionStateConnecting, change.Current)

		<-waitTillConnecting

		client.Close()

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		assert.Equal(t, ably.ConnectionStateClosing, change.Current,
			"expected %v; got %v", ably.ConnectionStateClosing, change.Current)

		interrupt <- nil // continue to connected

		ablytest.Soon.Recv(t, &change, stateChange, t.Fatalf)
		assert.Equal(t, ably.ConnectionStateClosed, change.Current,
			"expected %v; got %v (event: %+v)", ably.ConnectionStateClosed, change.Current, change)
	})
}

func TestRealtimeConn_RTN15a_ReconnectOnEOF(t *testing.T) {

	doEOF := make(chan struct{}, 1)

	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			c, err := ably.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err,
		"Connect=%s", err)

	channel := client.Channels.Get("channel")

	err = channel.Attach(context.Background())
	assert.NoError(t, err)

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

	assert.Equal(t, ably.ConnectionStateDisconnected, state.Current,
		"expected transition to %v, got %v", ably.ConnectionStateDisconnected, state.Current)

	// Publish a message to the channel through REST. If connection recovery
	// succeeds, we should then receive it without reattaching.

	rest, err := ably.NewREST(app.Options()...)
	assert.NoError(t, err)
	err = rest.Channels.Get("channel").Publish(context.Background(), "name", "data")
	assert.NoError(t, err)

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't reconnect")
	}

	assert.Equal(t, ably.ConnectionStateConnecting, state.Current,
		"expected transition to %v, got %v", ably.ConnectionStateConnecting, state.Current)

	select {
	case state = <-stateChanges:
	case <-time.After(ablytest.Timeout):
		t.Fatal("didn't transition from CONNECTING")
	}

	assert.Equal(t, ably.ConnectionStateConnected, state.Current,
		"expected transition to %v, got %v", ably.ConnectionStateConnected, state.Current)

	select {
	case msg := <-sub:
		assert.Equal(t, "data", msg.Data,
			"expected message with data \"data\", got %v", msg.Data)

	case <-time.After(ablytest.Timeout):
		t.Fatal("expected message after connection recovery; got none")
	}
}

type protoConnWithFakeEOF struct {
	ably.Conn
	doEOF     <-chan struct{}
	onMessage func(msg *ably.ProtocolMessage)
}

func (c protoConnWithFakeEOF) Receive(deadline time.Time) (*ably.ProtocolMessage, error) {
	type result struct {
		msg *ably.ProtocolMessage
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

type transportMessages struct {
	sync.Mutex
	dial     *url.URL
	messages []*ably.ProtocolMessage
}

func (tm *transportMessages) Add(m *ably.ProtocolMessage) {
	tm.Lock()
	defer tm.Unlock()
	tm.messages = append(tm.messages, m)
}

func (tm *transportMessages) Messages() []*ably.ProtocolMessage {
	tm.Lock()
	defer tm.Unlock()
	return tm.messages
}

func TestRealtimeConn_RTN15b(t *testing.T) {

	doEOF := make(chan struct{}, 1)

	var metaList []*transportMessages
	gotDial := make(chan chan struct{})
	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			m := &transportMessages{dial: u}
			metaList = append(metaList, m)
			if len(metaList) > 1 {
				goOn := make(chan struct{})
				gotDial <- goOn
				<-goOn
			}
			c, err := ably.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF, onMessage: func(msg *ably.ProtocolMessage) {
				m.Add(msg)
			}}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err,
		"Connect=%s", err)

	channel := client.Channels.Get("channel")

	err = channel.Attach(context.Background())
	assert.NoError(t, err)

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

	assert.Equal(t, ably.ConnectionStateDisconnected, state.Current,
		"expected transition to %v, got %v", ably.ConnectionStateDisconnected, state.Current)

	// Publish a message to the channel through REST. If connection recovery
	// succeeds, we should then receive it without reattaching.

	rest, err := ably.NewREST(app.Options()...)
	assert.NoError(t, err)
	goOn := <-gotDial
	err = rest.Channels.Get("channel").Publish(context.Background(), "name", "data")
	assert.NoError(t, err)
	close(goOn)

	select {
	case state = <-stateChanges:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("didn't reconnect")
	}

	assert.Equal(t, ably.ConnectionStateConnecting, state.Current,
		"expected transition to %v, got %v", ably.ConnectionStateConnecting, state.Current)

	select {
	case state = <-stateChanges:
	case <-time.After(ablytest.Timeout):
		t.Fatal("didn't transition from CONNECTING")
	}

	assert.Equal(t, ably.ConnectionStateConnected, state.Current,
		"expected transition to %v, got %v", ably.ConnectionStateConnected, state.Current)
	assert.Equal(t, 2, len(metaList),
		"expected 2 connection dialing got %d", len(metaList))

	{ //(RTN15b1)
		u := metaList[1].dial
		resume := u.Query().Get("resume")
		connKey := recent(metaList[0].Messages(), ably.ActionConnected).ConnectionDetails.ConnectionKey
		assert.NotEqual(t, "", resume,
			"expected resume query param to be set")
		assert.Equal(t, connKey, resume,
			"resume: expected %q got %q", connKey, resume)
	}
}

func recent(msgs []*ably.ProtocolMessage, action ably.ProtoAction) *ably.ProtocolMessage {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Action == action {
			return msgs[i]
		}
	}
	return nil
}

func TestRealtimeConn_RTN15c6(t *testing.T) {

	doEOF := make(chan struct{}, 1)

	continueDial := make(chan struct{}, 1)
	continueDial <- struct{}{}

	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			<-continueDial
			c, err := ably.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{
				Conn:  c,
				doEOF: doEOF,
			}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err, "Connect=%s", err)
	prevConnId := client.Connection.ID()

	// Increase msgSerial, to test that it doesn't reset later.
	err = client.Channels.Get("publish").Publish(context.Background(), "test", nil)
	assert.NoError(t, err)
	assert.NotZero(t, client.Connection.MsgSerial())

	channel := client.Channels.Get("channel")
	err = channel.Attach(context.Background())
	assert.NoError(t, err)

	sub, unsub, err := ablytest.ReceiveMessages(channel, "")
	assert.NoError(t, err)
	defer unsub()

	chanStateChanges := make(ably.ChannelStateChanges)
	off := channel.OnAll(chanStateChanges.Receive)
	defer off()

	connStateChanges := make(chan ably.ConnectionStateChange, 16)
	client.Connection.OnAll(func(c ably.ConnectionStateChange) {
		connStateChanges <- c
	})

	doEOF <- struct{}{}

	var connState ably.ConnectionStateChange

	ablytest.Soon.Recv(t, &connState, connStateChanges, t.Fatalf)

	assert.Equal(t, ably.ConnectionStateDisconnected, connState.Current,
		"expected transition to %v, got %v", ably.ConnectionStateDisconnected, connState.Current)

	rest, err := ably.NewREST(app.Options()...)
	assert.NoError(t, err)
	err = rest.Channels.Get("channel").Publish(context.Background(), "name", "data")
	assert.NoError(t, err)

	continueDial <- struct{}{}

	ablytest.Soon.Recv(t, &connState, connStateChanges, t.Fatalf)
	assert.Equal(t, ably.ConnectionStateConnecting, connState.Current, "expected connecting; got %+v", connState.Current)

	ablytest.Soon.Recv(t, &connState, connStateChanges, t.Fatalf)
	assert.Equal(t, ably.ConnectionStateConnected, connState.Current, "expected connected; got %+v", connState.Current)
	assert.Nil(t, connState.Reason, "expected nil conn error, got %+v", connState.Reason)

	// Check channel goes into attaching and attached state
	var change ably.ChannelStateChange
	ablytest.Soon.Recv(t, &change, chanStateChanges, t.Fatalf)
	assert.Equal(t, ably.ChannelStateAttaching, change.Current, "expected no state change; got %+v", change)
	ablytest.Soon.Recv(t, &change, chanStateChanges, t.Fatalf)
	assert.Equal(t, ably.ChannelStateAttached, change.Current, "expected no state change; got %+v", change)

	// Expect message to be received after resume success
	var msg *ably.Message
	ablytest.Soon.Recv(t, &msg, sub, t.Fatalf)

	// Check for resume success
	assert.Equal(t, prevConnId, client.Connection.ID())
	assert.Nil(t, client.Connection.ErrorReason())
	assert.NotZero(t, client.Connection.MsgSerial())

	// Set channel to attaching state
	channel.SetState(ably.ChannelStateAttaching)
	doEOF <- struct{}{}
	continueDial <- struct{}{}

	// Check channel goes into attaching and attached state
	ablytest.Soon.Recv(t, &change, chanStateChanges, t.Fatalf)
	assert.Equal(t, ably.ChannelStateAttaching, change.Current, "expected no state change; got %+v", change)
	ablytest.Soon.Recv(t, &change, chanStateChanges, t.Fatalf)
	assert.Equal(t, ably.ChannelStateAttached, change.Current, "expected no state change; got %+v", change)

	// Set channel to suspended state
	channel.SetState(ably.ChannelStateSuspended)
	doEOF <- struct{}{}
	continueDial <- struct{}{}

	// Check channel goes into attaching and attached state
	ablytest.Soon.Recv(t, &change, chanStateChanges, t.Fatalf)
	assert.Equal(t, ably.ChannelStateAttaching, change.Current, "expected no state change; got %+v", change)
	ablytest.Soon.Recv(t, &change, chanStateChanges, t.Fatalf)
	assert.Equal(t, ably.ChannelStateAttached, change.Current, "expected no state change; got %+v", change)

	// Check for resume success
	assert.Equal(t, prevConnId, client.Connection.ID())
	assert.Nil(t, client.Connection.ErrorReason())
	assert.NotZero(t, client.Connection.MsgSerial())
}

func TestRealtimeConn_RTN15c7_attached(t *testing.T) {

	doEOF := make(chan struct{}, 1)

	continueDial := make(chan struct{}, 1)
	continueDial <- struct{}{}

	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			<-continueDial
			c, err := ably.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{
				Conn:  c,
				doEOF: doEOF,
			}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err, "Connect=%s", err)
	prevConnId := client.Connection.ID()

	// Increase msgSerial, to test that it gets reset later.
	err = client.Channels.Get("publish").Publish(context.Background(), "test", nil)
	assert.NoError(t, err)
	assert.NotZero(t, client.Connection.MsgSerial())

	channel := client.Channels.Get("channel")
	err = channel.Attach(context.Background())
	assert.NoError(t, err)

	_, unsub, err := ablytest.ReceiveMessages(channel, "")
	assert.NoError(t, err)
	defer unsub()

	chanStateChanges := make(ably.ChannelStateChanges)
	off := channel.OnAll(chanStateChanges.Receive)
	defer off()

	connStateChanges := make(chan ably.ConnectionStateChange, 16)
	client.Connection.OnAll(func(c ably.ConnectionStateChange) {
		connStateChanges <- c
	})

	client.Connection.SetKey("xxxxx!xxxxxxx-xxxxxxxx-xxxxxxxx") // invalid connection key for next resume request
	doEOF <- struct{}{}

	var connState ably.ConnectionStateChange

	ablytest.Soon.Recv(t, &connState, connStateChanges, t.Fatalf)
	assert.Equal(t, ably.ConnectionStateDisconnected, connState.Current,
		"expected transition to %v, got %v", ably.ConnectionStateDisconnected, connState.Current)

	rest, err := ably.NewREST(app.Options()...)
	assert.NoError(t, err)
	err = rest.Channels.Get("channel").Publish(context.Background(), "name", "data")
	assert.NoError(t, err)

	continueDial <- struct{}{}

	ablytest.Soon.Recv(t, &connState, connStateChanges, t.Fatalf)
	assert.Equal(t, ably.ConnectionStateConnecting, connState.Current)

	ablytest.Soon.Recv(t, &connState, connStateChanges, t.Fatalf)
	assert.Equal(t, ably.ConnectionStateConnected, connState.Current)
	assert.NotNil(t, connState.Reason, "expected not nil connError, got nil connError")

	// Check channel goes into attaching and attached state
	var change ably.ChannelStateChange
	ablytest.Soon.Recv(t, &change, chanStateChanges, t.Fatalf)
	assert.Equal(t, ably.ChannelStateAttaching, change.Current)
	ablytest.Soon.Recv(t, &change, chanStateChanges, t.Fatalf)
	assert.Equal(t, ably.ChannelStateAttached, change.Current)

	// Check for resume failure
	assert.NotEqual(t, prevConnId, client.Connection.ID())
	assert.Zero(t, client.Connection.MsgSerial())
	assert.NotNil(t, client.Connection.ErrorReason())
	assert.Equal(t, 400, client.Connection.ErrorReason().StatusCode)

	// Todo - Expect message not to be arrived due to resume failure
	// var msg *ably.Message
	// ablytest.Soon.NoRecv(t, &msg, sub, t.Fatalf)
}

func TestRealtimeConn_RTN15c4(t *testing.T) {

	doEOF := make(chan struct{}, 1)

	continueDial := make(chan struct{}, 1)
	continueDial <- struct{}{}
	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			<-continueDial
			c, err := ably.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{
				Conn:  c,
				doEOF: doEOF,
			}, err
		}))
	defer safeclose(t, &closeClient{Closer: ablytest.FullRealtimeCloser(client), skip: []int{http.StatusBadRequest}}, app)

	err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err, "Connect=%s", err)

	channel := client.Channels.Get("channel")
	err = channel.Attach(context.Background())
	assert.NoError(t, err)
	chanStateChanges := make(ably.ChannelStateChanges, 1)
	off := channel.On(ably.ChannelEventFailed, chanStateChanges.Receive)
	defer off()

	connStateChanges := make(chan ably.ConnectionStateChange, 16)
	client.Connection.OnAll(func(c ably.ConnectionStateChange) {
		connStateChanges <- c
	})

	client.Connection.SetKey("wrong-conn-key") // wrong connection key for next resume request
	doEOF <- struct{}{}

	var connState ably.ConnectionStateChange

	ablytest.Soon.Recv(t, &connState, connStateChanges, t.Fatalf)
	assert.Equal(t, ably.ConnectionStateDisconnected, connState.Current,
		"expected transition to %v, got %v", ably.ConnectionStateDisconnected, connState.Current)

	rest, err := ably.NewREST(app.Options()...)
	assert.NoError(t, err)
	err = rest.Channels.Get("channel").Publish(context.Background(), "name", "data")
	assert.NoError(t, err)

	continueDial <- struct{}{}

	ablytest.Soon.Recv(t, &connState, connStateChanges, t.Fatalf)
	assert.Equal(t, ably.ConnectionStateConnecting, connState.Current)

	// Connection goes into failed state
	ablytest.Soon.Recv(t, &connState, connStateChanges, t.Fatalf)
	assert.Equal(t, ably.ConnectionStateFailed, connState.Current)

	// Check channel goes into failed state
	var change ably.ChannelStateChange
	ablytest.Soon.Recv(t, &change, chanStateChanges, t.Fatalf)
	assert.Equal(t, ably.ChannelStateFailed, change.Current)

	reason := client.Connection.ErrorReason()
	assert.NotNil(t, reason, "expected reason to be set")
	assert.Equal(t, http.StatusBadRequest, reason.StatusCode,
		"expected %d got %d", http.StatusBadRequest, reason.StatusCode)
}

func TestRealtimeConn_RTN15d_MessageRecovery(t *testing.T) {
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

	err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	channel := client.Channels.Get("test")
	err = channel.Attach(context.Background())
	assert.NoError(t, err)

	sub, unsub, err := ablytest.ReceiveMessages(channel, "test")
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	for i := 0; i < 3; i++ {
		err := rest.Channels.Get("test").Publish(context.Background(), "test", fmt.Sprintf("msg %d", i))
		assert.NoError(t, err,
			"%d: %v", i, err)
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

		assert.Equal(t, fmt.Sprintf("msg %d", i), msg.Data,
			"expected %v, got %v", fmt.Sprintf("msg %d", i), msg.Data)
	}
}

func TestRealtimeConn_RTN15e_ConnKeyUpdatedOnReconnect(t *testing.T) {

	doEOF := make(chan struct{}, 1)

	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			c, err := ably.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

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

	assert.NotEqual(t, "", key2,
		"the new key is empty")
	assert.NotEqual(t, key1, key2,
		"expected key %q to be different from key %q", key1, key2)
}

func TestRealtimeConn_RTN15g_NewConnectionOnStateLost(t *testing.T) {

	out := make(chan *ably.ProtocolMessage, 16)

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

	connDetails := ably.ConnectionDetails{
		ConnectionKey:      "foo",
		ConnectionStateTTL: ably.DurationFromMsecs(time.Minute * 2),
		MaxIdleInterval:    ably.DurationFromMsecs(time.Second),
	}

	dials := make(chan *url.URL, 1)
	connIDs := make(chan string, 1)
	var breakConn func()
	var in chan *ably.ProtocolMessage

	c, _ := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
		ably.WithNow(now),
		ably.WithDial(func(p string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			in = make(chan *ably.ProtocolMessage, 1)
			in <- &ably.ProtocolMessage{
				Action:            ably.ActionConnected,
				ConnectionID:      <-connIDs,
				ConnectionDetails: &connDetails,
			}
			breakConn = func() { close(in) }
			dials <- u
			return MessagePipe(in, out)(p, u, timeout)
		}))

	connIDs <- "conn-1"
	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)
	prevConnectionKey := c.Connection.Key()

	ablytest.Instantly.Recv(t, nil, dials, t.Fatalf)

	// Get channels to ATTACHING, ATTACHED and DETACHED. (TODO: SUSPENDED)
	attaching := c.Channels.Get("attaching")
	_ = ablytest.ResultFunc.Go(func(ctx context.Context) error { return attaching.Attach(ctx) })
	msg := <-out
	assert.Equal(t, ably.ActionAttach, msg.Action,
		"expected ATTACH, got %v", msg.Action)

	attached := c.Channels.Get("attached")
	attachWaiter := ablytest.ResultFunc.Go(func(ctx context.Context) error { return attached.Attach(ctx) })
	msg = <-out
	assert.Equal(t, ably.ActionAttach, msg.Action,
		"expected ATTACH, got %v", msg.Action)
	in <- &ably.ProtocolMessage{
		Action:  ably.ActionAttached,
		Channel: "attached",
	}
	ablytest.Wait(attachWaiter, err)

	detached := c.Channels.Get("detached")
	attachWaiter = ablytest.ResultFunc.Go(func(ctx context.Context) error { return detached.Attach(ctx) })
	msg = <-out
	assert.Equal(t, ably.ActionAttach, msg.Action,
		"expected ATTACH, got %v", msg.Action)
	in <- &ably.ProtocolMessage{
		Action:  ably.ActionAttached,
		Channel: "detached",
	}
	ablytest.Wait(attachWaiter, err)
	detachWaiter := ablytest.ResultFunc.Go(func(ctx context.Context) error { return detached.Detach(ctx) })
	msg = <-out
	assert.Equal(t, ably.ActionDetach, msg.Action,
		"expected DETACH, got %v", msg.Action)
	in <- &ably.ProtocolMessage{
		Action:  ably.ActionDetached,
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
	assert.Equal(t, prevConnectionKey, dialed.Query().Get("resume"))
	ablytest.Instantly.Recv(t, nil, connected, t.Fatalf) // wait for CONNECTED before disconnecting again

	// RTN15g3: Expect the previously attaching and attached channels to be
	// attached again.

	attachExpected := map[string]struct{}{
		"attaching": {},
		"attached":  {},
	}
	for len(attachExpected) > 0 {
		var msg *ably.ProtocolMessage
		ablytest.Instantly.Recv(t, &msg, out, t.Fatalf)
		_, ok := attachExpected[msg.Channel]
		assert.True(t, ok,
			"ATTACH sent for unexpected or already attaching channel %q", msg.Channel)
		delete(attachExpected, msg.Channel)
	}
	ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)

	// Now do the same, but past connectionStateTTL + maxIdleInterval. This
	// should make a fresh connection.

	setNow(now().Add(discardStateTTL + 1))
	breakConn()
	connIDs <- "conn-2" // different connection id, so resume failure
	ablytest.Instantly.Recv(t, &dialed, dials, t.Fatalf)
	assert.Empty(t, dialed.Query().Get("resume"))
	assert.Empty(t, dialed.Query().Get("recover"))
	ablytest.Instantly.Recv(t, nil, connected, t.Fatalf)

	// RTN15g3: Expect the previously attaching and attached channels to be
	// attached again.
	attachExpected = map[string]struct{}{
		"attaching": {},
		"attached":  {},
	}
	for len(attachExpected) > 0 {
		var msg *ably.ProtocolMessage
		ablytest.Instantly.Recv(t, &msg, out, t.Fatalf)
		_, ok := attachExpected[msg.Channel]
		assert.True(t, ok,
			"ATTACH sent for unexpected or already attaching channel %q", msg.Channel)
		delete(attachExpected, msg.Channel)
	}
	ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)
}

func TestRealtimeConn_RTN15h1_OnDisconnectedCannotRenewToken(t *testing.T) {

	in := make(chan *ably.ProtocolMessage, 1)
	out := make(chan *ably.ProtocolMessage, 16)

	c, _ := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
		ably.WithDial(MessagePipe(in, out)),
	)

	in <- &ably.ProtocolMessage{
		Action:            ably.ActionConnected,
		ConnectionID:      "connection-id",
		ConnectionDetails: &ably.ConnectionDetails{},
	}

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	err = ablytest.Wait(ablytest.ConnWaiter(c, func() {
		in <- &ably.ProtocolMessage{
			Action: ably.ActionDisconnected,
			Error: &ably.ProtoErrorInfo{
				StatusCode: 401,
				Code:       40141,
				Message:    "fake token error",
			},
		}
	}, ably.ConnectionEventFailed), nil)

	var errInfo *ably.ErrorInfo
	hasErr := errors.As(err, &errInfo)
	assert.True(t, hasErr)
	assert.Equal(t, 401, errInfo.StatusCode,
		"expected the token error as FAILED state change reason; got: %s", err)
	assert.Equal(t, 40141, int(errInfo.Code),
		"expected the token error as FAILED state change reason; got: %s", err)
}

func TestRealtimeConn_RTN15h2_ReauthFails(t *testing.T) {

	authErr := errors.New("reauth error")

	in := make(chan *ably.ProtocolMessage, 1)
	out := make(chan *ably.ProtocolMessage, 16)

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
		ably.WithDial(MessagePipe(in, out)),
	)

	in <- &ably.ProtocolMessage{
		Action:            ably.ActionConnected,
		ConnectionID:      "connection-id",
		ConnectionDetails: &ably.ConnectionDetails{},
	}

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	err = ablytest.Wait(ablytest.ConnWaiter(c, func() {
		in <- &ably.ProtocolMessage{
			Action: ably.ActionDisconnected,
			Error: &ably.ProtoErrorInfo{
				StatusCode: 401,
				Code:       40141,
				Message:    "fake token error",
			},
		}
	}, ably.ConnectionEventDisconnected), nil)
	assert.Error(t, err,
		"expected the auth error as DISCONNECTED state change reason; got: %s", err)
}

func TestRealtimeConn_RTN15h2_ReauthWithBadToken(t *testing.T) {

	in := make(chan *ably.ProtocolMessage, 1)
	out := make(chan *ably.ProtocolMessage, 16)

	dials := make(chan *url.URL, 1)

	c, _ := ably.NewRealtime(
		ably.WithToken("fake:token"),
		ably.WithAutoConnect(false),
		ably.WithAuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
			return ably.TokenString("bad:token"), nil
		}),
		ably.WithDial(func(proto string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			dials <- u
			return MessagePipe(in, out)(proto, u, timeout)
		}))

	in <- &ably.ProtocolMessage{
		Action:            ably.ActionConnected,
		ConnectionID:      "connection-id",
		ConnectionDetails: &ably.ConnectionDetails{},
	}

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	ablytest.Instantly.Recv(t, nil, dials, t.Fatalf)

	stateChanges := make(chan ably.ConnectionStateChange, 1)

	off := c.Connection.OnAll(func(change ably.ConnectionStateChange) {
		stateChanges <- change
	})
	defer off()

	// Remove the buffer from dials, so that we can make the library wait for
	// us to receive it before a state change.
	dials = make(chan *url.URL)

	in <- &ably.ProtocolMessage{
		Action: ably.ActionDisconnected,
		Error: &ably.ProtoErrorInfo{
			StatusCode: 401,
			Code:       40141,
			Message:    "fake token error",
		},
	}

	// No state change expected before a reauthorization and reconnection
	// attempt.
	ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)

	// The DISCONNECTED causes a reauth, and dial again with the new
	// token.
	var dialURL *url.URL
	ablytest.Instantly.Recv(t, &dialURL, dials, t.Fatalf)

	assert.Equal(t, "bad:token", dialURL.Query().Get("access_token"),
		"expected reauthorization with token returned by the authCallback; got %q", dialURL.Query().Get("access_token"))

	// After a token error response, we finally get to our expected
	// DISCONNECTED state.

	in <- &ably.ProtocolMessage{
		Action: ably.ActionError,
		Error: &ably.ProtoErrorInfo{
			StatusCode: 401,
			Code:       40141,
			Message:    "fake token error",
		},
	}

	var change ably.ConnectionStateChange
	ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)

	assert.Equal(t, ably.ConnectionStateDisconnected, change.Current,
		"expected DISCONNECTED event; got %v", change)
	assert.Equal(t, 401, change.Reason.StatusCode,
		"expected the token error as FAILED state change reason; got: %s", change.Reason)
	assert.Equal(t, 40141, int(change.Reason.Code),
		"expected the token error as FAILED state change reason; got: %s", change.Reason)
}

func TestRealtimeConn_RTN15h2_Success(t *testing.T) {

	in := make(chan *ably.ProtocolMessage, 1)
	out := make(chan *ably.ProtocolMessage, 16)

	dials := make(chan *url.URL, 1)

	c, _ := ably.NewRealtime(
		ably.WithToken("fake:token"),
		ably.WithAutoConnect(false),
		ably.WithAuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
			return ably.TokenString("good:token"), nil
		}),
		ably.WithDial(func(proto string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			dials <- u
			return MessagePipe(in, out)(proto, u, timeout)
		}))

	in <- &ably.ProtocolMessage{
		Action:            ably.ActionConnected,
		ConnectionID:      "connection-id",
		ConnectionDetails: &ably.ConnectionDetails{},
	}

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	ablytest.Instantly.Recv(t, nil, dials, t.Fatalf)

	stateChanges := make(chan ably.ConnectionStateChange, 1)

	off := c.Connection.OnAll(func(change ably.ConnectionStateChange) {
		stateChanges <- change
	})
	defer off()

	in <- &ably.ProtocolMessage{
		Action: ably.ActionDisconnected,
		Error: &ably.ProtoErrorInfo{
			StatusCode: 401,
			Code:       40141,
			Message:    "fake token error",
		},
	}

	// The DISCONNECTED causes a reauth, and dial again with the new
	// token.
	var dialURL *url.URL
	ablytest.Instantly.Recv(t, &dialURL, dials, t.Fatalf)

	assert.Equal(t, "good:token", dialURL.Query().Get("access_token"),
		"expected reauthorization with token returned by the authCallback; got %q", dialURL.Query().Get("access_token"))

	// Simulate a successful reconnection.
	in <- &ably.ProtocolMessage{
		Action:            ably.ActionConnected,
		ConnectionID:      "new-connection-id",
		ConnectionDetails: &ably.ConnectionDetails{},
	}

	// Expect a UPDATED event.

	var change ably.ConnectionStateChange
	ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)

	assert.Equal(t, ably.ConnectionEventUpdate, change.Event,
		"expected UPDATED event; got %v", change)

	// Expect no further events.break
	ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)
}

func TestRealtimeConn_RTN15i_OnErrorWhenConnected(t *testing.T) {

	in := make(chan *ably.ProtocolMessage, 1)
	out := make(chan *ably.ProtocolMessage, 16)

	c, _ := ably.NewRealtime(
		ably.WithToken("fake:token"),
		ably.WithAutoConnect(false),
		ably.WithDial(MessagePipe(in, out)),
	)

	// Get the connection to CONNECTED.

	in <- &ably.ProtocolMessage{
		Action:            ably.ActionConnected,
		ConnectionID:      "connection-id",
		ConnectionDetails: &ably.ConnectionDetails{},
	}

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	// Get a channel to ATTACHED.

	channel := c.Channels.Get("test")
	attachWaiter := ablytest.ResultFunc.Go(func(ctx context.Context) error { return channel.Attach(ctx) })

	in <- &ably.ProtocolMessage{
		Action:  ably.ActionAttached,
		Channel: "test",
	}

	ablytest.Wait(attachWaiter, err)

	// Send a fake ERROR; expect a transition to FAILED.

	channelFailed := make(ably.ChannelStateChanges, 1)
	off := channel.On(ably.ChannelEventFailed, channelFailed.Receive)
	defer off()

	err = ablytest.Wait(ablytest.ConnWaiter(c, func() {
		in <- &ably.ProtocolMessage{
			Action: ably.ActionError,
			Error: &ably.ProtoErrorInfo{
				StatusCode: 500,
				Code:       50123,
				Message:    "fake error",
			},
		}
	}, ably.ConnectionEventFailed), nil)

	var errorInfo *ably.ErrorInfo
	hasErr := errors.As(err, &errorInfo)
	assert.True(t, hasErr)
	assert.Equal(t, 50123, int(errorInfo.Code),
		"expected error code 50123; got %v", errorInfo)
	assert.NotNil(t, c.Connection.ErrorReason(),
		"expected error reason not to be nil")
	assert.Equal(t, errorInfo, c.Connection.ErrorReason(),
		"expected %v; got %v")

	// The channel should be moved to FAILED too.

	ablytest.Instantly.Recv(t, nil, channelFailed, t.Fatalf)

	assert.Equal(t, ably.ChannelStateFailed, channel.State(),
		"expected channel in state %v; got %v", ably.ChannelStateFailed, channel.State())
}

func TestRealtimeConn_RTN16(t *testing.T) {
	app, c := ablytest.NewRealtime()
	defer safeclose(t, ablytest.FullRealtimeCloser(c), app)

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)
	channel := c.Channels.Get("channel")
	err = channel.Attach(context.Background())
	assert.NoError(t, err)

	var msg *ably.Message
	sub, unsub, err := ablytest.ReceiveMessages(channel, "")
	if err != nil {
		t.Fatal(err)
	}
	defer unsub()
	err = channel.Publish(context.Background(), "name", "data")
	assert.NoError(t, err)

	ablytest.Soon.Recv(t, &msg, sub, t.Fatalf)
	assert.Equal(t, "data", msg.Data)

	prevMsgSerial := c.Connection.MsgSerial()
	prevConnId := c.Connection.ID()

	recoveryKey := c.Connection.CreateRecoveryKey()                // RTN16g - createRecoveryKey
	decodedRecoveryKey, err := ably.DecodeRecoveryKey(recoveryKey) // RTN16g1
	assert.Nil(t, err)

	deprecatedRecoveryKey := c.Connection.RecoveryKey()
	assert.Equal(t, deprecatedRecoveryKey, recoveryKey) //RTN16m

	client := app.NewRealtime(
		ably.WithRecover(recoveryKey),
	)
	defer safeclose(t, ablytest.FullRealtimeCloser(client))

	err = ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	{ // RTN16f, RTN16j, RTN16d
		assert.True(t, sameConnection(client.Connection.Key(), c.Connection.Key()),
			"expected the same connection")
		assert.Equal(t, prevConnId, c.Connection.ID())
		assert.Nil(t, client.Connection.ErrorReason())
		assert.Equal(t, prevMsgSerial, client.Connection.MsgSerial(),
			"expected %d got %d", prevMsgSerial, client.Connection.MsgSerial())
		assert.True(t, client.Channels.Exists("channel"))
		channelSerial := client.Channels.Get("channel").GetChannelSerial()
		assert.Equal(t, decodedRecoveryKey.ChannelSerials["channel"], channelSerial)
	}
	{ //(RTN16g2)
		err := ablytest.Wait(ablytest.ConnWaiter(client, client.Close, ably.ConnectionEventClosed), nil)
		assert.NoError(t, err)
		assert.Equal(t, "", client.Connection.Key(),
			"expected key to be empty got %q instead", client.Connection.Key())
		assert.Equal(t, "", client.Connection.RecoveryKey(),
			"expected recovery key to be empty got %q instead", client.Connection.RecoveryKey())
		assert.Equal(t, "", client.Connection.ID(),
			"expected id to be empty got %q instead", client.Connection.ID())
	}
	{ //(RTN16l)
		// This test was adopted from the ably-js project
		// https://github.com/ably/ably-js/blob/340e5ce31dc9d7434a06ae4e1eec32bdacc9c6c5/spec/realtime/connection.test.js#L119
		var query url.Values
		decodedRecoveryKey.ConnectionKey = "ablygo_test_fake-key____"
		faultyRecoveryKey, _ := decodedRecoveryKey.Encode()
		client2 := app.NewRealtime(
			ably.WithRecover(faultyRecoveryKey),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				query = u.Query()
				return ably.DialWebsocket(protocol, u, timeout)
			}))
		defer safeclose(t, ablytest.FullRealtimeCloser(client2))
		err = ablytest.Wait(ablytest.ConnWaiter(client2, client2.Connect, ably.ConnectionEventConnected), nil)
		assert.Error(t, err, "expected reason to be set")
		if err == nil {
			t.Fatal("expected reason to be set")
		}
		{ // (RTN16i)
			recoverValue := query.Get("recover")
			assert.NotEmpty(t, recoverValue)
			assert.Equal(t, "ablygo_test_fake-key____", recoverValue)
		}
		{ //(RTN16e)
			info := err.(*ably.ErrorInfo)
			assert.Equal(t, 80018, int(info.Code),
				"expected 80018 got %d", info.Code)
			reason := client2.Connection.ErrorReason()
			assert.Equal(t, 80018, int(reason.Code),
				"expected 80018 got %d", reason.Code)
			msgSerial := client2.Connection.MsgSerial()
			// verify msgSerial is 0 (new connection), not 3
			assert.Zero(t, msgSerial)
			assert.NotContains(t, client2.Connection.Key(), "ablygo_test_fake",
				"expected %q not to contain \"ablygo_test_fake\"", client2.Connection.Key())
		}
	}
}

func sameConnection(a, b string) bool {
	return strings.Split(a, "-")[0] == strings.Split(b, "-")[0]
}

func TestRealtimeConn_RTN23(t *testing.T) {
	connDetails := ably.ConnectionDetails{
		ConnectionKey:      "foo",
		ConnectionStateTTL: ably.DurationFromMsecs(time.Minute * 20),
		MaxIdleInterval:    ably.DurationFromMsecs(time.Minute * 5),
	}

	afterCalls := make(chan ablytest.AfterCall)
	now, after := ablytest.TimeFuncs(afterCalls)

	dials := make(chan *url.URL, 1)
	var in chan *ably.ProtocolMessage
	realtimeRequestTimeout := time.Minute
	c, _ := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
		ably.WithRealtimeRequestTimeout(realtimeRequestTimeout),
		ably.WithNow(now),
		ably.WithAfter(after),
		ably.WithDial(func(p string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			in = make(chan *ably.ProtocolMessage, 1)
			in <- &ably.ProtocolMessage{
				Action:            ably.ActionConnected,
				ConnectionID:      "connection",
				ConnectionDetails: &connDetails,
			}
			dials <- u
			return MessagePipe(in, nil,
				MessagePipeWithNowFunc(now),
				MessagePipeWithAfterFunc(after),
			)(p, u, timeout)
		}))
	disconnected := make(chan *ably.ErrorInfo, 1)
	c.Connection.Once(ably.ConnectionEventDisconnected, func(e ably.ConnectionStateChange) {
		disconnected <- e.Reason
	})
	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	var dialed *url.URL
	ablytest.Instantly.Recv(t, &dialed, dials, t.Fatalf)
	// RTN23b
	assert.Equal(t, "true", dialed.Query().Get("heartbeats"),
		"expected heartbeats query param to be \"true\" got %q", dialed.Query().Get("heartbeats"))

	maxIdleInterval := time.Duration(connDetails.MaxIdleInterval)
	receiveTimeout := realtimeRequestTimeout + maxIdleInterval

	// Expect a timer for a message receive.
	var timer ablytest.AfterCall
	ablytest.Instantly.Recv(t, &timer, afterCalls, t.Fatalf)
	assert.LessOrEqual(t, timer.D, receiveTimeout, "timer was not within expected receive timeout")

	in <- &ably.ProtocolMessage{
		Action: ably.ActionHeartbeat,
	}

	// An incoming message should cancel the timer and prevent a disconnection.
	ablytest.Instantly.Recv(t, nil, timer.Ctx.Done(), t.Fatalf)
	ablytest.Instantly.NoRecv(t, nil, disconnected, t.Fatalf)

	// Expect another timer for a message receive.
	ablytest.Instantly.Recv(t, &timer, afterCalls, t.Fatalf)
	assert.Equal(t, receiveTimeout, timer.D,
		"expected %v, got %v", receiveTimeout, timer.D)

	// Let the deadline pass without a message; expect a disconnection.
	timer.Fire()

	// RTN23a The connection should be disconnected due to lack of activity past
	// receiveTimeout
	var reason *ably.ErrorInfo
	ablytest.Instantly.Recv(t, &reason, disconnected, t.Fatalf)

	// make sure the reason is timeout
	assert.Contains(t, reason.Error(),
		"timeout", "expected %q to contain \"timeout\"", reason.Error())
}

type writerLogger struct {
	w io.Writer
}

func (w *writerLogger) Printf(level ably.LogLevel, format string, v ...interface{}) {
	fmt.Fprintf(w.w, format, v...)
}

func TestRealtimeConn_RTN14c_ConnectedTimeout(t *testing.T) {

	afterCalls := make(chan ablytest.AfterCall)
	now, after := ablytest.TimeFuncs(afterCalls)

	in := make(chan *ably.ProtocolMessage, 10)
	out := make(chan *ably.ProtocolMessage, 10)

	c, err := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
		ably.WithNow(now),
		ably.WithAfter(after),
		ably.WithDial(MessagePipe(in, out,
			MessagePipeWithNowFunc(now),
			MessagePipeWithAfterFunc(after),
		)),
	)
	assert.NoError(t, err)
	defer c.Close()

	stateChanges := make(ably.ConnStateChanges, 10)
	off := c.Connection.OnAll(stateChanges.Receive)
	defer off()

	c.Connect()

	var change ably.ConnectionStateChange
	ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
	assert.Equal(t, connecting, change.Current,
		"expected \"CONNECTING\"; got %v", change.Current)

	// Once dialed, expect a timer for realtimeRequestTimeout.

	const realtimeRequestTimeout = 10 * time.Second // DF1b

	var receiveTimer ablytest.AfterCall
	ablytest.Instantly.Recv(t, &receiveTimer, afterCalls, t.Fatalf)

	assert.Equal(t, realtimeRequestTimeout, receiveTimer.D,
		"expected %v; got %v", realtimeRequestTimeout, receiveTimer.D)

	// Expect a state change to DISCONNECTED when the timer expires.
	ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)

	receiveTimer.Fire()

	ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
	assert.Equal(t, disconnected, change.Current,
		"expected \"DISCONNECTED\"; got %v", change.Current)
}

func TestRealtimeConn_RTN14a(t *testing.T) {
	t.Skip(`
	Currently its impossible to implement/test this.
	See https://github.com/ably/docs/issues/984 for details
	`)
}

func TestRealtimeConn_RTN14b(t *testing.T) {
	t.Run("renewable token that fails to renew with token error", func(t *testing.T) {
		in := make(chan *ably.ProtocolMessage, 1)
		out := make(chan *ably.ProtocolMessage, 16)
		var reauth atomic.Value
		reauth.Store(int(0))
		c, _ := ably.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithAuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
				reauth.Store(reauth.Load().(int) + 1)
				if reauth.Load().(int) > 1 {
					return nil, errors.New("bad token request")
				}
				return ably.TokenString("fake:token"), nil
			}),
			ably.WithDial(MessagePipe(in, out)))
		// Get the connection to CONNECTED.
		in <- &ably.ProtocolMessage{
			Action:            ably.ActionError,
			ConnectionDetails: &ably.ConnectionDetails{},
			Error: &ably.ProtoErrorInfo{
				StatusCode: http.StatusUnauthorized,
				Code:       40140,
			},
		}
		change := make(ably.ConnStateChanges, 1)
		c.Connection.OnAll(change.Receive)
		c.Connect()
		var state ably.ConnectionStateChange
		ablytest.Instantly.Recv(t, nil, change, t.Fatalf) // Skip CONNECTING
		ablytest.Instantly.Recv(t, &state, change, t.Fatalf)
		assert.Equal(t, ably.ConnectionStateDisconnected, state.Current,
			"expected %v got %v", ably.ConnectionStateDisconnected, state.Current)
		assert.Contains(t, c.Connection.ErrorReason().Error(), "bad token request",
			"expected %v to contain \"bad token request\"", c.Connection.ErrorReason())

		n := reauth.Load().(int) - 1 // we remove the first attempt
		assert.Equal(t, 1, n,
			"expected re authorization to happen once but happened %d", n)
	})
	t.Run("renewable token, consecutive token errors", func(t *testing.T) {
		in := make(chan *ably.ProtocolMessage, 1)
		out := make(chan *ably.ProtocolMessage, 16)
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
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				dials.Store(dials.Load().(int) + 1)
				return MessagePipe(in, out)(protocol, u, timeout)
			}))
		// Get the connection to CONNECTED.
		in <- &ably.ProtocolMessage{
			Action:            ably.ActionError,
			ConnectionDetails: &ably.ConnectionDetails{},
			Error: &ably.ProtoErrorInfo{
				StatusCode: http.StatusUnauthorized,
				Code:       40140,
			},
		}
		change := make(ably.ConnStateChanges, 1)
		c.Connection.OnAll(change.Receive)
		c.Connect()
		in <- &ably.ProtocolMessage{
			Action:            ably.ActionError,
			ConnectionDetails: &ably.ConnectionDetails{},
			Error: &ably.ProtoErrorInfo{
				StatusCode: http.StatusUnauthorized,
				Code:       40140,
				Message:    "bad token request",
			},
		}
		var state ably.ConnectionStateChange
		ablytest.Instantly.Recv(t, nil, change, t.Fatalf) // skip CONNECTING
		ablytest.Instantly.Recv(t, &state, change, t.Fatalf)
		assert.Equal(t, ably.ConnectionStateDisconnected, state.Current,
			"expected \"DISCONNECTED\" got %v", state.Current)
		assert.Contains(t, c.Connection.ErrorReason().Error(), "bad token request",
			"expected %v to contain \"bad token request\"", c.Connection.ErrorReason().Error())
		n := reauth.Load().(int)
		assert.Equal(t, 2, n,
			"expected re authorization to happen twice got %d", n)

		// We should only try to reconnect once after token error.
		assert.Equal(t, 2, dials.Load().(int),
			"expected 2 got %v", dials.Load().(int))
	})

}

type closeConn struct {
	ably.Conn
	closed atomic.Int64
}

func (c *closeConn) Close() error {
	c.closed.Add(1)
	return c.Conn.Close()
}

type noopConn struct {
	ch chan struct{}
}

func (noopConn) Send(*ably.ProtocolMessage) error {
	return nil
}

func (n *noopConn) Receive(deadline time.Time) (*ably.ProtocolMessage, error) {
	n.ch <- struct{}{}
	return &ably.ProtocolMessage{}, nil
}
func (noopConn) Close() error { return nil }

func TestRealtimeConn_RTN14g(t *testing.T) {
	t.Run("Non RTN14b error", func(t *testing.T) {
		in := make(chan *ably.ProtocolMessage, 1)
		out := make(chan *ably.ProtocolMessage, 16)
		var ls *closeConn
		c, _ := ably.NewRealtime(
			ably.WithToken("fake:token"),
			ably.WithAutoConnect(false),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				w, err := MessagePipe(in, out)(protocol, u, timeout)
				if err != nil {
					return nil, err
				}
				ls = &closeConn{Conn: w}
				return ls, nil
			}))
		in <- &ably.ProtocolMessage{
			Action:            ably.ActionError,
			ConnectionDetails: &ably.ConnectionDetails{},
			Error: &ably.ProtoErrorInfo{
				StatusCode: http.StatusBadRequest,
			},
		}
		change := make(ably.ConnStateChanges, 1)
		c.Connection.OnAll(change.Receive)
		c.Connect()
		var state ably.ConnectionStateChange
		ablytest.Instantly.Recv(t, nil, change, t.Fatalf) // Skip CONNECTING
		ablytest.Instantly.Recv(t, &state, change, t.Fatalf)
		assert.Equal(t, ably.ConnectionStateFailed, state.Current,
			"expected %v got %v", ably.ConnectionStateFailed, state.Current)
		assert.Equal(t, http.StatusBadRequest, c.Connection.ErrorReason().StatusCode,
			"expected status 400 got %v", c.Connection.ErrorReason().StatusCode)

		// we make sure the connection is closed
		assert.Equal(t, 1, ls.closed.Load(), "expected 1 got %v", ls.closed.Load())
	})
}

func TestRealtimeConn_RTN14e(t *testing.T) {
	ttl := 10 * time.Millisecond
	disconnTTL := ttl * 2
	suspendTTL := ttl / 2
	c, _ := ably.NewRealtime(
		ably.WithToken("fake:token"),
		ably.WithAutoConnect(false),
		ably.WithConnectionStateTTL(ttl),
		ably.WithSuspendedRetryTimeout(suspendTTL),
		ably.WithDisconnectedRetryTimeout(disconnTTL),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			return nil, context.DeadlineExceeded
		}))
	defer c.Close()
	changes := make(ably.ConnStateChanges, 2)
	off := c.Connection.On(ably.ConnectionEventSuspended, changes.Receive)
	defer off()
	c.Connect()
	var state ably.ConnectionStateChange
	ablytest.Soon.Recv(t, &state, changes, t.Fatalf)
	assert.Equal(t, suspendTTL, state.RetryIn,
		"expected retry to be in %v got %v", suspendTTL, state.RetryIn)
	assert.Contains(t, state.Reason.Error(), "Exceeded connectionStateTtl=10ms while in DISCONNECTED state",
		"expected %v to contain \"Exceeded connectionStateTtl=10ms while in DISCONNECTED state\"", state.Reason.Error())
	// make sure we are from DISCONNECTED => SUSPENDED
	assert.Equal(t, ably.ConnectionStateDisconnected, state.Previous,
		"expected transitioning from \"DISCONNECTED\" got %v", state.Previous)
	state = ably.ConnectionStateChange{}
	ablytest.Soon.Recv(t, &state, changes, t.Fatalf)

	// in suspend retries we move from
	//  SUSPENDED => CONNECTING => SUSPENDED ...
	assert.Equal(t, ably.ConnectionStateConnecting, state.Previous,
		"expected transitioning from \"CONNECTING\" got %v", state.Previous)
}

func TestRealtimeConn_RTN19b(t *testing.T) {
	connIDs := make(chan string)
	var breakConn func()
	var out, in chan *ably.ProtocolMessage
	c, err := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithKey("fake:key"),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			in = make(chan *ably.ProtocolMessage, 1)
			in <- &ably.ProtocolMessage{
				Action:       ably.ActionConnected,
				ConnectionID: <-connIDs,
				ConnectionDetails: &ably.ConnectionDetails{
					ConnectionKey: "key",
				},
			}
			out = make(chan *ably.ProtocolMessage, 16)
			breakConn = func() { close(in) }
			return MessagePipe(in, out)(protocol, u, timeout)
		}),
	)
	assert.NoError(t, err)
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
	assert.Equal(t, ably.ChannelStateAttaching, state.Current,
		"expected %v got %v", ably.ChannelStateAttaching, state.Current)
	in <- &ably.ProtocolMessage{
		Action:  ably.ActionAttached,
		Channel: "detaching",
	}
	ablytest.Wait(wait, nil)
	state = ably.ChannelStateChange{}
	ablytest.Soon.Recv(t, &state, detachChange, t.Fatalf)
	assert.Equal(t, ably.ChannelStateAttached, state.Current,
		"expected %v got %v", ably.ChannelStateAttached, state.Current)

	_ = ablytest.ResultFunc.Go(func(ctx context.Context) error { return detaching.Detach(ctx) })
	state = ably.ChannelStateChange{}
	ablytest.Soon.Recv(t, &state, detachChange, t.Fatalf)
	assert.Equal(t, ably.ChannelStateDetaching, state.Current,
		"expected %v got %v", ably.ChannelStateDetaching, state.Current)

	msgs := []ably.ProtocolMessage{
		{
			Channel: "attaching",
			Action:  ably.ActionAttach,
		},
		{
			Channel: "detaching",
			Action:  ably.ActionAttach,
		},
		{
			Channel: "detaching",
			Action:  ably.ActionDetach,
		},
	}
	var gots []*ably.ProtocolMessage
	for range msgs {
		var got *ably.ProtocolMessage
		ablytest.Instantly.Recv(t, &got, out, t.Fatalf)
		gots = append(gots, got)
	}

	sort.Slice(gots, func(i, j int) bool { return gots[i].Action < gots[j].Action })

	for i, expect := range msgs {
		got := gots[i]
		assert.Equal(t, expect.Action, got.Action,
			"expected %v got %v", expect.Action, got.Action)
		assert.Equal(t, expect.Channel, got.Channel,
			"expected %v got %v", expect.Channel, got.Channel)
	}
	breakConn()
	connIDs <- "2"
	ablytest.Soon.Recv(t, nil, changes, t.Fatalf)
	if c.Connection.ID() != "2" {
		t.Fatal("expected new connection")
	}
	msgs = []ably.ProtocolMessage{
		{
			Channel: "attaching",
			Action:  ably.ActionAttach,
		},
		{
			Channel: "detaching",
			Action:  ably.ActionDetach,
		},
	}

	gots = nil
	for range msgs {
		var got *ably.ProtocolMessage
		ablytest.Instantly.Recv(t, &got, out, t.Fatalf)
		gots = append(gots, got)
	}

	sort.Slice(gots, func(i, j int) bool { return gots[i].Action < gots[j].Action })

	for i, expect := range msgs {
		got := gots[i]
		assert.Equal(t, expect.Action, got.Action,
			"expected %v got %v", expect.Action, got.Action)
		assert.Equal(t, expect.Channel, got.Channel,
			"expected %v got %v", expect.Channel, got.Channel)
	}
}

func TestRealtimeConn_RTN19a(t *testing.T) {
	connIDs := make(chan string)
	var breakConn func()
	var out, in chan *ably.ProtocolMessage
	c, err := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithKey("fake:key"),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			in = make(chan *ably.ProtocolMessage, 1)
			in <- &ably.ProtocolMessage{
				Action:       ably.ActionConnected,
				ConnectionID: <-connIDs,
				ConnectionDetails: &ably.ConnectionDetails{
					ConnectionKey: "key",
				},
			}
			out = make(chan *ably.ProtocolMessage, 16)
			breakConn = func() { close(in) }
			return MessagePipe(in, out)(protocol, u, timeout)
		}),
	)
	assert.NoError(t, err)
	changes := make(ably.ConnStateChanges, 2)
	off := c.Connection.On(ably.ConnectionEventConnected, changes.Receive)
	defer off()
	c.Connect()
	connIDs <- "1"
	ablytest.Soon.Recv(t, nil, changes, t.Fatalf)

	channel := c.Channels.Get("channel")
	chanChange := make(ably.ChannelStateChanges, 1)
	off = channel.OnAll(chanChange.Receive)
	defer off()

	wait := ablytest.ResultFunc.Go(func(ctx context.Context) error { return channel.Attach(ctx) })
	var state ably.ChannelStateChange
	ablytest.Soon.Recv(t, &state, chanChange, t.Fatalf)
	assert.Equal(t, ably.ChannelStateAttaching, state.Current,
		"expected %v got %v", ably.ChannelStateAttaching, state.Current)
	in <- &ably.ProtocolMessage{
		Action:  ably.ActionAttached,
		Channel: "channel",
	}
	ablytest.Wait(wait, nil)
	state = ably.ChannelStateChange{}
	ablytest.Soon.Recv(t, &state, chanChange, t.Fatalf)
	assert.Equal(t, ably.ChannelStateAttached, state.Current,
		"expected %v got %v", ably.ChannelStateAttached, state.Current)
	ctx, cancel := context.WithTimeout(context.TODO(), time.Millisecond)
	defer cancel()
	err = channel.Publish(ctx, "ack", "ack")
	if err != nil {
		assert.Equal(t, context.DeadlineExceeded, err)
	}
	ablytest.Soon.Recv(t, nil, out, t.Fatalf) // attach

	var msg *ably.ProtocolMessage
	ablytest.Soon.Recv(t, &msg, out, t.Fatalf)
	assert.Equal(t, ably.ActionMessage, msg.Action,
		"expected %v got %v", ably.ActionMessage, msg.Action)
	assert.Equal(t, 1, c.Connection.PendingItems(),
		"expected 1 got %v", c.Connection.PendingItems())

	breakConn()
	connIDs <- "2"
	ablytest.Soon.Recv(t, nil, changes, t.Fatalf)
	assert.Equal(t, "2", c.Connection.ID(),
		"expected \"2\" got %v", c.Connection.ID())

	ablytest.Instantly.Recv(t, nil, out, t.Fatalf) // attach
	msg = nil
	ablytest.Instantly.Recv(t, &msg, out, t.Fatalf)
	assert.Equal(t, ably.ActionMessage, msg.Action,
		"expected %v got %v", ably.ActionMessage, msg.Action)
	assert.Equal(t, "channel", msg.Channel,
		"expected \"channel\" got %v", msg.Channel)
	assert.Equal(t, 1, c.Connection.PendingItems(),
		"expected 1 got %v", c.Connection.PendingItems())
}

func TestRealtimeConn_RTN24_RTN21_RTC8a_RTN4h_Override_ConnectionDetails_On_Connected(t *testing.T) {

	in := make(chan *ably.ProtocolMessage, 1)
	out := make(chan *ably.ProtocolMessage, 16)

	c, _ := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
		ably.WithDial(MessagePipe(in, out)),
	)

	connDetails := ably.ConnectionDetails{
		ClientID:           "id1",
		ConnectionKey:      "foo",
		MaxFrameSize:       12,
		MaxInboundRate:     14,
		MaxMessageSize:     67,
		ConnectionStateTTL: ably.DurationFromMsecs(time.Minute * 2),
		MaxIdleInterval:    ably.DurationFromMsecs(time.Second),
	}

	in <- &ably.ProtocolMessage{
		Action:            ably.ActionConnected,
		ConnectionID:      "connection-id-1",
		ConnectionDetails: &connDetails,
	}

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	changes := make(ably.ConnStateChanges, 3)
	off := c.Connection.OnAll(changes.Receive)
	defer off()

	//  Send new connection details
	in <- &ably.ProtocolMessage{
		Action:       ably.ActionConnected,
		ConnectionID: "connection-id-2",
		ConnectionDetails: &ably.ConnectionDetails{
			ClientID:           "id2",
			ConnectionKey:      "bar",
			MaxFrameSize:       13,
			MaxInboundRate:     15,
			MaxMessageSize:     70,
			ConnectionStateTTL: ably.DurationFromMsecs(time.Minute * 3),
			MaxIdleInterval:    ably.DurationFromMsecs(time.Second),
		},
		Error: &ably.ProtoErrorInfo{
			StatusCode: 500,
			Code:       50500,
			Message:    "fake error",
		},
	}

	var newConnectionState ably.ConnectionStateChange
	ablytest.Instantly.Recv(t, &newConnectionState, changes, t.Fatalf)

	// RTN4h - can emit UPDATE event
	assert.Equal(t, ably.ConnectionEventUpdate, newConnectionState.Event,
		"expected %v; got %v (event: %+v)", ably.ConnectionEventUpdate, newConnectionState.Event, newConnectionState)
	assert.Equal(t, ably.ConnectionStateConnected, newConnectionState.Current,
		"expected %v; got %v (event: %+v)", ably.ConnectionStateConnected, newConnectionState.Current, newConnectionState)
	assert.Equal(t, ably.ConnectionStateConnected, newConnectionState.Previous,
		"expected %v; got %v (event: %+v)", ably.ConnectionStateConnected, newConnectionState.Previous, newConnectionState)
	assert.Contains(t, newConnectionState.Reason.Message(), "fake error",
		"expected %+v to contain \"fake error\"", newConnectionState.Reason.Message())
	assert.Contains(t, c.Connection.ErrorReason().Message(), "fake error",
		"expected %+v to contain \"fake error\"", c.Connection.ErrorReason().Message())

	// RTN21 - new connection details overwrite old values
	assert.Equal(t, "bar", c.Connection.Key(),
		"expected \"bar\"; got %v")
	assert.Equal(t, "id2", c.Auth.ClientID(),
		"expected \"id2\"; got %v", c.Auth.ClientID())
	assert.Equal(t, time.Duration(time.Minute*3), c.Connection.ConnectionStateTTL(),
		"expected %v; got %v", time.Duration(time.Minute*3), c.Connection.ConnectionStateTTL())
	assert.Equal(t, time.Duration(time.Minute*3), c.Connection.ConnectionStateTTL(),
		"expected %v; got %v", time.Duration(time.Minute*3), c.Connection.ConnectionStateTTL())
	assert.Equal(t, "connection-id-2", c.Connection.ID(),
		"expected \"connection-id-2\"; got %v", c.Connection.ID())
}

func Test_RTN7b_ACK_NACK(t *testing.T) {

	// See also https://docs.ably.io/client-lib-development-guide/protocol/#message-acknowledgement

	in := make(chan *ably.ProtocolMessage, 16)
	out := make(chan *ably.ProtocolMessage, 16)

	c, _ := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
		ably.WithDial(MessagePipe(in, out)),
	)

	connDetails := ably.ConnectionDetails{
		ClientID:      "id1",
		ConnectionKey: "foo",
	}

	in <- &ably.ProtocolMessage{
		Action:            ably.ActionConnected,
		ConnectionID:      "connection-id-1",
		ConnectionDetails: &connDetails,
	}

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	// Set things up.

	ch := c.Channels.Get("test")
	publish, receiveAck := testConcurrentPublisher(t, ch, out)

	// Publish 5 messages, get ACK-2, NACK-1. Then publish 2 more, get
	// NACK-1, ACK-2, ACK-1.

	// Publish 5 messages...
	for i := 0; i < 3; i++ {
		publish()
	}

	// Intersperse an ATTACH, which used to increase msgSerial, but shouldn't.
	// Regression test for https://github.com/ably/docs/pull/1115
	c.Channels.Get("test2").Attach(canceledCtx)
	var attachMsg *ably.ProtocolMessage
	ablytest.Instantly.Recv(t, &attachMsg, out, t.Fatalf)
	assert.Equal(t, ably.ActionAttach, attachMsg.Action)
	assert.Equal(t, int64(0), attachMsg.MsgSerial)

	for i := 0; i < 2; i++ {
		publish()
	}

	// ... get ACK-2, NACK-1 ...
	in <- &ably.ProtocolMessage{
		Action:    ably.ActionAck,
		MsgSerial: 0,
		Count:     2,
	}
	in <- &ably.ProtocolMessage{
		Action:    ably.ActionNack,
		MsgSerial: 2,
		Count:     1,
		Error: &ably.ProtoErrorInfo{
			StatusCode: 500,
			Code:       50500,
			Message:    "fake error",
		},
	}
	for i, expectErr := range []bool{false, false, true} {
		err := receiveAck()
		hasErr := err != nil
		assert.Equal(t, hasErr, expectErr,
			"%v [%d]", err, i)
	}

	// ... publish 2 more ...
	for i := 0; i < 2; i++ {
		publish()
	}

	// ... get NACK-1, ACK-2, ACK-1
	in <- &ably.ProtocolMessage{
		Action:    ably.ActionNack,
		MsgSerial: 3,
		Count:     1,
		Error: &ably.ProtoErrorInfo{
			StatusCode: 500,
			Code:       50500,
			Message:    "fake error",
		},
	}
	in <- &ably.ProtocolMessage{
		Action:    ably.ActionAck,
		MsgSerial: 4,
		Count:     2,
	}
	in <- &ably.ProtocolMessage{
		Action:    ably.ActionAck,
		MsgSerial: 6,
		Count:     1,
	}
	for i, expectErr := range []bool{true, false, false, false} {
		err := receiveAck()
		if hasErr := err != nil; hasErr != expectErr {
			t.Fatalf("%v [%d]", err, i)
		}
	}

	ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)
}

func TestImplicitNACK(t *testing.T) {
	// From https://docs.ably.io/client-lib-development-guide/protocol/#message-acknowledgement:
	//
	// It is a protocol error if the system sends an ACK or NACK that skips past
	// one or more msgSerial without there having been either and ACK or NACK;
	// but a client in this situation should treat this case as
	// implicitly @NACK@ing the skipped messages.

	in := make(chan *ably.ProtocolMessage, 16)
	out := make(chan *ably.ProtocolMessage, 16)

	c, _ := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
		ably.WithDial(MessagePipe(in, out)),
	)

	in <- &ably.ProtocolMessage{
		Action:       ably.ActionConnected,
		ConnectionID: "connection-id-1",
		ConnectionDetails: &ably.ConnectionDetails{
			ClientID:      "id1",
			ConnectionKey: "foo",
		},
	}

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	ch := c.Channels.Get("test")
	publish, receiveAck := testConcurrentPublisher(t, ch, out)

	for i := 0; i < 4; i++ {
		publish()
	}

	in <- &ably.ProtocolMessage{
		Action:    ably.ActionAck,
		MsgSerial: 2, // skip 2
		Count:     2,
	}

	for i := 0; i < 2; i++ {
		err := receiveAck()
		assert.Error(t, err,
			"expected implicit NACK for msg %d", i)
	}
	for i := 2; i < 4; i++ {
		err := receiveAck()
		assert.NoError(t, err,
			"expected ACK for msg %d", i)
	}

	ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)
}

func TestIdempotentACK(t *testing.T) {
	// From https://docs.ably.io/client-lib-development-guide/protocol/#message-acknowledgement:
	//
	// It is also a protocol error if the system sends an ACK or NACK that
	// covers a msgSerial that was covered by an earlier ACK or NACK; in such
	// cases the client library must silently ignore the response insofar as it
	// relates to @msgSerial@s that were covered previously (whether the
	// response is the same now or different).

	in := make(chan *ably.ProtocolMessage, 16)
	out := make(chan *ably.ProtocolMessage, 16)

	c, _ := ably.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithToken("fake:token"),
		ably.WithDial(MessagePipe(in, out)),
	)

	in <- &ably.ProtocolMessage{
		Action:       ably.ActionConnected,
		ConnectionID: "connection-id-1",
		ConnectionDetails: &ably.ConnectionDetails{
			ClientID:      "id1",
			ConnectionKey: "foo",
		},
	}

	err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	ch := c.Channels.Get("test")
	publish, receiveAck := testConcurrentPublisher(t, ch, out)
	for i := 0; i < 4; i++ {
		publish()
	}

	in <- &ably.ProtocolMessage{
		Action:    ably.ActionAck,
		MsgSerial: 0,
		Count:     2,
	}

	for i := 0; i < 2; i++ {
		err := receiveAck()
		assert.NoError(t, err,
			"expected ACK for msg %d", i)
	}

	in <- &ably.ProtocolMessage{
		Action:    ably.ActionAck,
		MsgSerial: 1, // repeat ACK for msgSerial 1; ACK 2 more
		Count:     3,
	}

	for i := 2; i < 4; i++ {
		err := receiveAck()
		assert.NoError(t, err, "expected ACK for msg %d", i)
	}

	ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)
}

func TestRealtimeConn_RTC8a_ExplicitAuthorizeWhileConnected(t *testing.T) {
	// Set up a Realtime with AuthCallback that the test controls. Auth requests
	// are sent to the authRequested channel.
	type authResponse struct {
		token ably.Tokener
		err   error
	}
	type authRequest struct {
		params ably.TokenParams
		resp   chan<- authResponse
	}
	authRequests := make(chan authRequest, 1)
	handleAuth := func(getToken func(ably.TokenParams) ably.Tokener, err error) {
		t.Helper()

		var authReq authRequest
		ablytest.Soon.Recv(t, &authReq, authRequests, t.Fatalf)
		ablytest.Instantly.Send(t, authReq.resp, authResponse{
			token: getToken(authReq.params),
			err:   err,
		}, t.Fatalf)
	}

	// We'll use this REST to get real, working tokens.
	app, rest := ablytest.NewREST()
	getToken := func(params ably.TokenParams) ably.Tokener {
		t.Helper()
		token, err := rest.Auth.RequestToken(context.Background(), &params)
		assert.NoError(t, err)
		return token
	}
	dial, intercept := DialIntercept(ably.DialWebsocket)
	c := app.NewRealtime(
		ably.WithDial(dial),
		ably.WithDefaultTokenParams(ably.TokenParams{
			Capability: `{"foo":["subscribe"]}`,
		}),
		ably.WithAuthCallback(func(ctx context.Context, params ably.TokenParams) (ably.Tokener, error) {
			respCh := make(chan authResponse, 1)
			authRequests <- authRequest{
				params: params,
				resp:   respCh,
			}
			resp := <-respCh
			return resp.token, resp.err
		}),
	)
	defer safeclose(t, ablytest.FullRealtimeCloser(c), app)

	stateChanges := make(ably.ConnStateChanges, 1)
	off := c.Connection.OnAll(stateChanges.Receive)
	defer off()
	expectConnEvent := func(expected ably.ConnectionEvent) {
		t.Helper()

		var change ably.ConnectionStateChange
		ablytest.Soon.Recv(t, &change, stateChanges, t.Fatalf)
		assert.Equal(t, expected, change.Event,
			"expected: %v; got: %v", expected, change.Event)
	}

	// Expect a first auth when connecting.
	handleAuth(getToken, nil)
	expectConnEvent(ably.ConnectionEventConnected)

	t.Run("RTC8a1: Successful reauth with more capabilities", func(t *testing.T) {
		var rg ablytest.ResultGroup
		rg.GoAdd(func(ctx context.Context) error {
			_, err := c.Auth.Authorize(ctx, nil)
			return err
		})

		handleAuth(getToken, nil)
		expectConnEvent(ably.ConnectionEventUpdate)
		assert.Equal(t, connected, c.Connection.State())

		rg.Wait()
	})

	t.Run("RTC8a1: Successful reauth with more capabilities", func(t *testing.T) {
		var rg ablytest.ResultGroup
		rg.GoAdd(func(ctx context.Context) error {
			_, err := c.Auth.Authorize(ctx, &ably.TokenParams{
				Capability: `{"*":["*"]}`,
			})
			return err
		})

		handleAuth(getToken, nil)
		expectConnEvent(ably.ConnectionEventUpdate)
		assert.Equal(t, connected, c.Connection.State())

		rg.Wait()
	})

	t.Run("RTC8a1: Unsuccessful reauth with capabilities downgrade", func(t *testing.T) {
		ch := c.Channels.Get("notfoo")
		err := ch.Attach(context.Background())
		assert.NoError(t, err)

		channelChanges := make(ably.ChannelStateChanges, 1)
		off := ch.OnAll(channelChanges.Receive)
		defer off()

		var rg ablytest.ResultGroup
		rg.GoAdd(func(ctx context.Context) error {
			_, err := c.Auth.Authorize(ctx, &ably.TokenParams{
				Capability: `{"foo":["publish"]}`,
			})
			return err
		})

		handleAuth(getToken, nil)
		expectConnEvent(ably.ConnectionEventUpdate)
		assert.Equal(t, connected, c.Connection.State())

		rg.Wait()

		var change ably.ChannelStateChange
		ablytest.Soon.Recv(t, &change, channelChanges, t.Fatalf)
		assert.Equal(t, ably.ChannelEventFailed, change.Event)
		assert.Equal(t, chFailed, ch.State())
	})

	t.Run("RTC8a3: Authorize waits for connection update", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		connectedMsg := intercept(ctx, ably.ActionConnected)

		authorizeDone := make(chan struct{})
		var rg ablytest.ResultGroup
		defer rg.Wait()
		rg.GoAdd(func(ctx context.Context) error {
			defer close(authorizeDone)
			_, err := c.Auth.Authorize(ctx, nil)
			return err
		})

		handleAuth(getToken, nil)

		ablytest.Soon.Recv(t, nil, connectedMsg, t.Fatalf)

		// At this point, we have a new token and the server has sent a CONNECTED
		// message, but the library hasn't got it yet. So Authorize should still
		// be waiting.
		ablytest.Instantly.NoRecv(t, nil, authorizeDone, t.Fatalf)

		// Let go of the CONNECTED message, completing Authorization.
		cancel()
		expectConnEvent(ably.ConnectionEventUpdate)
		ablytest.Instantly.Recv(t, nil, authorizeDone, t.Fatalf)
	})

	t.Run("RTC8a4: reauthorize with JWT token", func(t *testing.T) {
		t.Skip("not implemented")
	})

	t.Run("RTC8a2: Failed reauth moves connection to FAILED", func(t *testing.T) {
		var rg ablytest.ResultGroup
		rg.GoAdd(func(ctx context.Context) error {
			_, err := c.Auth.Authorize(ctx, nil)
			return err
		})

		handleAuth(func(ably.TokenParams) ably.Tokener {
			return ably.TokenString("made:up")
		}, nil)
		expectConnEvent(ably.ConnectionEventFailed)
		assert.Equal(t, failed, c.Connection.State())

		rg.Wait()
	})
}

func testConcurrentPublisher(t *testing.T, ch *ably.RealtimeChannel, out <-chan *ably.ProtocolMessage) (
	publish func(),
	receiveAck func() error,
) {
	publishErrs := map[int64]<-chan error{}

	var i int64
	publish = func() {
		t.Helper()

		err := make(chan error, 1)
		publishErrs[i] = err
		go func(serial int64) {
			err <- ch.Publish(context.Background(), fmt.Sprintf("msg%d", serial), nil)
		}(i)

		var msg *ably.ProtocolMessage
		ablytest.Instantly.Recv(t, &msg, out, t.Fatalf, i)
		i++
	}

	var got int64
	receiveAck = func() error {
		t.Helper()

		var err error
		ablytest.Instantly.Recv(t, &err, publishErrs[got], t.Fatalf, got)
		got++
		return err
	}

	return
}
