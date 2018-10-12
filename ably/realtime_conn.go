package ably

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ably/proto"
)

var (
	errQueueing      = errors.New("unable to send messages in current state with disabled queueing")
	errCloseInactive = errors.New("attempted to close inactive connection")
)

// Conn represents a single connection RealtimeClient instantiates for
// communication with Ably servers.
type Conn struct {
	details   proto.ConnectionDetails
	id        string
	serial    int64
	msgSerial int64
	err       error
	conn      proto.Conn
	msgCh     chan *proto.ProtocolMessage
	opts      *ClientOptions
	state     *stateEmitter
	stateCh   chan State
	pending   pendingEmitter
	queue     *msgQueue
	auth      *Auth
}

func newConn(opts *ClientOptions, auth *Auth) (*Conn, error) {
	c := &Conn{
		opts:    opts,
		msgCh:   make(chan *proto.ProtocolMessage),
		state:   newStateEmitter(StateConn, StateConnInitialized, "", auth.logger()),
		pending: newPendingEmitter(auth.logger()),
		auth:    auth,
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

func (c *Conn) dial(proto string, u *url.URL) (proto.Conn, error) {
	if c.opts.Dial != nil {
		return c.opts.Dial(proto, u)
	}
	return ablyutil.DialWebsocket(proto, u)
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
		return nil, c.state.set(StateConnFailed, err)
	}
	var res Result
	if result {
		res = c.state.listenResult(connectResultStates...)
	}
	proto := c.opts.protocol()
	query := url.Values{
		"timestamp": []string{strconv.FormatInt(TimeNow(), 10)},
		"echo":      []string{"true"},
		"format":    []string{"msgpack"},
	}
	if c.opts.NoEcho {
		query.Set("echo", "false")
	}
	if c.opts.NoBinaryProtocol {
		query.Set("format", "json")
	}
	if c.opts.ClientID != "" && c.auth.method == authBasic {
		// References RSA7e1
		query.Set("clientId", c.opts.ClientID)
	}
	for k, v := range c.opts.TransportParams {
		query.Set(k, v)
	}
	if err := c.auth.authQuery(query); err != nil {
		return nil, c.state.set(StateConnFailed, err)
	}
	u.RawQuery = query.Encode()
	conn, err := c.dial(proto, u)
	if err != nil {
		return nil, c.state.set(StateConnFailed, err)
	}
	if c.logger().Is(LogVerbose) {
		c.setConn(verboseConn{conn: conn, logger: c.logger()})
	} else {
		c.setConn(conn)
	}
	return res, nil
}

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
		return c.state.syncSet(StateConnFailed, err)
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
	case StateConnInitialized, StateConnFailed, StateConnDisconnected:
		return nil, stateError(c.state.current, errCloseInactive)
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
	return c.details.ConnectionKey
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
	switch state := c.state.current; state {
	case StateConnInitialized, StateConnConnecting, StateConnDisconnected:
		c.state.Unlock()
		if c.opts.NoQueueing {
			return stateError(state, errQueueing)
		}
		c.queue.Enqueue(msg, listen)
		return nil
	case StateConnConnected:
	default:
		c.state.Unlock()
		return stateError(state, nil)
	}
	if err := c.verifyAndUpdateMessages(msg); err != nil {
		c.state.Unlock()
		return err
	}
	c.updateSerial(msg, listen)
	c.state.Unlock()
	return c.conn.Send(msg)
}

// verifyAndUpdateMessages ensures the ClientID sent with published messages or
// presence messages matches the authenticated user's ClientID and if it does,
// ensures it's empty as Able service is responsible for populating it.
//
// If both user was not authenticated with a wildcard ClientID and the one
// being sent does not match it, the method return non-nil error.
func (c *Conn) verifyAndUpdateMessages(msg *proto.ProtocolMessage) (err error) {
	clientID := c.auth.clientIDForCheck()
	connectionID := c.id
	switch msg.Action {
	case proto.ActionMessage:
		for _, msg := range msg.Messages {
			if !isClientIDAllowed(clientID, msg.ClientID) {
				return newError(90000, fmt.Errorf("unable to send message as %q", msg.ClientID))
			}
			if clientID == msg.ClientID {
				msg.ClientID = ""
			}
			msg.ConnectionID = connectionID
		}
	case proto.ActionPresence:
		for _, presmsg := range msg.Presence {
			switch {
			case !isClientIDAllowed(clientID, presmsg.ClientID):
				return newError(90000, fmt.Errorf("unable to send presence message as %q", presmsg.ClientID))
			case clientID == "" && presmsg.ClientID == "":
				return newError(90000, errors.New("unable to infer ClientID from the connection"))
			case presmsg.ClientID == "":
				presmsg.ClientID = clientID
			}
			presmsg.ConnectionID = connectionID
		}
	}
	return nil
}

func (c *Conn) isActive() bool {
	return c.state.current == StateConnConnecting || c.state.current == StateConnConnected
}

func (c *Conn) lockIsActive() bool {
	c.state.Lock()
	defer c.state.Unlock()
	return c.isActive()
}

func (c *Conn) setConn(conn proto.Conn) {
	c.conn = conn
	go c.eventloop()
}

func (c *Conn) logger() *LoggerOptions {
	return c.auth.logger()
}

func (c *Conn) eventloop() {
	for {
		msg, err := c.conn.Receive()
		if err != nil {
			c.state.Lock()
			if c.state.current == StateConnClosed {
				c.state.Unlock()
				return
			}
			c.state.set(StateConnFailed, err)
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
			c.auth.updateClientID(msg.ConnectionDetails.ClientID)
			c.state.Lock()
			c.id = msg.ConnectionID
			if msg.ConnectionDetails != nil {
				c.details = *msg.ConnectionDetails
			}
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

type verboseConn struct {
	conn   proto.Conn
	logger *LoggerOptions
}

func (vc verboseConn) Send(msg *proto.ProtocolMessage) error {
	vc.logger.Printf(LogVerbose, "Realtime Connection: sending %s", msg)
	return vc.conn.Send(msg)
}

func (vc verboseConn) Receive() (*proto.ProtocolMessage, error) {
	msg, err := vc.conn.Receive()
	if err != nil {
		return nil, err
	}
	vc.logger.Printf(LogVerbose, "Realtime Connection: received %s", msg)
	return msg, nil
}

func (vc verboseConn) Close() error {
	vc.logger.Printf(LogVerbose, "Realtime Connection: closed")
	return vc.conn.Close()
}
