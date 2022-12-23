package ably

import (
	"context"
	"net/http"
	"time"
)

// Realtime is an ably realtime client that extends the functionality of the [ably.REST] and provides
// additional realtime-specific features.
type Realtime struct {
	// An [ably.Auth] object (RTC4).
	Auth *Auth
	// A [ably.RealtimeChannels] object (RTC3, RTS1).
	Channels *RealtimeChannels
	// A [ably.Connection] object (RTC2).
	Connection *Connection
	rest       *REST
}

// NewRealtime constructs a new [ably.Realtime] client object using an Ably [ably.ClientOption] object (RSC1)
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

// Connect calls Connection.Connect and causes the connection to open, entering the connecting state.
// Explicitly calling Connect() is needed if the ClientOptions.NoConnect is set true (proxy for RTN11).
func (c *Realtime) Connect() {
	c.Connection.Connect()
}

// Close calls Connection.Close and causes the connection to close, entering the closing state. Once closed,
// the library will not attempt to re-establish the connection without an explicit call to Connection.Connect
// proxy for RTN12
func (c *Realtime) Close() {
	c.Connection.Close()
}

// Stats queries the REST /stats API and retrieves your application's usage statistics.
// Returns a [ably.PaginatedResult] object, containing an array of [ably.Stats] objects (RTC5).
//
// See package-level documentation => [ably] Pagination for handling stats pagination.
func (c *Realtime) Stats(o ...StatsOption) StatsRequest {
	return c.rest.Stats(o...)
}

// Time retrieves the time from the Ably service as milliseconds since the Unix epoch.
// Clients that do not have access to a sufficiently well maintained time source and wish to issue Ably
// multiple [ably.TokenRequest] with a more accurate timestamp should use the clientOptions.UseQueryTime property
// instead of this method (RTC6a).
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
