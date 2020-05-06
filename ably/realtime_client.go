package ably

import (
	"sync"
	"time"
)

// The Realtime libraries establish and maintain a persistent connection
// to Ably enabling extremely low latency broadcasting of messages and presence
// state.
type Realtime struct {
	Auth       *Auth
	Channels   *Channels
	Connection *Conn

	chansMtx sync.RWMutex
	chans    map[string]*RealtimeChannel
	rest     *RestClient
	err      chan error
}

var NewRealtimeClient = newRealtime // TODO: remove once tests use functional ClientOptions

// NewRealtime constructs a new RealtimeV12.
func NewRealtime(options ClientOptionsV12) (*Realtime, error) {
	var o ClientOptions
	options.applyWithDefaults(&o)
	return newRealtime(&o)
}

// newRealtime
func newRealtime(opts *ClientOptions) (*Realtime, error) {
	if opts == nil {
		panic("called NewRealtimeClient with nil ClientOptions")
	}

	// Temporarily set defaults here in case this wasn't called from NewRealtimeV12.
	ClientOptionsV12{}.applyWithDefaults(opts)

	c := &Realtime{
		err:   make(chan error),
		chans: make(map[string]*RealtimeChannel),
	}
	rest, err := NewRestClient(opts)
	if err != nil {
		return nil, err
	}
	c.rest = rest
	conn, err := newConn(c.opts(), rest.Auth)
	if err != nil {
		return nil, err
	}
	c.Auth = rest.Auth
	c.Channels = newChannels(c)
	c.Connection = conn
	go c.dispatchloop()
	return c, nil
}

// Close
func (c *Realtime) Close() error {
	return c.Connection.Close()
}

// Stats gives the clients metrics according to the given parameters. The
// returned result can be inspected for the statistics via the Stats()
// method.
func (c *Realtime) Stats(params *PaginateParams) (*PaginatedResult, error) {
	return c.rest.Stats(params)
}

// Time
func (c *Realtime) Time() (time.Time, error) {
	return c.rest.Time()
}

func (c *Realtime) dispatchloop() {
	for msg := range c.Connection.msgCh {
		c.Channels.Get(msg.Channel).notify(msg)
	}
}

func (c *Realtime) opts() *ClientOptions {
	return &c.rest.opts
}

func (c *Realtime) logger() *LoggerOptions {
	return c.rest.logger()
}
