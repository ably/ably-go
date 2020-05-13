package ably

import (
	"net/http"
	"time"

	"github.com/ably/ably-go/ably/proto"
)

// The RealtimeClient libraries establish and maintain a persistent connection
// to Ably enabling extremely low latency broadcasting of messages and presence
// state.
type RealtimeClient struct {
	Auth       *Auth
	Channels   *Channels
	Connection *Conn

	rest *RestClient
}

// NewRealtimeClient
func NewRealtimeClient(opts *ClientOptions) (*RealtimeClient, error) {
	if opts == nil {
		panic("called NewRealtimeClient with nil ClientOptions")
	}
	c := &RealtimeClient{}
	rest, err := NewRestClient(opts)
	if err != nil {
		return nil, err
	}
	c.rest = rest
	c.Auth = rest.Auth
	c.Channels = newChannels(c)
	conn, err := newConn(c.opts(), rest.Auth, connCallbacks{
		c.onChannelMsg, c.onReconnectMsg, c.onConnStateChange,
	})
	if err != nil {
		return nil, err
	}
	c.Connection = conn
	return c, nil
}

// Close
func (c *RealtimeClient) Close() error {
	return c.Connection.Close()
}

// Stats gives the clients metrics according to the given parameters. The
// returned result can be inspected for the statistics via the Stats()
// method.
func (c *RealtimeClient) Stats(params *PaginateParams) (*PaginatedResult, error) {
	return c.rest.Stats(params)
}

// Time
func (c *RealtimeClient) Time() (time.Time, error) {
	return c.rest.Time()
}

func (c *RealtimeClient) onChannelMsg(msg *proto.ProtocolMessage) {
	c.Channels.Get(msg.Channel).notify(msg)
}

func (c *RealtimeClient) onReconnectMsg(msg *proto.ProtocolMessage) {
	switch msg.Action {
	case proto.ActionConnected:
		if msg.Error != nil {
			// (RTN15c3)
			for _, ch := range c.Channels.All() {
				switch ch.State() {
				case StateConnSuspended:
					ch.attach(false)
				case StateChanAttaching, StateChanAttached:
					ch.mayAttach(false, false)
				}
			}
		}

	case proto.ActionError:
		// (RTN15c4)

		for _, ch := range c.Channels.All() {
			if ch.State() == StateChanAttached {
				ch.state.syncSet(StateChanFailed, msg.Error)
			}

		}
	}
}

func tokenError(err *proto.ErrorInfo) bool {
	return err.StatusCode == http.StatusUnauthorized && (40140 <= err.Code && err.Code < 40150)
}

func (c *RealtimeClient) onConnStateChange(state State) {
	// TODO: Replace with EventEmitter https://github.com/ably/ably-go/pull/144
	c.Channels.broadcastConnStateChange(state)
}

func (c *RealtimeClient) opts() *ClientOptions {
	return &c.rest.opts
}

func (c *RealtimeClient) logger() *LoggerOptions {
	return c.rest.logger()
}
