package ably

import (
	"fmt"
	"sync"

	"github.com/ably/ably-go/ably/proto"
)

func NewRealtimeClient(params Params) *RealtimeClient {
	params.Prepare()
	c := &RealtimeClient{
		Params:     params,
		Err:        make(chan error),
		rest:       NewRestClient(params),
		channels:   make(map[string]*RealtimeChannel),
		Connection: NewConn(params),
	}
	go c.connect()
	return c
}

type RealtimeClient struct {
	Params
	Err chan error

	rest *RestClient

	Connection *Conn

	channels map[string]*RealtimeChannel
	chanMtx  sync.RWMutex
}

func (c *RealtimeClient) Close() {
	c.Connection.Close()
}

func (c *RealtimeClient) RealtimeChannel(name string) *RealtimeChannel {
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
			c.RealtimeChannel(msg.Channel).notify(msg)
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
