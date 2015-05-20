package ably

import (
	"sync"
	"time"
)

// The RealtimeClient libraries establish and maintain a persistent connection
// to Ably enabling extremely low latency broadcasting of messages and presence
// state.
type RealtimeClient struct {
	Auth       *Auth
	Channels   *Channels
	Connection *Conn

	opts     ClientOptions
	err      chan error
	rest     *RestClient
	chans    map[string]*RealtimeChannel
	chansMtx sync.RWMutex
}

// NewRealtimeClient
func NewRealtimeClient(options *ClientOptions) (*RealtimeClient, error) {
	if options == nil {
		options = DefaultOptions
	}
	c := &RealtimeClient{
		opts:  *options,
		err:   make(chan error),
		chans: make(map[string]*RealtimeChannel),
	}
	rest, err := NewRestClient(&c.opts)
	if err != nil {
		return nil, err
	}
	conn, err := newConn(&c.opts)
	if err != nil {
		return nil, err
	}
	c.rest = rest
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
