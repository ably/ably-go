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
	Connection *Connection

	chansMtx sync.RWMutex
	chans    map[string]*RealtimeChannel
	rest     *RestClient
	err      chan error
}

// NewRealtime constructs a new Realtime.
func NewRealtime(opts *ClientOptions) (*Realtime, error) {
	if opts == nil {
		panic("called NewRealtime with nil ClientOptions")
	}
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

// ConnectV12 is the same as Connection.Connect.
func (c *Realtime) ConnectV12() {
	c.Connection.ConnectV12()
}

// Close is the same as Connection.Close.
func (c *Realtime) Close() {
	c.Connection.Close()
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
