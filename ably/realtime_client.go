package ably

import (
	"context"
	"net/http"
	"time"
)
// **LEGACY**
// The Realtime libraries establish and maintain a persistent connection
// to Ably enabling extremely low latency broadcasting of messages and presence
// state.
type Realtime struct {
	// **CANONICAL**
	// An [Auth]{@link Auth} object.
	// RTC4
	Auth       *Auth
	// **CANONICAL**
	// A [Channels]{@link Channels} object.
	// RTC3, RTS1
	Channels   *RealtimeChannels
	// **CANONICAL**
	// A [Connection]{@link Connection} object.
	// RTC2
	Connection *Connection
	rest *REST
}
// **LEGACY**
// NewRealtime constructs a new Realtime.
// **CANONICAL**
// Constructs a RealtimeClient object using an Ably [ClientOptions]{@link ClientOptions} object.
// options - A [ClientOptions]{@link ClientOptions} object.
// RSC1
func NewRealtime(options ...ClientOption) (*Realtime, error) {
	c := &Realtime{}
	rest, err := NewREST(options...) //options validated in NewREST
	if err != nil {
		return nil, err
	}
	c.rest = rest
	c.Auth = rest.Auth
	c.Channels = newChannels(c)
	conn := newConn(c.opts(), rest.Auth, connCallbacks{
		c.onChannelMsg,
		c.onReconnected,
		c.onReconnectionFailed,
	})
	conn.internalEmitter.OnAll(func(change ConnectionStateChange) {
		c.Channels.broadcastConnStateChange(change)
	})
	c.Connection = conn
	return c, nil
}

// **LEGACY**
// Connect is the same as Connection.Connect.
// **CANONICAL**
// Calls [connection.connect()]{@link Connection#connect} and causes the connection to open, entering the connecting state.
// Explicitly calling connect() is unnecessary unless the [autoConnect]{@link ClientOptions#autoConnect} property is disabled.
// proxy for RTN11
func (c *Realtime) Connect() {
	c.Connection.Connect()
}

// **LEGACY**
// Close is the same as Connection.Close.
// **CANONICAL**
// Calls [connection.close()]{@link Connection#close} and causes the connection to close, entering the closing state.
// Once closed, the library will not attempt to re-establish the connection without an explicit call to [connect()]{@link Connection#connect}.
// proxy for RTN12
func (c *Realtime) Close() {
	c.Connection.Close()
}

// **LEGACY**
// Stats is the same as REST.Stats.
// **CANONICAL**
// Queries the REST /stats API and retrieves your application's usage statistics. Returns a [PaginatedResult]{@link PaginatedResult} object, containing an array of [Stats]{@link Stats} objects. See the Stats docs.
// Returns A [PaginatedResult]{@link PaginatedResult} object containing an array of [Stats]{@link Stats} objects.
// RTC5
func (c *Realtime) Stats(o ...StatsOption) StatsRequest {
	return c.rest.Stats(o...)
}

// **LEGACY**
// Time
// **CANONICAL**
// Retrieves the time from the Ably service as milliseconds since the Unix epoch.
// Clients that do not have access to a sufficiently well maintained time source and wish to issue Ably [TokenRequests]{@link TokenRequest with a more accurate timestamp should use the [queryTime]{@link ClientOptions#queryTime} property instead of this method.
// returns The time as milliseconds since the Unix epoch.
//RTC6a
func (c *Realtime) Time(ctx context.Context) (time.Time, error) {
	return c.rest.Time(ctx)
}

func (c *Realtime) onChannelMsg(msg *protocolMessage) {
	c.Channels.Get(msg.Channel).notify(msg)
}

func (c *Realtime) onReconnected(isNewID bool) {
	if !isNewID /* RTN15c3, RTN15g3 */ {
		// No need to reattach: state is preserved. We just need to flush the
		// queue of pending messages.
		for _, ch := range c.Channels.Iterate() {
			ch.queue.Flush()
		}
		//RTN19a
		c.Connection.resendPending()
		return
	}

	for _, ch := range c.Channels.Iterate() {
		switch ch.State() {
		// TODO: SUSPENDED
		case ChannelStateAttaching, ChannelStateAttached: //RTN19b
			ch.mayAttach(false)
		case ChannelStateDetaching: //RTN19b
			ch.detachSkipVerifyActive()
		}
	}
	//RTN19a
	c.Connection.resendPending()
}

func (c *Realtime) onReconnectionFailed(err *errorInfo) {
	for _, ch := range c.Channels.Iterate() {
		ch.setState(ChannelStateFailed, newErrorFromProto(err), false)
	}
}

func isTokenError(err *errorInfo) bool {
	return err != nil && err.StatusCode == http.StatusUnauthorized && (40140 <= err.Code && err.Code < 40150)
}

func (c *Realtime) opts() *clientOptions {
	return c.rest.opts
}

func (c *Realtime) log() logger {
	return c.rest.log
}
