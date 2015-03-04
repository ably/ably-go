package realtime

import (
	"fmt"
	"log"
	"net/url"
	"sync"

	"github.com/ably/ably-go/protocol"

	"github.com/ably/ably-go/Godeps/_workspace/src/code.google.com/p/go.net/websocket"
)

func Dial(w string) (*Conn, error) {
	u, err := url.Parse(w)
	if err != nil {
		return nil, err
	}
	ws, err := websocket.Dial(w, "", "https://"+u.Host)
	if err != nil {
		return nil, err
	}
	c := &Conn{
		ws:  ws,
		Ch:  make(chan *protocol.ProtocolMessage),
		Err: make(chan error),
	}
	go c.read()
	return c, nil
}

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

type Conn struct {
	ID    string
	State ConnState
	Ch    chan *protocol.ProtocolMessage
	Err   chan error
	ws    *websocket.Conn
	mtx   sync.RWMutex
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
	}
}

func (c *Conn) handle(msg *protocol.ProtocolMessage) {
	switch msg.Action {
	case protocol.ActionHeartbeat,
		protocol.ActionAck,
		protocol.ActionNack:
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
		log.Println("connected!")
		c.mtx.Lock()
		c.ID = msg.ConnectionId
		c.State = ConnStateConnected
		c.mtx.Unlock()
		return
	case protocol.ActionDisconnected:
		c.mtx.Lock()
		c.ID = ""
		c.State = ConnStateDisconnected
		c.mtx.Unlock()
		return
	default:
		c.Ch <- msg
	}
}
