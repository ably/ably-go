package ably

import (
	"errors"
	"net/url"
	"strconv"
	"time"

	"github.com/ably/ably-go/ably/proto"

	"github.com/ably/ably-go/Godeps/_workspace/src/code.google.com/p/go.net/websocket"
	"github.com/ably/ably-go/Godeps/_workspace/src/gopkg.in/vmihailenco/msgpack.v2"
)

type msgerr struct {
	msg proto.ProtocolMessage
	err error
}

// Conn represents a single connection RealtimeClient instantiates for
// communication with Ably servers.
type Conn struct {
	id        string
	key       string
	serial    int64
	msgSerial int64
	err       error
	conn      MsgConn
	connCh    chan *msgerr
	msgCh     chan *proto.ProtocolMessage
	opts      *ClientOptions
	state     *stateEmitter
	stateCh   chan State
	pending   pendingEmitter
	queue     *msgQueue
}

func newConn(opts *ClientOptions) (*Conn, error) {
	c := &Conn{
		opts:   opts,
		connCh: make(chan *msgerr),
		msgCh:  make(chan *proto.ProtocolMessage),
		state:  newStateEmitter(StateConn, StateConnInitialized, ""),
	}
	c.queue = newMsgQueue(c)
	if opts.Listener != nil {
		c.On(opts.Listener)
	}
	if !opts.NoConnect {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Conn) dial(proto string, u *url.URL) (MsgConn, error) {
	if c.opts.Dial != nil {
		return c.opts.Dial(proto, u)
	}
	return dialWebsocket(proto, u)
}

func booltext(b ...bool) []string {
	ok := true
	for _, b := range b {
		ok = ok && b
	}
	if ok {
		return []string{"true"}
	}
	return []string{"false"}
}

// Connect is used to connect to Ably servers manually, when the client owning
// the connection was created with NoConnect option. The connect request is
// being processed on a separate goroutine.
//
// In order to receive its result register a channel with the On method; upon
// succsufful connection channel will receive StatConnConnected event.
//
// If client is already connected, this method is a nop.
func (c *Conn) Connect() error {
	if c.isActive() {
		return nil
	}
	c.state.Lock()
	defer c.state.Unlock()
	c.state.set(StateConnConnecting, nil)
	u, err := url.Parse(c.opts.realtimeURL())
	if err != nil {
		err = newError(50000, err)
		c.state.set(StateConnFailed, err)
		return err
	}
	rest, err := NewRestClient(c.opts)
	if err != nil {
		c.state.set(StateConnFailed, err)
		return err
	}
	token, err := rest.Auth.RequestToken(nil)
	if err != nil {
		c.state.set(StateConnFailed, err)
		return err
	}
	proto := c.opts.protocol()
	query := url.Values{
		"access_token": {token.Token},
		"binary":       booltext(proto == ProtocolMsgPack),
		"timestamp":    []string{strconv.FormatInt(TimestampNow(), 10)},
		"echo":         booltext(!c.opts.NoEcho),
	}
	u.RawQuery = query.Encode()
	conn, err := c.dial(proto, u)
	if err != nil {
		err = newError(ErrCodeConnectionFailed, err)
		c.state.set(StateConnFailed, err)
		return err
	}
	c.setConn(conn)
	return nil
}

// Close initiates closing sequence for the connection, which runs asynchronously
// on a separate goroutine.
//
// In order to receive the result of the requset register a channel with the
// On method; upon successful Close channel will receive StatConnClosed event.
//
// If connection is already closed, this method is a nop.
func (c *Conn) Close() error {
	return errors.New("TODO")
}

// ID gives unique ID string obtained from Ably upon successful connection.
// The ID may change due to reconnection and recovery; on every received
// StateConnConnected event previously obtained ID is no longer valid.
func (c *Conn) ID() string {
	c.state.Lock()
	defer c.state.Unlock()
	return c.id
}

// Key gives unique key string obtained from Ably upon successful connection.
// The key may change due to reconnection and recovery; on every received
// StatConnConnected event previously obtained Key is no longer valid.
func (c *Conn) Key() string {
	c.state.Lock()
	defer c.state.Unlock()
	return c.key
}

// Ping issues a ping request against configured endpoint and returns TTR times
// for ping request and pong response.
//
// Ping returns non-nil error without any attemp of communication with Ably
// if the connection state is StateConnClosed or StateConnFailed.
func (c *Conn) Ping() (ping, pong time.Duration, err error) {
	return 0, 0, errors.New("TODO")
}

// Reason gives last known error that caused connection transit to
// StateConnFailed state.
func (c *Conn) Reason() error {
	c.state.Lock()
	err := c.state.err
	c.state.Unlock()
	return err
}

// Serial gives serial number of a message received most recently. Last known
// serial number is used when recovering connection state.
func (c *Conn) Serial() int64 {
	c.state.Lock()
	serial := c.serial
	c.state.Unlock()
	return serial
}

// State returns current state of the connection.
func (c *Conn) State() int {
	c.state.Lock()
	state := c.state.current
	c.state.Unlock()
	return state
}

// On relays request connection states to the given channel; on state transition
// connection will not block sending to c - the caller must ensure the incoming
// values are read at proper pace or the c is sufficiently buffered.
//
// If no states are given, c is registered for all of them.
// If c is nil, the method panics.
// If c is alreadt registered, its state set is expanded.
func (c *Conn) On(ch chan<- State, states ...int) {
	c.state.on(ch, states...)
}

// Off removes c from listetning on the given connection state transitions.
//
// If no states are given, c is removed for all of the connection's states.
// If c is nil, the method panics.
// If c was not registered or is already removed, the method is a nop.
func (c *Conn) Off(ch chan<- State, states ...int) {
	c.state.off(ch, states...)
}

var errQueueing = &Error{Code: 40000, Err: errors.New("unable to send messages in current state with disabled queueing")}

func (c *Conn) send(msg *proto.ProtocolMessage, listen chan<- error) error {
	const maxint64 = 1<<63 - 1
	c.state.Lock()
	switch c.state.current {
	case StateConnInitialized, StateConnConnecting, StateConnDisconnected:
		c.state.Unlock()
		if c.opts.NoQueueing {
			return errQueueing
		}
		c.queue.Enqueue(msg, listen)
		return nil
	case StateConnConnected:
	default:
		c.state.Unlock()
		return &Error{Code: 80000}
	}
	msg.MsgSerial = c.msgSerial
	c.msgSerial = (c.msgSerial + 1) % maxint64
	if listen != nil {
		c.pending.Enqueue(msg.MsgSerial, listen)
	}
	c.state.Unlock()
	return c.conn.Send(msg)
}

func (c *Conn) isActive() bool {
	c.state.Lock()
	defer c.state.Unlock()
	return c.state.current == StateConnConnecting || c.state.current == StateConnConnected
}

func (c *Conn) setConn(conn MsgConn) {
	c.conn = conn
	go c.eventloop()
	go c.msgloop()
}

func (c *Conn) msgloop() {
	for {
		me := &msgerr{}
		me.err = c.conn.Receive(&me.msg)
		c.connCh <- me
		if me.err != nil {
			return // TODO recovery
		}
	}
}

func (c *Conn) eventloop() {
	for ret := range c.connCh {
		if ret.err != nil {
			c.state.Lock()
			c.state.set(StateConnFailed, newError(80000, ret.err))
			c.state.Unlock()
			return // TODO recovery
		}
		if ret.msg.ConnectionSerial != 0 {
			c.state.Lock()
			c.serial = ret.msg.ConnectionSerial
			c.state.Unlock()
		}
		switch ret.msg.Action {
		case proto.ActionHeartbeat:
		case proto.ActionAck:
			c.state.Lock()
			c.pending.Ack(ret.msg.MsgSerial, ret.msg.Count, newErrorProto(ret.msg.Error))
			c.serial++
			c.state.Unlock()
		case proto.ActionNack:
			c.state.Lock()
			c.pending.Nack(ret.msg.MsgSerial, ret.msg.Count, newErrorProto(ret.msg.Error))
			c.state.Unlock()
		case proto.ActionError:
			if ret.msg.Channel != "" {
				c.msgCh <- &ret.msg
				break
			}
			c.state.Lock()
			c.state.set(StateConnFailed, newErrorProto(ret.msg.Error))
			c.state.Unlock()
			c.queue.Fail(newErrorProto(ret.msg.Error))
		case proto.ActionConnected:
			c.state.Lock()
			c.id = ret.msg.ConnectionId
			c.serial = -1
			c.msgSerial = 0
			c.state.set(StateConnConnected, nil)
			c.state.Unlock()
			c.queue.Flush()
		case proto.ActionDisconnected:
			c.state.Lock()
			c.id = ""
			c.state.set(StateConnDisconnected, nil)
			c.state.Unlock()
		default:
			c.msgCh <- &ret.msg
		}
	}
}

// MsgConn represents a message-oriented connection.
type MsgConn interface {
	// Send write the given message to the connection.
	// It is expected to block until whole message is written.
	Send(msg interface{}) error

	// Receive reads the given message from the connection.
	// It is expected to block until whole message is read.
	Receive(msg interface{}) error

	// Close closes the connection.
	Close() error
}

func dialWebsocket(proto string, u *url.URL) (MsgConn, error) {
	ws := &wsConn{}
	switch proto {
	case ProtocolJSON:
		ws.codec = websocket.JSON
	case ProtocolMsgPack:
		ws.codec = msgpackCodec
	default:
		return nil, errors.New(`invalid protocol "` + proto + `"`)
	}
	conn, err := websocket.Dial(u.String(), "", "https://"+u.Host)
	if err != nil {
		return nil, err
	}
	ws.conn = conn
	return ws, nil
}

var msgpackCodec = websocket.Codec{
	Marshal: func(v interface{}) ([]byte, byte, error) {
		p, err := msgpack.Marshal(v)
		return p, websocket.BinaryFrame, err
	},
	Unmarshal: func(p []byte, _ byte, v interface{}) error {
		return msgpack.Unmarshal(p, v)
	},
}

type wsConn struct {
	conn  *websocket.Conn
	codec websocket.Codec
}

func (ws *wsConn) Send(v interface{}) error {
	return ws.codec.Send(ws.conn, v)
}

func (ws *wsConn) Receive(v interface{}) error {
	return ws.codec.Receive(ws.conn, v)
}

func (ws *wsConn) Close() error {
	return ws.conn.Close()
}
