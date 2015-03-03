package realtime

import (
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/protocol"
	"github.com/ably/ably-go/rest"

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
	config.Params

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

func NewConn(params config.Params) *Conn {
	c := &Conn{
		Params: params,
	}
	return c
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

func (c *Conn) Close() {
	c.mtx.Lock()
	c.ID = ""
	c.mtx.Unlock()
	c.ws.Close()
}

func (c *Conn) websocketUrl(token *rest.Token) (*url.URL, error) {
	u, err := url.Parse(c.Params.RealtimeEndpoint + "?access_token=" + token.ID + "&binary=false&timestamp=" + strconv.Itoa(int(time.Now().Unix())))
	if err != nil {
		return nil, err
	}
	return u, err
}

func (c *Conn) Connect() error {
	if c.isActive() {
		return nil
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()

	restClient := rest.NewClient(c.Params)
	token, err := restClient.Auth.RequestToken(60*60, &rest.Capability{"*": []string{"*"}})
	if err != nil {
		return fmt.Errorf("Error fetching token: %s", err)
	}

	u, err := c.websocketUrl(token)
	if err != nil {
		return err
	}

	return c.dial(u)
}

func (c *Conn) dial(u *url.URL) error {
	ws, err := websocket.Dial(u.String(), "", "https://"+u.Host)
	if err != nil {
		return err
	}

	c.Ch = make(chan *protocol.ProtocolMessage)
	c.Err = make(chan error)
	c.ws = ws

	go c.read()
	go c.watchConnectionState()

	return nil
}

func (c *Conn) read() {
	for {
		msg := &protocol.ProtocolMessage{}
		err := websocket.JSON.Receive(c.ws, &msg)
		if err != nil {
			c.Close()
			c.Err <- fmt.Errorf("Failed to get websocket frame: %s", err)
			return
		}
		c.handle(msg)
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
		c.Close()
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
