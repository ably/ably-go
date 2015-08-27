package ably

import (
	"errors"
	"net/url"
	"strconv"
	"time"

	"github.com/ably/ably-go/ably/proto"

	"github.com/ably/ably-go/Godeps/_workspace/src/golang.org/x/net/websocket"
	"github.com/ably/ably-go/Godeps/_workspace/src/gopkg.in/vmihailenco/msgpack.v2"
)

// Conn represents a single connection RealtimeClient instantiates for
// communication with Ably servers.
type Conn struct {
	id        string
	key       string
	serial    int64
	msgSerial int64
	err       error
	conn      MsgConn
	msgCh     chan *proto.ProtocolMessage
	opts      *ClientOptions
	state     *stateEmitter
	stateCh   chan State
	pending   pendingEmitter
	queue     *msgQueue
}

func newConn(opts *ClientOptions) (*Conn, error) {
	c := &Conn{
		opts:  opts,
		msgCh: make(chan *proto.ProtocolMessage),
		state: newStateEmitter(StateConn, StateConnInitialized, ""),
	}
	c.queue = newMsgQueue(c)
	if opts.Listener != nil {
		c.On(opts.Listener)
	}
	if !opts.NoConnect {
		if _, err := c.connect(false); err != nil {
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
// If client is already connected, this method is a nop.
// If connecting fail due to authorization error, the returned error value
// is non-nil.
// If authorization succeeds, the returned Result value can be used to wait
// until connection confirmation is received from a server.
func (c *Conn) Connect() (Result, error) {
	return c.connect(true)
}

var connectResultStates = []StateEnum{
	StateConnConnected, // expected state
	StateConnFailed,
	StateConnDisconnected,
}

func (c *Conn) connect(result bool) (Result, error) {
	c.state.Lock()
	defer c.state.Unlock()
	if c.isActive() {
		return nopResult, nil
	}
	c.state.set(StateConnConnecting, nil)
	u, err := url.Parse(c.opts.realtimeURL())
	if err != nil {
		return nil, c.state.set(StateConnFailed, newError(50000, err))
	}
	rest, err := NewRestClient(c.opts)
	if err != nil {
		return nil, c.state.set(StateConnFailed, err)
	}
	token, err := rest.Auth.RequestToken(nil)
	if err != nil {
		return nil, c.state.set(StateConnFailed, err)
	}
	var res Result
	if result {
		res = c.state.listenResult(connectResultStates...)
	}
	proto := c.opts.protocol()
	query := url.Values{
		"access_token": {token.Token},
		"timestamp":    []string{strconv.FormatInt(TimestampNow(), 10)},
		"echo":         booltext(!c.opts.NoEcho),
	}
	if c.opts.UseBinaryProtocol || c.opts.protocol() == ProtocolMsgPack {
		query.Add("format", "msgpack")
	}
	u.RawQuery = query.Encode()
	conn, err := c.dial(proto, u)
	if err != nil {
		return nil, c.state.set(StateConnFailed, newError(ErrCodeConnectionFailed, err))
	}
	c.setConn(conn)
	return res, nil
}

var errClose = newError(50002, errors.New("Close() on inactive connection"))

// Close initiates closing sequence for the connection; it waits until the
// operation is complete.
//
// If connection is already closed, this method is a nop.
func (c *Conn) Close() error {
	err := wait(c.close())
	if c.conn != nil {
		c.conn.Close()
	}
	if err != nil {
		return c.state.syncSet(StateConnFailed, newErrorf(50002, "Close error: %s", err))
	}
	return nil
}

var closeResultStates = []StateEnum{
	StateConnClosed, // expected state
	StateConnFailed,
	StateConnDisconnected,
}

func (c *Conn) close() (Result, error) {
	c.state.Lock()
	defer c.state.Unlock()
	switch c.state.current {
	case StateConnClosing, StateConnClosed:
		return nopResult, nil
	case StateConnFailed, StateConnDisconnected:
		return nil, errClose
	}
	res := c.state.listenResult(closeResultStates...)
	c.state.set(StateConnClosing, nil)
	msg := &proto.ProtocolMessage{Action: proto.ActionClose}
	c.updateSerial(msg, nil)
	return res, c.conn.Send(msg)
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
	defer c.state.Unlock()
	return c.state.err
}

// Serial gives serial number of a message received most recently. Last known
// serial number is used when recovering connection state.
func (c *Conn) Serial() int64 {
	c.state.Lock()
	defer c.state.Unlock()
	return c.serial
}

// State returns current state of the connection.
func (c *Conn) State() StateEnum {
	c.state.Lock()
	defer c.state.Unlock()
	return c.state.current
}

// On relays request connection states to the given channel; on state transition
// connection will not block sending to c - the caller must ensure the incoming
// values are read at proper pace or the c is sufficiently buffered.
//
// If no states are given, c is registered for all of them.
// If c is nil, the method panics.
// If c is alreadt registered, its state set is expanded.
func (c *Conn) On(ch chan<- State, states ...StateEnum) {
	c.state.on(ch, states...)
}

// Off removes c from listetning on the given connection state transitions.
//
// If no states are given, c is removed for all of the connection's states.
// If c is nil, the method panics.
// If c was not registered or is already removed, the method is a nop.
func (c *Conn) Off(ch chan<- State, states ...StateEnum) {
	c.state.off(ch, states...)
}

var errQueueing = &Error{Code: 40000, Err: errors.New("unable to send messages in current state with disabled queueing")}

func (c *Conn) updateSerial(msg *proto.ProtocolMessage, listen chan<- error) {
	const maxint64 = 1<<63 - 1
	msg.MsgSerial = c.msgSerial
	c.msgSerial = (c.msgSerial + 1) % maxint64
	if listen != nil {
		c.pending.Enqueue(msg.MsgSerial, listen)
	}
}

func (c *Conn) send(msg *proto.ProtocolMessage, listen chan<- error) error {
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
	c.updateSerial(msg, listen)
	c.state.Unlock()
	return c.conn.Send(msg)
}

func (c *Conn) isActive() bool {
	return c.state.current == StateConnConnecting || c.state.current == StateConnConnected
}

func (c *Conn) lockIsActive() bool {
	c.state.Lock()
	defer c.state.Unlock()
	return c.isActive()
}

func (c *Conn) setConn(conn MsgConn) {
	c.conn = conn
	go c.eventloop()
}

func (c *Conn) eventloop() {
	for {
		msg := &proto.ProtocolMessage{}
		err := c.conn.Receive(&msg)
		if err != nil {
			c.state.Lock()
			if c.state.current == StateConnClosed {
				c.state.Unlock()
				return
			}
			c.state.set(StateConnFailed, newError(80000, err))
			c.state.Unlock()
			return // TODO recovery
		}
		if msg.ConnectionSerial != 0 {
			c.state.Lock()
			c.serial = msg.ConnectionSerial
			c.state.Unlock()
		}
		switch msg.Action {
		case proto.ActionHeartbeat:
		case proto.ActionAck:
			c.state.Lock()
			c.pending.Ack(msg.MsgSerial, msg.Count, newErrorProto(msg.Error))
			c.serial++
			c.state.Unlock()
		case proto.ActionNack:
			c.state.Lock()
			c.pending.Nack(msg.MsgSerial, msg.Count, newErrorProto(msg.Error))
			c.state.Unlock()
		case proto.ActionError:
			if msg.Channel != "" {
				c.msgCh <- msg
				break
			}
			c.state.Lock()
			c.state.set(StateConnFailed, newErrorProto(msg.Error))
			c.state.Unlock()
			c.queue.Fail(newErrorProto(msg.Error))
		case proto.ActionConnected:
			c.state.Lock()
			c.id = msg.ConnectionId
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
		case proto.ActionClosed:
			c.state.Lock()
			c.id = ""
			c.state.set(StateConnClosed, nil)
			c.state.Unlock()
		default:
			c.msgCh <- msg
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
