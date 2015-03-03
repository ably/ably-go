package realtime

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/ably/ably-go/protocol"

	"github.com/ably/ably-go/Godeps/_workspace/src/code.google.com/p/go.net/websocket"
)

type ConnState int

const (
	ConnStateInitialized ConnState = iota
	ConnStateConnecting
	ConnStateConnected
	ConnStateDisconnected
	ConnStateSuspended
	ConnStateClosed
	ConnStateFailed
)

type ConnListener func()

type Conn struct {
	ID        string
	State     ConnState
	stateChan chan ConnState
	Ch        chan *protocol.ProtocolMessage
	Err       chan error
	ws        *websocket.Conn
	mtx       sync.RWMutex

	listeners map[ConnState][]ConnListener
	lmtx      sync.RWMutex
}

func (c *Conn) isActive() bool {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	switch c.State {
	case ConnStateInitialized,
		ConnStateConnecting,
		ConnStateConnected,
		ConnStateDisconnected:
		return true
	default:
		return false
	}
}

func (c *Conn) send(msg *protocol.ProtocolMessage) error {
	return websocket.JSON.Send(c.ws, msg)
}

func (c *Conn) close() {
	c.mtx.Lock()
	c.ID = ""
	c.mtx.Unlock()
	c.ws.Close()
}

func (c *Conn) Dial(w string) error {
	u, err := url.Parse(w)
	if err != nil {
		return err
	}
	ws, err := websocket.Dial(w, "", "https://"+u.Host)
	if err != nil {
		return err
	}

	c.Ch = make(chan *protocol.ProtocolMessage)
	c.Err = make(chan error)
	c.ws = ws

	go c.read()
	return nil
}

func (c *Conn) read() {
	for {
		msg := &protocol.ProtocolMessage{}
		err := websocket.JSON.Receive(c.ws, &msg)
		if err != nil {
			c.close()
			c.Err <- fmt.Errorf("Failed to get websocket frame: %s", err)
			return
		}
		c.handle(msg)
		go c.watchConnectionState()
	}
}

func (c *Conn) watchConnectionState() {
	for {
		connState := <-c.stateChan
		c.trigger(connState)
	}
}

func (c *Conn) trigger(connState ConnState) {
	for i := range c.listeners[connState] {
		go c.listeners[connState][i]()
	}
}

func (c *Conn) On(connState ConnState, listener ConnListener) {
	c.lmtx.Lock()
	defer c.lmtx.Unlock()

	if c.listeners == nil {
		c.listeners = make(map[ConnState][]ConnListener)
	}

	if c.listeners[connState] == nil {
		c.listeners[connState] = []ConnListener{}
	}

	c.listeners[connState] = append(c.listeners[connState], listener)
}

func (c *Conn) handle(msg *protocol.ProtocolMessage) {
	switch msg.Action {
	case protocol.ActionHeartbeat, protocol.ActionAck, protocol.ActionNack:
		return
	case protocol.ActionError:
		if msg.Channel != "" {
			c.Ch <- msg
			return
		}
		c.close()
		c.Err <- msg.Error
		return
	case protocol.ActionConnected:
		c.mtx.Lock()
		c.ID = msg.ConnectionId
		c.State = ConnStateConnected
		c.stateChan <- ConnStateConnected
		c.mtx.Unlock()
		return
	case protocol.ActionDisconnected:
		c.mtx.Lock()
		c.ID = ""
		c.State = ConnStateDisconnected
		c.stateChan <- ConnStateDisconnected
		c.mtx.Unlock()
		return
	default:
		c.Ch <- msg
	}
}
