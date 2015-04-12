package realtime

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ably/ably-go/proto"
)

func NewChannel(name string, client *Client) *Channel {
	return &Channel{
		Name:      name,
		client:    client,
		listeners: make(map[string]map[chan *proto.Message]struct{}),
	}
}

type ChanState int

const (
	ChanStateInitialized ChanState = iota
	ChanStateAttaching
	ChanStateAttached
	ChanStateDetaching
	ChanStateDetached
	ChanStateFailed
)

type Channel struct {
	Name string

	client *Client

	State    ChanState
	stateMtx sync.Mutex

	Err error

	listeners map[string]map[chan *proto.Message]struct{}
	listenMtx sync.RWMutex
}

func (c *Channel) SubscribeTo(event string) chan *proto.Message {
	ch := make(chan *proto.Message)
	c.listenMtx.Lock()
	if _, ok := c.listeners[event]; !ok {
		c.listeners[event] = make(map[chan *proto.Message]struct{})
	}
	c.listeners[event][ch] = struct{}{}
	c.listenMtx.Unlock()
	go c.Attach()
	return ch
}

func (c *Channel) Unsubscribe(event string, ch chan *proto.Message) {
	c.listenMtx.Lock()
	delete(c.listeners[event], ch)
	if len(c.listeners[event]) == 0 {
		delete(c.listeners, event)
	}
	c.listenMtx.Unlock()
	close(ch)
}

func (c *Channel) Publish(name string, data string) error {
	c.Attach()
	msg := &proto.ProtocolMessage{
		Action:  proto.ActionMessage,
		Channel: c.Name,
		Messages: []*proto.Message{
			{Name: name, Data: data},
		},
	}
	return c.client.send(msg)
}

func (c *Channel) notify(msg *proto.ProtocolMessage) {
	switch msg.Action {
	case proto.ActionAttached:
		c.setState(ChanStateAttached)
	case proto.ActionDetached:
		c.setState(ChanStateDetached)
	case proto.ActionPresence:
		// TODO
	case proto.ActionError:
		c.setState(ChanStateFailed)
		c.Err = msg.Error
		// TODO c.Close()
	case proto.ActionMessage:
		c.listenMtx.RLock()
		defer c.listenMtx.RUnlock()
		for _, m := range msg.Messages {
			if l, ok := c.listeners[""]; ok {
				for ch := range l {
					ch <- m
				}
			}
			if l, ok := c.listeners[m.Name]; ok {
				for ch := range l {
					ch <- m
				}
			}
		}
	default:
	}
}

func (c *Channel) Attach() {
	c.stateMtx.Lock()
	defer c.stateMtx.Unlock()
	if c.State == ChanStateAttaching || c.State == ChanStateAttached {
		return
	}
	if !c.client.isActive() {
		c.Err = errors.New("Connection not active")
		return
	}
	msg := &proto.ProtocolMessage{Action: proto.ActionAttach, Channel: c.Name}
	if err := c.client.send(msg); err != nil {
		c.Err = fmt.Errorf("Attach request failed: %s", err)
		return
	}
	c.State = ChanStateAttaching
}

func (c *Channel) setState(s ChanState) {
	c.stateMtx.Lock()
	defer c.stateMtx.Unlock()
	c.State = s
}
