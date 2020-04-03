package ably

import (
	"sync"
	"time"
)

// The RealtimeV12 libraries establish and maintain a persistent connection
// to Ably enabling extremely low latency broadcasting of messages and presence
// state.
type RealtimeV12 = RealtimeClient

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

// NewRealtimeV12 constructs a new RealtimeV12.
func NewRealtimeV12(options ClientOptionsV12) (*RealtimeV12, error) {
	var o ClientOptions
	for _, setOption := range options {
		setOption(&o)
	}
	return NewRealtimeClient(&o)
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

func (c *RealtimeClient) dispatchloop() {
	for msg := range c.Connection.msgCh {
		c.Channels.Get(msg.Channel).notify(msg)
	}
}

func (c *RealtimeClient) opts() *ClientOptions {
	return &c.rest.opts
}

func (c *RealtimeClient) logger() *LoggerOptions {
	return c.rest.logger()
}
