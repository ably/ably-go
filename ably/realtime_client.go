package ably

import (
	"sync"
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

	chansMtx sync.RWMutex
	chans    map[string]*RealtimeChannel
	rest     *RestClient
	err      chan error
}

// NewRealtimeClient
func NewRealtimeClient(opts *ClientOptions) (*RealtimeClient, error) {
	if opts == nil {
		panic("called NewRealtimeClient with nil ClientOptions")
	}
	c := &RealtimeClient{
		err:   make(chan error),
		chans: make(map[string]*RealtimeChannel),
	}
	rest, err := NewRestClient(opts)
	if err != nil {
		return nil, err
	}
	c.rest = rest
	c.Auth = rest.Auth
	c.Channels = newChannels(c)
	conn, err := newConn(c.opts(), rest.Auth, c.onChannelMsg, c.onReconnectMsg, c.onConnStateChange)
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
		if c.Connection.id == msg.ConnectionID {
			if msg.Error != nil {
				// (RTN15c2)
				c.Connection.state.Lock()
				c.Connection.setState(StateConnConnected, msg.Error)

				// According to the spec we need to set Connection#errorReason but Conn
				// doesn't have errorReason field.However it has Reason method that
				// returns the last know error which in our case will be be msg.Error so
				// it will be conforming to spec.
				//
				// Adding Conn.errorReason won't work in our case as it will be private
				// according to Go conventions. So, I'm choosing to only set state with
				// appropriate reason and assuming users will use Conn.Reason to access
				// the error.
				//
				// TODO: Add Conn.ErrorReason  field.
				c.Connection.state.Unlock()
				return
			}
			// (RTN15c1)
			return
		}
		if msg.Error != nil {
			// (RTN15c3)
			// process all queued messages. No state changes here.
			c.Channels.client.Connection.queue.Flush()
			return
		}
	case proto.ActionError:
		// (RTN15c5)
		// (RTN15c4) ?
	default:
		// We have received unexpected message here. We are in failure state
	}
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
