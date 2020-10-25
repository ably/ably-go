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

func Test_RTN4a_ConnectionEventForStateChange(t *testing.T) {
	t.Run(fmt.Sprintf("on %s", ably.ConnectionStateConnecting), func(t *testing.T) {
		t.Parallel()

		app, realtime := ablytest.NewRealtime(ably.ClientOptions{}.AutoConnect(false))
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

		app, realtime := ablytest.NewRealtime(ably.ClientOptions{}.AutoConnect(false))
		defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

		connectAndWait(t, realtime)
	})

	t.Run(fmt.Sprintf("on %s", ably.ConnectionStateDisconnected), func(t *testing.T) {
		t.Parallel()

		dial, disconnect := ablytest.DialFakeDisconnect(nil)
		options := ably.ClientOptions{}.
			AutoConnect(false).
			Dial(dial)
		app, realtime := ablytest.NewRealtime(options)
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

		app, realtime := ablytest.NewRealtime(ably.ClientOptions{}.AutoConnect(false))
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

		app, realtime := ablytest.NewRealtime(ably.ClientOptions{}.AutoConnect(false))
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

		options := ably.ClientOptions{}.
			Environment("sandbox").
			AutoConnect(false).
			Key("made:up")

		realtime, err := ably.NewRealtime(options)
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

func TestRealtimeConn_RTN15a_ReconnectOnEOF(t *testing.T) {
	t.Parallel()

	doEOF := make(chan struct{}, 1)

	app, client := ablytest.NewRealtime(ably.ClientOptions{}.
		AutoConnect(false).
		Dial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			c, err := ablyutil.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait(); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	channel := client.Channels.Get("channel")

	if err := ablytest.Wait(channel.Attach()); err != nil {
		t.Fatal(err)
	}

	sub, err := channel.Subscribe()
	if err != nil {
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

	rest, err := ably.NewREST(app.Options(nil))
	if err != nil {
		t.Fatal(err)
	}
	err = rest.Channels.Get("channel").Publish("name", "data")
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
	case msg := <-sub.MessageChannel():
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
	app, client := ablytest.NewRealtime(ably.ClientOptions{}.
		AutoConnect(false).
		Dial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
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

	if err := ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait(); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	channel := client.Channels.Get("channel")

	if err := ablytest.Wait(channel.Attach()); err != nil {
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

	rest, err := ably.NewREST(app.Options(nil))
	if err != nil {
		t.Fatal(err)
	}
	goOn := <-gotDial
	err = rest.Channels.Get("channel").Publish("name", "data")
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

	app, client := ablytest.NewRealtime(ably.ClientOptions{}.
		AutoConnect(false).
		Dial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
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

	if err := ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait(); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	channel := client.Channels.Get("channel")
	if err := ablytest.Wait(channel.Attach()); err != nil {
		t.Fatal(err)
	}

	sub, err := channel.Subscribe()
	if err != nil {
		t.Fatal(err)
	}

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
	rest, err := ably.NewREST(app.Options(nil))
	if err != nil {
		t.Fatal(err)
	}
	goOn := <-gotDial
	err = rest.Channels.Get("channel").Publish("name", "data")
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
	case msg := <-sub.MessageChannel():
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
	app, client := ablytest.NewRealtime(ably.ClientOptions{}.
		AutoConnect(false).
		Dial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
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

	if err := ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait(); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	channel := client.Channels.Get("channel")
	if err := ablytest.Wait(channel.Attach()); err != nil {
		t.Fatal(err)
	}
	chanStateChanges := make(ably.ChannelStateChanges)
	off := channel.OnAll(chanStateChanges.Receive)
	defer off()

	sub, err := channel.Subscribe()
	if err != nil {
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
	rest, err := ably.NewREST(app.Options(nil))
	if err != nil {
		t.Fatal(err)
	}
	goOn := <-gotDial
	err = rest.Channels.Get("channel").Publish("name", "data")
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
	case msg := <-sub.MessageChannel():
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
	app, client := ablytest.NewRealtime(ably.ClientOptions{}.
		AutoConnect(false).
		Dial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
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

	if err := ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait(); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	channel := client.Channels.Get("channel")
	if err := ablytest.Wait(channel.Attach()); err != nil {
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
	rest, err := ably.NewREST(app.Options(nil))
	if err != nil {
		t.Fatal(err)
	}
	goOn := <-gotDial
	err = rest.Channels.Get("channel").Publish("name", "data")
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
	app, client := ablytest.NewRealtime(ably.ClientOptions{}.
		AutoConnect(false).
		Dial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
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

	if err := ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait(); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	channel := client.Channels.Get("channel")
	if _, err := channel.Attach(); err != nil {
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
	rest, err := ably.NewREST(app.Options(nil))
	if err != nil {
		t.Fatal(err)
	}
	goOn := <-gotDial
	err = rest.Channels.Get("channel").Publish("name", "data")
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
	app, client := ablytest.NewRealtime(ably.ClientOptions{}.
		AutoConnect(false).
		Dial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
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

	if err := ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait(); err != nil {
		t.Fatalf("Connect=%s", err)
	}

	channel := client.Channels.Get("channel")
	if err := ablytest.Wait(channel.Attach()); err != nil {
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
	rest, err := ably.NewREST(app.Options(nil))
	if err != nil {
		t.Fatal(err)
	}
	goOn := <-gotDial
	err = rest.Channels.Get("channel").Publish("name", "data")
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

	app, client := ablytest.NewRealtime(ably.ClientOptions{}.
		AutoConnect(false).
		Dial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			<-allowDial
			c, err := ablyutil.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait(); err != nil {
		t.Fatal(err)
	}

	channel := client.Channels.Get("test")
	err := ablytest.Wait(channel.Attach())
	if err != nil {
		t.Fatal(err)
	}

	sub, err := channel.Subscribe("test")
	if err != nil {
		t.Fatal(err)
	}

	if err := ablytest.ConnWaiter(client, func() {
		doEOF <- struct{}{}
	}, ably.ConnectionEventDisconnected).Wait(); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}

	// While we're DISCONNECTED, publish a few messages to the channel through
	// REST. If we then successfully recover connection state, the channel will
	// still be attached and the messages will arrive.

	rest, err := ably.NewREST(app.Options(nil))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		err := rest.Channels.Get("test").Publish("test", fmt.Sprintf("msg %d", i))
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
	}

	// Now let the connection reconnect.

	if err := ablytest.ConnWaiter(client, func() {
		allowDial <- struct{}{}
	}, ably.ConnectionEventConnected).Wait(); err != nil {
		t.Fatal(err)
	}

	// And expect the messages in the same channel.
	for i := 0; i < 3; i++ {
		fatalf := ablytest.FmtFunc(t.Fatalf).Wrap(t, "%d: %s", i)

		var msg *proto.Message
		ablytest.Soon.Recv(t, &msg, sub.MessageChannel(), fatalf)

		if expected, got := fmt.Sprintf("msg %d", i), msg.Data; expected != got {
			fatalf("expected %v, got %v", expected, got)
		}
	}
}

func TestRealtimeConn_RTN15e_ConnKeyUpdatedOnReconnect(t *testing.T) {
	t.Parallel()

	doEOF := make(chan struct{}, 1)

	app, client := ablytest.NewRealtime(ably.ClientOptions{}.
		AutoConnect(false).
		Dial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			c, err := ablyutil.DialWebsocket(protocol, u, timeout)
			return protoConnWithFakeEOF{Conn: c, doEOF: doEOF}, err
		}))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	if err := ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait(); err != nil {
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

	c, _ := ably.NewRealtime(ably.ClientOptions{}.
		AutoConnect(false).
		Token("fake:token").
		Now(now).
		Dial(func(p string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
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
	err := ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected).Wait()
	if err != nil {
		t.Fatal(err)
	}

	ablytest.Instantly.Recv(t, nil, dials, t.Fatalf) // discard first URL; we're interested in reconnections

	// Get channels to ATTACHING, ATTACHED and DETACHED. (TODO: SUSPENDED)

	attaching := c.Channels.Get("attaching")
	_, err = attaching.Attach()
	if err != nil {
		t.Fatal(err)
	}
	if msg := <-out; msg.Action != proto.ActionAttach {
		t.Fatalf("expected ATTACH, got %v", msg)
	}

	attached := c.Channels.Get("attached")
	attachWaiter, err := attached.Attach()
	if err != nil {
		t.Fatal(err)
	}
	if msg := <-out; msg.Action != proto.ActionAttach {
		t.Fatalf("expected ATTACH, got %v", msg)
	}
	in <- &proto.ProtocolMessage{
		Action:  proto.ActionAttached,
		Channel: "attached",
	}
	ablytest.Wait(attachWaiter, err)

	detached := c.Channels.Get("detached")
	attachWaiter, err = detached.Attach()
	if err != nil {
		t.Fatal(err)
	}
	if msg := <-out; msg.Action != proto.ActionAttach {
		t.Fatalf("expected ATTACH, got %v", msg)
	}
	in <- &proto.ProtocolMessage{
		Action:  proto.ActionAttached,
		Channel: "detached",
	}
	ablytest.Wait(attachWaiter, err)
	detachWaiter, err := detached.Detach()
	if err != nil {
		t.Fatal(err)
	}
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

	c, _ := ably.NewRealtime(ably.ClientOptions{}.
		AutoConnect(false).
		Token("fake:token").
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

	tokenErr := proto.ErrorInfo{
		StatusCode: 401,
		Code:       40141,
		Message:    "fake token error",
	}

	err = ablytest.ConnWaiter(c, func() {
		in <- &proto.ProtocolMessage{
			Action: proto.ActionDisconnected,
			Error:  &tokenErr,
		}
	}, ably.ConnectionEventFailed).Wait()

	var errInfo *ably.ErrorInfo
	if !errors.As(err, &errInfo) {
		t.Fatal(err)
	}

	if errInfo.StatusCode != tokenErr.StatusCode || errInfo.Code != tokenErr.Code {
		t.Fatalf("expected the token error as FAILED state change reason; got: %s", err)
	}
}

func TestRealtimeConn_RTN15h2_ReauthFails(t *testing.T) {
	t.Parallel()

	authErr := fmt.Errorf("reauth error")

	in := make(chan *proto.ProtocolMessage, 1)
	out := make(chan *proto.ProtocolMessage, 16)

	authCallbackCalled := false

	c, _ := ably.NewRealtime(ably.ClientOptions{}.
		AutoConnect(false).
		Token("fake:token").
		AuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
			if authCallbackCalled {
				t.Errorf("expected a single attempt to reauth")
			}
			authCallbackCalled = true
			return nil, authErr
		}).
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

	tokenErr := proto.ErrorInfo{
		StatusCode: 401,
		Code:       40141,
		Message:    "fake token error",
	}

	err = ablytest.ConnWaiter(c, func() {
		in <- &proto.ProtocolMessage{
			Action: proto.ActionDisconnected,
			Error:  &tokenErr,
		}
	}, ably.ConnectionEventDisconnected).Wait()

	if !errors.Is(err, authErr) {
		t.Fatalf("expected the auth error as DISCONNECTED state change reason; got: %s", err)
	}
}

func TestRealtimeConn_RTN15h2_ReauthWithBadToken(t *testing.T) {
	t.Parallel()

	in := make(chan *proto.ProtocolMessage, 1)
	out := make(chan *proto.ProtocolMessage, 16)

	dials := make(chan *url.URL, 1)

	c, _ := ably.NewRealtime(ably.ClientOptions{}.
		Token("fake:token").
		AutoConnect(false).
		AuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
			return ably.TokenString("bad:token"), nil
		}).
		Dial(func(proto string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			dials <- u
			return ablytest.MessagePipe(in, out)(proto, u, timeout)
		}))

	in <- &proto.ProtocolMessage{
		Action:            proto.ActionConnected,
		ConnectionID:      "connection-id",
		ConnectionDetails: &proto.ConnectionDetails{},
	}

	err := ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected).Wait()
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

	if change.Reason.StatusCode != tokenErr.StatusCode || change.Reason.Code != tokenErr.Code {
		t.Fatalf("expected the token error as FAILED state change reason; got: %s", change.Reason)
	}
}

func TestRealtimeConn_RTN15h2_Success(t *testing.T) {
	t.Parallel()

	in := make(chan *proto.ProtocolMessage, 1)
	out := make(chan *proto.ProtocolMessage, 16)

	dials := make(chan *url.URL, 1)

	c, _ := ably.NewRealtime(ably.ClientOptions{}.
		Token("fake:token").
		AutoConnect(false).
		AuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
			return ably.TokenString("good:token"), nil
		}).
		Dial(func(proto string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			dials <- u
			return ablytest.MessagePipe(in, out)(proto, u, timeout)
		}))

	in <- &proto.ProtocolMessage{
		Action:            proto.ActionConnected,
		ConnectionID:      "connection-id",
		ConnectionDetails: &proto.ConnectionDetails{},
	}

	err := ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected).Wait()
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

	if change.Event != ably.ConnectionEventUpdated {
		t.Fatalf("expected UPDATED event; got %v", change)
	}

	// Expect no further events.break
	ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)
}

func TestRealtimeConn_RTN15i_OnErrorWhenConnected(t *testing.T) {
	t.Parallel()

	in := make(chan *proto.ProtocolMessage, 1)
	out := make(chan *proto.ProtocolMessage, 16)

	c, _ := ably.NewRealtime(ably.ClientOptions{}.
		Token("fake:token").
		AutoConnect(false).
		Dial(ablytest.MessagePipe(in, out)))

	// Get the connection to CONNECTED.

	in <- &proto.ProtocolMessage{
		Action:            proto.ActionConnected,
		ConnectionID:      "connection-id",
		ConnectionDetails: &proto.ConnectionDetails{},
	}

	err := ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected).Wait()
	if err != nil {
		t.Fatal(err)
	}

	// Get a channel to ATTACHED.

	channel := c.Channels.Get("test")
	attachWaiter, err := channel.Attach()
	if err != nil {
		t.Fatal(err)
	}

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

	err = ablytest.ConnWaiter(c, func() {
		in <- &proto.ProtocolMessage{
			Action: proto.ActionError,
			Error: &proto.ErrorInfo{
				StatusCode: 500,
				Code:       errorCode,
				Message:    "fake error",
			},
		}
	}, ably.ConnectionEventFailed).Wait()

	var errorInfo *ably.ErrorInfo
	if !errors.As(err, &errorInfo) {
		t.Fatal(err)
	}
	if expected, got := errorCode, errorInfo.Code; expected != got {
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
	app, c := ablytest.NewRealtime(ably.ClientOptions{})
	defer safeclose(t, ablytest.FullRealtimeCloser(c), app)

	err := ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected).Wait()
	if err != nil {
		t.Fatal(err)
	}
	channel := c.Channels.Get("channel")
	if err := ablytest.Wait(channel.Attach()); err != nil {
		t.Fatal(err)
	}
	if err := ablytest.Wait(channel.Publish("name", "data")); err != nil {
		t.Fatal(err)
	}
	prevMsgSerial := c.Connection.MsgSerial()

	client := app.NewRealtime(ably.ClientOptions{}.
		Recover(c.Connection.RecoveryKey()))
	defer safeclose(t, ablytest.FullRealtimeCloser(client))

	err = ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected).Wait()
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
		if true {
			return
		}
	}

	{ //(RTN16c)
		client.Close()
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
		client2 := app.NewRealtime(ably.ClientOptions{}.
			Recover(fakeRecoveryKey).
			Dial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
				query = u.Query()
				return ablyutil.DialWebsocket(protocol, u, timeout)
			}))
		defer safeclose(t, ablytest.FullRealtimeCloser(client2))
		err = ablytest.ConnWaiter(client2, client2.Connect, ably.ConnectionEventConnected).Wait()
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
			if info.Code != code {
				// verify unrecoverable-connection error set in stateChange.reason
				t.Errorf("expected 80000 got %d", info.Code)
			}
			reason := client2.Connection.ErrorReason()
			if reason.Code != code {
				// verify unrecoverable-connection error set in connection.errorReason
				t.Errorf("expected 80000 got %d", reason.Code)
			}
			if serial := client2.Connection.Serial(); serial != -1 {
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
		ConnectionStateTTL: proto.DurationFromMsecs(time.Millisecond * 20),
		MaxIdleInterval:    proto.DurationFromMsecs(time.Millisecond * 5),
	}

	dials := make(chan *url.URL, 1)
	var in chan *proto.ProtocolMessage
	realtimeRequestTimeout := time.Millisecond
	c, _ := ably.NewRealtime(ably.ClientOptions{}.
		AutoConnect(false).
		Token("fake:token").
		RealtimeRequestTimeout(realtimeRequestTimeout).
		Dial(func(p string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			in = make(chan *proto.ProtocolMessage, 1)
			in <- &proto.ProtocolMessage{
				Action:            proto.ActionConnected,
				ConnectionID:      "connection",
				ConnectionDetails: &connDetails,
			}
			dials <- u
			return ablytest.MessagePipe(in, nil,
				ablytest.MessagePipeWithNowFunc(time.Now),
			)(p, u, timeout)
		}))
	err := ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected).Wait()
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

	disconnected := make(chan *ably.ErrorInfo, 1)
	c.Connection.Once(ably.ConnectionEventDisconnected, func(e ably.ConnectionStateChange) {
		disconnected <- e.Reason
	})
	in <- &proto.ProtocolMessage{
		Action: proto.ActionHeartbeat,
	}
	// The connection should not disconnect as we received the heartbeat message
	ablytest.Before(receiveTimeout).NoRecv(t, nil, disconnected, t.Fatalf)

	// RTN23a The connection should be disconnected due to lack of activity past
	// receiveTimeout
	var reason *ably.ErrorInfo
	ablytest.Instantly.Recv(t, &reason, disconnected, t.Fatalf)

	// make sure the reason is timeout
	if !strings.Contains(reason.Error(), "timeout") {
		t.Errorf("expected %q to contain timeout", reason.Error())
	}
}

func TestRealtimeConn_RTN14c(t *testing.T) {
	t.Parallel()
	{
		//(RTN14c)
		connTimeout := make(chan time.Duration, 1)
		client, err := ably.NewRealtime(ably.ClientOptions{}.
			Token("fake:key").
			Dial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
				connTimeout <- timeout
				return ablyutil.DialWebsocket(protocol, u, timeout)
			}))
		if err != nil {
			t.Fatal(err)
		}
		client.Close()
		var got time.Duration
		ablytest.Instantly.Recv(t, &got, connTimeout, t.Fatalf)
		// Check if we passed default request timeout
		expect := 4 * time.Second
		if got != expect {
			t.Errorf("expected %v got %v", expect, connTimeout)
		}
	}
}
func TestRealtimeConn_RTN14a(t *testing.T) {
	t.Parallel()

	t.Run("Missing key", func(t *testing.T) {
		t.Parallel()
		in := make(chan *proto.ProtocolMessage, 1)
		out := make(chan *proto.ProtocolMessage, 16)

		c, _ := ably.NewRealtime(ably.ClientOptions{}.
			Token("fake:token").
			AutoConnect(false).
			Dial(ablytest.MessagePipe(in, out)))
		c.Connect()
		// Respond to connection attempt with a token error.
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
		var state ably.ConnectionStateChange
		ablytest.Instantly.Recv(t, nil, change, t.Fatalf) // skip CONNECTING
		ablytest.Instantly.Recv(t, &state, change, t.Fatalf)
		if expect, got := ably.ConnectionStateDisconnected, state.Current; expect != got {
			t.Errorf("expected %v got %v", expect, got)
		}
		if expect, got := "missing key", c.Connection.ErrorReason(); !strings.Contains(got.Error(), expect) {
			t.Errorf("expected %v to contain %q", got, expect)
		}
	})

	t.Run("Invalid key", func(t *testing.T) {
		t.Parallel()
		in := make(chan *proto.ProtocolMessage, 1)
		out := make(chan *proto.ProtocolMessage, 16)

		c, _ := ably.NewRealtime(ably.ClientOptions{}.
			Token("fake:token").
			AutoConnect(false).
			Key("invalid").
			Dial(ablytest.MessagePipe(in, out)))
		c.Connect()
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
		var state ably.ConnectionStateChange
		ablytest.Instantly.Recv(t, nil, change, t.Fatalf) // skip CONNECTING
		ablytest.Instantly.Recv(t, &state, change, t.Fatalf)
		if expect, got := ably.ConnectionStateDisconnected, state.Current; expect != got {
			t.Errorf("expected %v got %v", expect, got)
		}
		if expect, got := "invalid key", c.Connection.ErrorReason(); !strings.Contains(got.Error(), expect) {
			t.Errorf("expected %v to contain %q", expect, got)
		}
	})

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
		c, _ := ably.NewRealtime(ably.ClientOptions{}.
			AutoConnect(false).
			AuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
				reauth.Store(reauth.Load().(int) + 1)
				if reauth.Load().(int) > 1 {
					return nil, errors.New(bad)
				}
				return ably.TokenString("fake:token"), nil
			}).
			Dial(ablytest.MessagePipe(in, out)))
		c.Connect()
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
		c, _ := ably.NewRealtime(ably.ClientOptions{}.
			AutoConnect(false).
			AuthCallback(func(context.Context, ably.TokenParams) (ably.Tokener, error) {
				reauth.Store(reauth.Load().(int) + 1)
				return ably.TokenString("fake:token"), nil
			}).
			Dial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
				dials.Store(dials.Load().(int) + 1)
				return ablytest.MessagePipe(in, out)(protocol, u, timeout)
			}))
		c.Connect()
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
		c, _ := ably.NewRealtime(ably.ClientOptions{}.
			Token("fake:token").
			AutoConnect(false).
			Dial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
				w, err := ablytest.MessagePipe(in, out)(protocol, u, timeout)
				if err != nil {
					return nil, err
				}
				ls = &closeConn{Conn: w}
				return ls, nil
			}))
		c.Connect()
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
	var count atomic.Value
	count.Store(0)
	in := make(chan *proto.ProtocolMessage)
	out := make(chan *proto.ProtocolMessage, 16)
	ttl := 10 * time.Millisecond
	c, _ := ably.NewRealtime(ably.ClientOptions{}.
		Token("fake:token").
		AutoConnect(false).
		LogLevel(ably.LogDebug).
		ConnectionStateTTL(ttl).
		SuspendedRetryTimeout(ttl / 2).
		DisconnectedRetryTimeout(ttl * 2).
		Dial(func(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
			x := count.Load().(int)
			if x > 3 {
				return ablytest.MessagePipe(in, out)(protocol, u, timeout)
			}
			count.Store(x + 1)
			return nil, context.DeadlineExceeded
		}))
	defer c.Close()
	change := make(ably.ConnStateChanges, 16)
	off := c.Connection.OnAll(change.Receive)
	defer off()

	go c.Connect()
	connected := make(ably.ConnStateChanges, 1)
	clear := c.Connection.On(ably.ConnectionEventConnected, func(csc ably.ConnectionStateChange) {
		connected <- csc
	})
	defer clear()

	in <- &proto.ProtocolMessage{
		Action: proto.ActionConnected,
	}

	// Make sure we have CONNECTED event fired before we inspect state changes
	ablytest.Soon.Recv(t, nil, connected, t.Fatalf)

	// - Start connecting
	// - Get recoverable error, immediately set to disconnected
	// - Stay disconnected long than connectionStateTTL, then tansition to suspended
	// - Periodic start connecting until we successful connect
	// - Transition to Connected after we receive connected action.
	changes := []ably.ConnectionState{
		ably.ConnectionStateConnecting,
		ably.ConnectionStateDisconnected,
		ably.ConnectionStateSuspended, //(RTN14e)
		ably.ConnectionStateConnecting,
		ably.ConnectionStateConnected,
	}
	for _, e := range changes {
		t.Run(e.String(), func(t *testing.T) {
			var state ably.ConnectionStateChange
			ablytest.Instantly.Recv(t, &state, change, t.Fatalf)
			if expect, got := e, state.Current; expect != got {
				t.Errorf("expected %v got %v", expect, got)
			}
		})
	}
	t.Run("(RTN14f)", func(t *testing.T) {
		// We fall into SUSPENDED state after the first dial. We perodically dial 3
		// times until we are successful.
		//
		// This satisfies the check that we stay in SUSPENDED state indefinitely
		x := count.Load().(int)
		if x != 4 {
			t.Errorf("expected 4 failed dials got %d instead ", x)
		}
	})

}

func TestStartupLoop(t *testing.T) {
	opts := ably.ClientOptions{}.
		Key("xxx:xxx").
		LogLevel(ably.LogDebug).
		AutoConnect(false).
		RESTHost("192.168.0.200").
		RealtimeHost("192.168.0.200").
		HTTPOpenTimeout(1 * time.Second)

	if _, err := ably.NewRealtime(opts); err != nil {
		t.Fatal(err)
	}
}
