package realtime

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ably/ably-go/protocol"
)

func NewChannel(name string, client *Client) *Channel {
	return &Channel{
		Name:      name,
		client:    client,
		listeners: make(map[string]map[chan *protocol.Message]struct{}),
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

	listeners map[string]map[chan *protocol.Message]struct{}
	listenMtx sync.RWMutex
}

func (c *Channel) Subscribe(event string) chan *protocol.Message {
	ch := make(chan *protocol.Message)
	c.listenMtx.Lock()
	if _, ok := c.listeners[event]; !ok {
		c.listeners[event] = make(map[chan *protocol.Message]struct{})
	}
	c.listeners[event][ch] = struct{}{}
	c.listenMtx.Unlock()
	go c.attach()
	return ch
}

func (c *Channel) Unsubscribe(event string, ch chan *protocol.Message) {
	c.listenMtx.Lock()
	delete(c.listeners[event], ch)
	if len(c.listeners[event]) == 0 {
		delete(c.listeners, event)
	}
	c.listenMtx.Unlock()
	close(ch)
}

func (c *Channel) Publish(name string, data interface{}) error {
	c.attach()
	msg := &protocol.ProtocolMessage{
		Action:  protocol.ActionMessage,
		Channel: c.Name,
		Messages: []*protocol.Message{
			{Name: name, Data: data},
		},
	}
	return c.client.send(msg)
}

func (c *Channel) notify(msg *protocol.ProtocolMessage) {
	switch msg.Action {
	case protocol.ActionAttached:
		c.setState(ChanStateAttached)
	case protocol.ActionDetached:
		c.setState(ChanStateDetached)
	case protocol.ActionPresence:
		// TODO
	case protocol.ActionError:
		c.setState(ChanStateFailed)
		c.Err = msg.Error
		// TODO c.Close()
	case protocol.ActionMessage:
		c.listenMtx.RLock()
		defer c.listenMtx.RUnlock()
		for _, m := range msg.Messages {
			if l, ok := c.listeners[""]; ok {
				for ch, _ := range l {
					ch <- m
				}
			}
			if l, ok := c.listeners[m.Name]; ok {
				for ch, _ := range l {
					ch <- m
				}
			}
		}
	default:
	}
}

func (c *Channel) attach() {
	c.stateMtx.Lock()
	defer c.stateMtx.Unlock()
	if c.State == ChanStateAttaching || c.State == ChanStateAttached {
		return
	}
	if !c.client.isActive() {
		c.Err = errors.New("Connection not active")
		return
	}
	msg := &protocol.ProtocolMessage{Action: protocol.ActionAttach, Channel: c.Name}
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
