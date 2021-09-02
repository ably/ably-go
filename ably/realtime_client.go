package ably

import (
	"context"
	"net/http"
	"time"
)

// The Realtime libraries establish and maintain a persistent connection
// to Ably enabling extremely low latency broadcasting of messages and presence
// state.
type Realtime struct {
	Auth       *Auth
	Channels   *RealtimeChannels
	Connection *Connection

	rest *REST
}

// NewRealtime constructs a new Realtime.
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

// Connect is the same as Connection.Connect.
func (c *Realtime) Connect() {
	c.Connection.Connect()
}

// Close is the same as Connection.Close.
func (c *Realtime) Close() {
	c.Connection.Close()
}

// Stats is the same as REST.Stats.
func (c *Realtime) Stats(o ...StatsOption) StatsRequest {
	return c.rest.Stats(o...)
}

// Time
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
