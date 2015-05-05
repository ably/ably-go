package ably

import (
	"fmt"
	"sync"

	"github.com/ably/ably-go/ably/proto"
)

func NewRealtimeClient(options ClientOptions) (*RealtimeClient, error) {
	rest, err := NewRestClient(&options)
	if err != nil {
		return nil, err
	}
	c := &RealtimeClient{
		ClientOptions: options,
		Err:           make(chan error),
		rest:          rest,
		channels:      make(map[string]*RealtimeChannel),
		Connection:    NewConn(options),
	}
	go c.connect()
	return c, nil
}

type RealtimeClient struct {
	ClientOptions
	Connection *Conn
	Err        chan error

	rest     *RestClient
	channels map[string]*RealtimeChannel
	chanMtx  sync.RWMutex
}

func (c *RealtimeClient) Close() {
	c.Connection.Close()
}

func (c *RealtimeClient) Channel(name string) *RealtimeChannel {
	c.chanMtx.Lock()
	defer c.chanMtx.Unlock()
	if ch, ok := c.channels[name]; ok {
		return ch
	}
	ch := NewRealtimeChannel(name, c)
	c.channels[name] = ch
	return ch
}

func (c *RealtimeClient) connect() {
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

func (c *RealtimeClient) send(msg *proto.ProtocolMessage) error {
	return c.Connection.send(msg)
}

func (c *RealtimeClient) isActive() bool {
	return c.Connection.isActive()
}
