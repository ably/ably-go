package realtime

import (
	"fmt"
	"sync"

	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/proto"
	"github.com/ably/ably-go/rest"
)

func NewClient(params config.Params) *Client {
	c := &Client{
		Params:     params,
		Err:        make(chan error),
		rest:       rest.NewClient(params),
		channels:   make(map[string]*Channel),
		Connection: NewConn(params),
	}
	go c.connect()
	return c
}

type Client struct {
	config.Params
	Err chan error

	rest *rest.Client

	Connection *Conn

	channels map[string]*Channel
	chanMtx  sync.RWMutex
}

func (c *Client) Close() {
	c.Connection.Close()
}

func (c *Client) Channel(name string) *Channel {
	c.chanMtx.Lock()
	defer c.chanMtx.Unlock()

	if ch, ok := c.channels[name]; ok {
		return ch
	}

	ch := NewChannel(name, c)
	c.channels[name] = ch
	return ch
}

func (c *Client) connect() {
	err := c.Connection.Connect()

	if err != nil {
		c.Err <- fmt.Errorf("Connection error : %s", err)
		return
	}

	for {
		select {
		case msg := <-c.Connection.Ch:
			c.Channel(msg.Channel).notify(msg)
		case err := <-c.Connection.Err:
			c.Close()
			c.Err <- err
			return
		}
	}
}

func (c *Client) send(msg *proto.ProtocolMessage) error {
	return c.Connection.send(msg)
}

func (c *Client) isActive() bool {
	return c.Connection.isActive()
}
