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
	errQueueing = errors.New("unable to send messages in current state with disabled queueing")
)

// Conn represents a single connection RealtimeClient instantiates for
// communication with Ably servers.
type Conn struct {
	details      proto.ConnectionDetails
	id           string
	serial       int64
	msgSerial    int64
	err          error
	conn         proto.Conn
	opts         *ClientOptions
	state        *stateEmitter
	stateCh      chan State
	pending      pendingEmitter
	queue        *msgQueue
	auth         *Auth
	callbacks    connCallbacks
	reconnecting bool
}

type connCallbacks struct {
	onChannelMsg func(*proto.ProtocolMessage)
	// onReconnectMsg is called when we get a response from reconnect request. We
	// move this up because some implementation details for (RTN15c) requires
	// access to Channels and we dont have it here so we let RealtimeClient do the
	// work.
	onReconnectMsg func(*proto.ProtocolMessage)
	onStateChange  func(State)
	// reconnecting tracks if we have issued a reconnection request. If we receive any message
	// with this set to true then its the first message/response after issuing the
	// reconnection request.
}

func newConn(opts *ClientOptions, auth *Auth, callbacks connCallbacks) (*Conn, error) {
	c := &Conn{
		opts:      opts,
		state:     newStateEmitter(StateConn, StateConnInitialized, "", auth.logger()),
		pending:   newPendingEmitter(auth.logger()),
		auth:      auth,
		callbacks: callbacks,
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
	return c.connectWithRecovery(result, "", 0)
}

func (c *Conn) reconnect(result bool) (Result, error) {
	c.state.Lock()
	connKey := c.details.ConnectionKey
	connSerial := c.serial
	c.state.Unlock()
	r, err := c.connectWithRecovery(result, connKey, connSerial)
	if err != nil {
		return nil, err
	}
	// We have successfully dialed reconnection request. We need to set this so
	// when the next message arrives it will be treated as the response to
	// reconnection request.
	c.state.Lock()
	c.reconnecting = true
	c.state.Unlock()
	return r, nil
}

func (c *Conn) connectWithRecovery(result bool, connKey string, connSerial int64) (Result, error) {
	c.state.Lock()
	defer c.state.Unlock()
	if c.isActive() {
		return nopResult, nil
	}
	c.setState(StateConnConnecting, nil)
	u, err := url.Parse(c.opts.realtimeURL())
	if err != nil {
		return nil, c.setState(StateConnFailed, err)
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
		return nil, c.setState(StateConnFailed, err)
	}
	if connKey != "" {
		query.Set("resume", connKey)
		query.Set("connectionSerial", fmt.Sprint(connSerial))
	}
	u.RawQuery = query.Encode()
	conn, err := c.dial(proto, u)
	if err != nil {
		return nil, c.setState(StateConnFailed, err)
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
	case
		StateConnClosing,
		StateConnClosed,
		StateConnInitialized,
		StateConnFailed,
		StateConnDisconnected:
		return nopResult, nil
	}
	res := c.state.listenResult(closeResultStates...)
	c.setState(StateConnClosing, nil)
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

func (c *Conn) ServerID() string {
	c.state.Lock()
	defer c.state.Unlock()
	return c.details.ServerID
}

// Ping issues a ping request against configured endpoint and returns TTR times
// for ping request and pong response.
//
// Ping returns non-nil error without any attempt of communication with Ably
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
// If c is already registered, its state set is expanded.
func (c *Conn) On(ch chan<- State, states ...StateEnum) {
	c.state.on(ch, states...)
}

// Off removes c from listening on the given connection state transitions.
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

func (c *Conn) lockCanReceiveMessages() bool {
	c.state.Lock()
	defer c.state.Unlock()
	return c.state.current == StateConnConnecting || c.state.current == StateConnConnected || c.state.current == StateConnClosing
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
	var receiveTimeout time.Duration

	for c.lockCanReceiveMessages() {
		var deadline time.Time
		if receiveTimeout != 0 {
			deadline = time.Now().Add(receiveTimeout) // RTN23a
		}
		msg, err := c.conn.Receive(deadline)
		if err != nil {
			c.state.Lock()
			if c.state.current == StateConnClosed {
				c.state.Unlock()
				return
			}

			c.setState(StateConnDisconnected, err)
			c.state.Unlock()
			c.reconnect(false)
			return
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
				c.callbacks.onChannelMsg(msg)
				break
			}
			c.state.Lock()
			if c.reconnecting {
				c.reconnecting = false
				if tokenError(msg.Error) {
					// (RTN15c5)
					// TODO: (gernest) implement (RTN15h) This can be done as a separate task?
				} else {
					// (RTN15c4)
					c.callbacks.onReconnectMsg(msg)
				}
			}
			c.setState(StateConnFailed, newErrorProto(msg.Error))
			c.state.Unlock()
			c.queue.Fail(newErrorProto(msg.Error))
		case proto.ActionConnected:
			c.auth.updateClientID(msg.ConnectionDetails.ClientID)
			if msg.ConnectionDetails != nil {
				c.state.Lock()
				c.details = *msg.ConnectionDetails
				c.state.Unlock()

				// Spec RSA7b3, RSA7b4, RSA12a
				c.auth.updateClientID(c.details.ClientID)

				maxIdleInterval := time.Duration(msg.ConnectionDetails.MaxIdleInterval) * time.Millisecond
				receiveTimeout = c.opts.realtimeRequestTimeout() + maxIdleInterval // RTN23a
			}
			c.state.Lock()
			reconnecting := c.reconnecting
			if reconnecting {
				c.reconnecting = false
			}
			c.state.Unlock()
			if reconnecting {
				// (RTN15c1) (RTN15c2)
				c.state.Lock()
				c.setState(StateConnConnected, msg.Error)
				id := c.id
				c.state.Unlock()
				if id != msg.ConnectionID {
					// (RTN15c3)
					// we are calling this outside of locks to avoid deadlock because in the
					// RealtimeClient client where this callback is implemented we do some ops
					// with this Conn where we re acquire Conn.state.Lock again.
					c.callbacks.onReconnectMsg(msg)
				}
			} else {
				// preserve old behavior.
				c.state.Lock()
				c.setState(StateConnConnected, nil)
				c.state.Unlock()
			}
			c.state.Lock()
			c.id = msg.ConnectionID
			c.serial = -1
			c.msgSerial = 0
			c.state.Unlock()
			c.queue.Flush()
		case proto.ActionDisconnected:
			c.state.Lock()
			c.id = ""
			c.setState(StateConnDisconnected, nil)
			c.state.Unlock()
		case proto.ActionClosed:
			c.state.Lock()
			c.id = ""
			c.setState(StateConnClosed, nil)
			c.state.Unlock()
		default:
			c.callbacks.onChannelMsg(msg)
		}
	}
}

func (c *Conn) setState(state StateEnum, err error) error {
	// TODO: Tempporary hack to fix https://github.com/ably/ably-go/issues/68.
	//
	// The proper way of propagating state changes is through the new
	// EventEmitter at https://github.com/ably/ably-go/pull/144.
	ch := make(chan State, 1)
	c.state.once(ch)
	go func() { c.callbacks.onStateChange(<-ch) }()

	return c.state.set(state, err)
}

type verboseConn struct {
	conn   proto.Conn
	logger *LoggerOptions
}

func (vc verboseConn) Send(msg *proto.ProtocolMessage) error {
	vc.logger.Printf(LogVerbose, "Realtime Connection: sending %s", msg)
	return vc.conn.Send(msg)
}

func (vc verboseConn) Receive(deadline time.Time) (*proto.ProtocolMessage, error) {
	msg, err := vc.conn.Receive(deadline)
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
