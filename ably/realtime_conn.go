package ably

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ably/proto"
)

var (
	errQueueing = errors.New("unable to send messages in current state with disabled queueing")
)

// connectionMode is the mode in which the connection is operating
type connectionMode uint

const (
	// normalMode this is set when the Connection operating normally
	normalMode connectionMode = iota
	// resumeMode this is set when the Connection is trying to resume
	resumeMode
	// recoveryMode this is set when the Connection is trying to recover
	recoveryMode
)

// Connection represents a single connection Realtime instantiates for
// communication with Ably servers.
type Connection struct {
	id        string
	key       string
	serial    int64
	msgSerial int64
	err       error
	conn      proto.Conn
	opts      *clientOptions
	state     *stateEmitter
	pending   pendingEmitter
	queue     *msgQueue
	auth      *Auth

	callbacks connCallbacks
	// reconnecting tracks if we have issued a reconnection request. If we receive any message
	// with this set to true then its the first message/response after issuing the
	// reconnection request.
	reconnecting bool
}

type connCallbacks struct {
	onChannelMsg func(*proto.ProtocolMessage)
	// onReconnected is called when we get a CONNECTED response from reconnect request. We
	// move this up because some implementation details for (RTN15c) requires
	// access to Channels and we dont have it here so we let RealtimeClient do the
	// work.
	onReconnected func(*proto.ErrorInfo)
	// onReconnectionFailed is called when we get a FAILED response from a
	// reconnection request.
	onReconnectionFailed func(*proto.ErrorInfo)
	onStateChange        func(State)
}

func newConn(opts *clientOptions, auth *Auth, callbacks connCallbacks) (*Connection, error) {
	c := &Connection{
		opts:      opts,
		state:     newStateEmitter(StateConn, StateConnInitialized, "", auth.logger()),
		pending:   newPendingEmitter(auth.logger()),
		auth:      auth,
		callbacks: callbacks,
	}
	c.queue = newMsgQueue(c)
	if opts.Listener != nil {
		c.onState(opts.Listener)
	}
	if !opts.NoConnect {
		if _, err := c.connect(false); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Connection) dial(proto string, u *url.URL) (proto.Conn, error) {
	// (RTN23b)
	query := u.Query()
	query.Add("heartbeats", "true")
	u.RawQuery = query.Encode()
	if c.opts.Dial != nil {
		return c.opts.Dial(proto, u)
	}
	return ablyutil.DialWebsocket(proto, u)
}

// Connect attempts to move the connection to the CONNECTED state, if it
// can and if it isn't already.
func (c *Connection) Connect() {
	c.connect(false)
}

// Close attempts to move the connection to the CLOSED state, if it can and if
// it isn't already.
func (c *Connection) Close() {
	c.close()
}

var connectResultStates = []StateEnum{
	StateConnConnected, // expected state
	StateConnFailed,
	StateConnDisconnected,
}

func (c *Connection) connect(result bool) (Result, error) {
	c.state.Lock()
	mode := c.getMode()
	c.state.Unlock()
	return c.connectWith(result, mode)
}

func (c *Connection) reconnect(result bool) (Result, error) {
	c.state.Lock()
	mode := c.getMode()
	c.state.Unlock()
	r, err := c.connectWith(result, mode)
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

func (c *Connection) getMode() connectionMode {
	if c.key != "" {
		return resumeMode
	}
	if c.opts.Recover != "" {
		return recoveryMode
	}
	return normalMode
}

func (c *Connection) params(mode connectionMode) (url.Values, error) {
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
		query[k] = v
	}
	if err := c.auth.authQuery(query); err != nil {
		return nil, err
	}
	switch mode {
	case resumeMode:
		query.Set("resume", c.key)
		query.Set("connectionSerial", fmt.Sprint(c.serial))
	case recoveryMode:
		m := strings.Split(c.opts.Recover, ":")
		if len(m) != 3 {
			return nil, errors.New("conn: Invalid recovery key")
		}
		query.Set("recover", m[0])
		query.Set("connectionSerial", m[1])
	}
	return query, nil
}

func (c *Connection) connectWith(result bool, mode connectionMode) (Result, error) {
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
	query, err := c.params(mode)
	if err != nil {
		return nil, c.setState(StateConnFailed, err)
	}
	u.RawQuery = query.Encode()
	proto := c.opts.protocol()
	conn, err := c.dial(proto, u)
	if err != nil {
		return nil, c.setState(StateConnFailed, err)
	}
	if c.logger().Is(LogVerbose) {
		c.setConn(verboseConn{conn: conn, logger: c.logger()})
	} else {
		c.setConn(conn)
	}
	c.reconnecting = mode == recoveryMode || mode == resumeMode
	return res, nil
}

var closeResultStates = []StateEnum{
	StateConnClosed, // expected state
	StateConnFailed,
	StateConnDisconnected,
}

func (c *Connection) close() {
	c.state.Lock()
	c.key, c.id = "", "" //(RTN16c)
	defer c.state.Unlock()
	switch c.state.current {
	case
		StateConnClosing,
		StateConnClosed,
		StateConnInitialized,
		StateConnFailed,
		StateConnDisconnected:

		return
	}

	c.state.set(StateConnClosing, nil)
	msg := &proto.ProtocolMessage{Action: proto.ActionClose}
	c.updateSerial(msg, nil)

	// TODO: handle error. If you can't send a message, the fail-fast way to
	// deal with it is to discard the WebSocket and perform a normal
	// reconnection. We could also have a retry loop, but in any case, it should
	// be dealt with centrally, so Send shouldn't return the error but handle
	// it in some way. The caller isn't responsible for recovering from realtime
	// connection transient errors.
	_ = c.conn.Send(msg)
}

// ID gives unique ID string obtained from Ably upon successful connection.
// The ID may change due to reconnection and recovery; on every received
// StateConnConnected event previously obtained ID is no longer valid.
func (c *Connection) ID() string {
	c.state.Lock()
	defer c.state.Unlock()
	return c.id
}

// Key gives unique key string obtained from Ably upon successful connection.
// The key may change due to reconnection and recovery; on every received
// StatConnConnected event previously obtained Key is no longer valid.
func (c *Connection) Key() string {
	c.state.Lock()
	defer c.state.Unlock()
	return c.key
}

// Ping issues a ping request against configured endpoint and returns TTR times
// for ping request and pong response.
//
// Ping returns non-nil error without any attempt of communication with Ably
// if the connection state is StateConnClosed or StateConnFailed.
func (c *Connection) Ping() (ping, pong time.Duration, err error) {
	return 0, 0, errors.New("TODO")
}

// ErrorReason gives last known error that caused connection transit to
// StateConnFailed state.
func (c *Connection) ErrorReason() *ErrorInfo {
	c.state.Lock()
	defer c.state.Unlock()
	return c.state.err
}

func (c *Connection) RecoveryKey() string {
	c.state.Lock()
	defer c.state.Unlock()
	if c.key == "" {
		return ""
	}
	return strings.Join([]string{c.key, fmt.Sprint(c.serial), fmt.Sprint(c.msgSerial)}, ":")
}

// Serial gives serial number of a message received most recently. Last known
// serial number is used when recovering connection state.
func (c *Connection) Serial() int64 {
	c.state.Lock()
	defer c.state.Unlock()
	return c.serial
}

// State returns current state of the connection.
func (c *Connection) State() StateEnum {
	c.state.Lock()
	defer c.state.Unlock()
	return c.state.current
}

// onState relays request connection states to the given channel; onState state transition
// connection will not block sending to c - the caller must ensure the incoming
// values are read at proper pace or the c is sufficiently buffered.
//
// If no states are given, c is registered for all of them.
// If c is nil, the method panics.
// If c is already registered, its state set is expanded.
func (c *Connection) onState(ch chan<- State, states ...StateEnum) {
	c.state.on(ch, states...)
}

// On registers an event handler for connection events of a specific kind.
//
// See package-level documentation on Event Emitter for details.
func (c *Connection) On(e ConnectionEvent, handle func(ConnectionStateChange)) (off func()) {
	return c.state.eventEmitter.On(e, func(change emitterData) {
		handle(change.(ConnectionStateChange))
	})
}

// OnAll registers an event handler for all connection events.
//
// See package-level documentation on Event Emitter for details.
func (c *Connection) OnAll(handle func(ConnectionStateChange)) (off func()) {
	return c.state.eventEmitter.OnAll(func(change emitterData) {
		handle(change.(ConnectionStateChange))
	})
}

// Once registers an one-off event handler for connection events of a specific kind.
//
// See package-level documentation on Event Emitter for details.
func (c *Connection) Once(e ConnectionEvent, handle func(ConnectionStateChange)) (off func()) {
	return c.state.eventEmitter.On(e, func(change emitterData) {
		handle(change.(ConnectionStateChange))
	})
}

// OnceAll registers an one-off event handler for all connection events.
//
// See package-level documentation on Event Emitter for details.
func (c *Connection) OnceAll(handle func(ConnectionStateChange)) (off func()) {
	return c.state.eventEmitter.OnceAll(func(change emitterData) {
		handle(change.(ConnectionStateChange))
	})
}

// offState removes c from listening on the given connection state transitions.
//
// If no states are given, c is removed for all of the connection's states.
// If c is nil, the method panics.
// If c was not registered or is already removed, the method is a nop.
func (c *Connection) offState(ch chan<- State, states ...StateEnum) {
	c.state.off(ch, states...)
}

// Off deregisters event handlers for connection events of a specific kind.
//
// See package-level documentation on Event Emitter for details.
func (c *Connection) Off(e ConnectionEvent) {
	c.state.eventEmitter.Off(e)
}

// Off deregisters all event handlers.
//
// See package-level documentation on Event Emitter for details.
func (c *Connection) OffAll() {
	c.state.eventEmitter.OffAll()
}

func (c *Connection) updateSerial(msg *proto.ProtocolMessage, listen chan<- error) {
	const maxint64 = 1<<63 - 1
	msg.MsgSerial = c.msgSerial
	c.msgSerial = (c.msgSerial + 1) % maxint64
	if listen != nil {
		c.pending.Enqueue(msg.MsgSerial, listen)
	}
}

func (c *Connection) send(msg *proto.ProtocolMessage, listen chan<- error) error {
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
func (c *Connection) verifyAndUpdateMessages(msg *proto.ProtocolMessage) (err error) {
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

func (c *Connection) isActive() bool {
	return c.state.current == StateConnConnecting || c.state.current == StateConnConnected
}

func (c *Connection) lockCanReceiveMessages() bool {
	c.state.Lock()
	defer c.state.Unlock()
	return c.state.current == StateConnConnecting || c.state.current == StateConnConnected || c.state.current == StateConnClosing
}

func (c *Connection) lockIsActive() bool {
	c.state.Lock()
	defer c.state.Unlock()
	return c.isActive()
}

func (c *Connection) setConn(conn proto.Conn) {
	c.conn = conn
	go c.eventloop()
}

func (c *Connection) logger() *LoggerOptions {
	return c.auth.logger()
}

func (c *Connection) setSerial(serial int64) {
	c.serial = serial
}

func (c *Connection) eventloop() {
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
			c.setSerial(msg.ConnectionSerial)
			c.state.Unlock()
		}
		switch msg.Action {
		case proto.ActionHeartbeat:
		case proto.ActionAck:
			c.state.Lock()
			c.pending.Ack(msg.MsgSerial, msg.Count, newErrorProto(msg.Error))
			c.setSerial(c.serial + 1)
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
			c.failedConnSideEffects(msg.Error)
		case proto.ActionConnected:
			c.auth.updateClientID(msg.ConnectionDetails.ClientID)
			c.state.Lock()
			// we need to get this before we set c.key so as to be sure if we were
			// resuming or recovering the connection.
			mode := c.getMode()
			c.state.Unlock()
			if msg.ConnectionDetails != nil {
				c.state.Lock()
				c.key = msg.ConnectionDetails.ConnectionKey //(RTN15e) (RTN16d)
				c.state.Unlock()

				// Spec RSA7b3, RSA7b4, RSA12a
				c.auth.updateClientID(msg.ConnectionDetails.ClientID)

				maxIdleInterval := time.Duration(msg.ConnectionDetails.MaxIdleInterval) * time.Millisecond
				receiveTimeout = c.opts.realtimeRequestTimeout() + maxIdleInterval // RTN23a
			}
			c.state.Lock()
			reconnecting := c.reconnecting
			if reconnecting {
				// reset the mode
				c.reconnecting = false
			}
			id := c.id
			c.state.Unlock()
			if reconnecting {
				// (RTN15c1) (RTN15c2)
				c.state.Lock()
				c.setState(StateConnConnected, newErrorProto(msg.Error))
				c.state.Unlock()
				if id != msg.ConnectionID {
					// (RTN15c3)
					// we are calling this outside of locks to avoid deadlock because in the
					// RealtimeClient client where this callback is implemented we do some ops
					// with this Conn where we re acquire Conn.state.Lock again.
					c.callbacks.onReconnected(msg.Error)
				}
			} else {
				// preserve old behavior.
				c.state.Lock()
				c.setState(StateConnConnected, nil)
				c.state.Unlock()
			}
			c.state.Lock()
			c.id = msg.ConnectionID
			c.msgSerial = 0
			if reconnecting && mode == recoveryMode {
				// we are setting msgSerial as per (RTN16f)
				msgSerial, err := strconv.ParseInt(strings.Split(c.opts.Recover, ":")[2], 10, 64)
				if err != nil {
					//TODO: how to handle this? Panic?
				}
				c.msgSerial = msgSerial
			}
			c.setSerial(-1)
			c.state.Unlock()
			c.queue.Flush()
		case proto.ActionDisconnected:
			if !isTokenError(msg.Error) {
				// The spec doesn't say what to do in this case, so do nothing.
				// Ably is supposed to then close the transport, which will
				// trigger a transition to DISCONNECTED.
				continue
			}

			// TODO: RTN15h
		case proto.ActionClosed:
			c.state.Lock()
			c.id, c.key = "", "" //(RTN16c)
			c.setState(StateConnClosed, nil)
			c.state.Unlock()
			if c.conn != nil {
				c.conn.Close()
			}
		default:
			c.callbacks.onChannelMsg(msg)
		}
	}
}

func (c *Connection) setState(state StateEnum, err error) error {
	// TODO: Tempporary hack to fix https://github.com/ably/ably-go/issues/68.
	//
	// The proper way of propagating state changes is through the new
	// EventEmitter at https://github.com/ably/ably-go/pull/144.
	ch := make(chan State, 1)
	c.state.once(ch)
	go func() { c.callbacks.onStateChange(<-ch) }()

	return c.state.set(state, err)
}

func (c *Connection) failedConnSideEffects(err *proto.ErrorInfo) {
	c.state.Lock()
	if c.reconnecting {
		c.reconnecting = false
		c.callbacks.onReconnectionFailed(err)
	}
	c.setState(StateConnFailed, newErrorProto(err))
	c.state.Unlock()
	c.queue.Fail(newErrorProto(err))
	if c.conn != nil {
		c.conn.Close()
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
