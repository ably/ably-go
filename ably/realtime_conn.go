package ably

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
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
	mtx sync.Mutex

	// on setConn we write to conn with mtx protection, however in eventLoop we
	// read conn unprotected, this is racy because now we establish connection in a
	// separate goroutine.
	//
	// using mtx to protect reads in eventLoop causes a deadlock.
	connMtx sync.Mutex
	ConnectionEventEmitter

	state           ConnectionState
	errorReason     *ErrorInfo
	internalEmitter ConnectionEventEmitter

	id        string
	key       string
	serial    int64
	msgSerial int64
	err       error
	conn      proto.Conn
	opts      *clientOptions
	pending   pendingEmitter
	queue     *msgQueue
	auth      *Auth

	callbacks connCallbacks
	// reconnecting tracks if we have issued a reconnection request. If we receive any message
	// with this set to true then its the first message/response after issuing the
	// reconnection request.
	reconnecting bool
	// reauthorizing tracks if the current reconnection attempt is happening
	// after a reauthorization, to avoid re-reauthorizing.
	reauthorizing bool
	arg           connArgs
}

type connCallbacks struct {
	onChannelMsg func(*proto.ProtocolMessage)
	// onReconnected is called when we get a CONNECTED response from reconnect request. We
	// move this up because some implementation details for (RTN15c) requires
	// access to Channels and we dont have it here so we let RealtimeClient do the
	// work.
	onReconnected func(isNewID bool)
	// onReconnectionFailed is called when we get a FAILED response from a
	// reconnection request.
	onReconnectionFailed func(*proto.ErrorInfo)
}

func newConn(opts *clientOptions, auth *Auth, callbacks connCallbacks) *Connection {
	c := &Connection{

		ConnectionEventEmitter: ConnectionEventEmitter{newEventEmitter(auth.logger())},
		state:                  ConnectionStateInitialized,
		internalEmitter:        ConnectionEventEmitter{newEventEmitter(auth.logger())},

		opts:      opts,
		pending:   newPendingEmitter(auth.logger()),
		auth:      auth,
		callbacks: callbacks,
	}
	c.queue = newMsgQueue(c)
	if !opts.NoConnect {
		c.setState(ConnectionStateConnecting, nil, 0)
		go func() {
			lg := opts.Logger.sugar()
			lg.Info("Trying to establish a connection asynchronously")
			if _, err := c.connect(connArgs{}); err != nil {
				lg.Errorf("Failed to open connection with err:%v", err)
			}
		}()
	}
	return c
}

func (c *Connection) dial(proto string, u *url.URL) (conn proto.Conn, err error) {
	lg := c.logger().sugar()
	start := time.Now()
	lg.Debugf("Dial protocol=%q url %q ", proto, u.String())
	// (RTN23b)
	query := u.Query()
	query.Add("heartbeats", "true")
	u.RawQuery = query.Encode()
	timeout := c.opts.realtimeRequestTimeout()
	// TODO: Use c.opts.baseCtx to dial.
	if c.opts.Dial != nil {
		conn, err = c.opts.Dial(proto, u, timeout)
	} else {
		conn, err = ablyutil.DialWebsocket(proto, u, timeout)
	}
	if err != nil {
		lg.Debugf("Dial Failed in %v with %v", time.Since(start), err)
		return nil, err
	}
	lg.Debugf("Dial success in %v", time.Since(start))
	return conn, err
}

func (c *Connection) connectAfterSuspension(arg connArgs) (result, error) {
	retryIn := c.opts.suspendedRetryTimeout()
	lg := c.logger().sugar()
	lg.Debugf("Attemting to periodically establish connection after suspension with timeout %v", retryIn)
	ctx, cancel := c.ctxCancelOnStateTransition()
	defer cancel()
	tick := ablyutil.NewTicker(c.opts.After)(ctx, retryIn)
	for {
		_, ok := <-tick
		if !ok {
			return nil, ctx.Err()
		}
		res, err := c.connectWith(arg)
		if err != nil {
			if recoverable(err) {
				c.setState(ConnectionStateSuspended, err, retryIn)
				continue
			}
			return nil, c.setState(ConnectionStateFailed, err, 0)
		}
		return res, nil
	}
}

// recoverable returns true if err is recoverable, err is from making a
// connection
func recoverable(err error) bool {
	var e *ErrorInfo
	if errors.As(err, &e) {
		return !(40000 <= e.Code && e.Code < 50000)
	}
	return true
}

// Connect attempts to move the connection to the CONNECTED state, if it
// can and if it isn't already.
func (c *Connection) Connect() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	isActive := c.isActive()
	if isActive {
		return
	}

	c.lockSetState(ConnectionStateConnecting, nil, 0)

	go func() {
		c.connect(connArgs{})
	}()
}

// Close attempts to move the connection to the CLOSED state, if it can and if
// it isn't already.
func (c *Connection) Close() {
	c.close()
}

func (c *Connection) connect(arg connArgs) (result, error) {
	c.mtx.Lock()
	arg.mode = c.getMode()
	c.mtx.Unlock()
	return c.connectWithRetryLoop(arg)
}

type connArgs struct {
	lastActivityAt time.Time
	connDetails    *proto.ConnectionDetails
	result         bool
	dialOnce       bool
	mode           connectionMode
	retryIn        time.Duration
}

func (c *Connection) reconnect(arg connArgs) (result, error) {
	c.mtx.Lock()

	var mode connectionMode
	if arg.connDetails != nil && c.opts.Now().Sub(arg.lastActivityAt) >= time.Duration(arg.connDetails.ConnectionStateTTL+arg.connDetails.MaxIdleInterval) {
		// RTN15g
		c.msgSerial = 0
		c.key = ""
		// c.id isn't cleared since it's used later to determine if the
		// reconnection resulted in a new transport-level connection.
		mode = normalMode
	} else {
		mode = c.getMode()
	}

	c.mtx.Unlock()
	arg.mode = mode
	r, err := c.connectWithRetryLoop(arg)
	if err != nil {
		return nil, err
	}
	// We have successfully dialed reconnection request. We need to set this so
	// when the next message arrives it will be treated as the response to
	// reconnection request.
	c.mtx.Lock()
	c.reconnecting = true
	c.mtx.Unlock()

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
	c.mtx.Lock()
	defer c.mtx.Unlock()

	query := url.Values{
		"timestamp": []string{strconv.FormatInt(unixMilli(c.opts.Now()), 10)},
		"echo":      []string{"true"},
		"format":    []string{"msgpack"},
		"lib":       []string{proto.LibraryString},
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
	if err := c.auth.authQuery(c.opts.BaseCtx, query); err != nil {
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

const connectionStateTTLErrFmt = "Exceeded connectionStateTtl=%v while in DISCONNECTED state"

func (c *Connection) connectWithRetryLoop(arg connArgs) (result, error) {
	lg := c.logger().sugar()
	res, err := c.connectWith(arg)
	if err == nil {
		return res, nil
	}
	if arg.dialOnce || !recoverable(err) {
		return nil, c.setState(ConnectionStateFailed, err, 0)
	}

	lg.Errorf("Received recoverable error %v", err)
	retryIn := c.opts.disconnectedRetryTimeout()
	c.setState(ConnectionStateDisconnected, err, retryIn)

	ctx, cancel := c.ctxCancelOnStateTransition()
	defer cancel()

	// The initial DISCONNECTED event has been fired. If we reach stateTTL without
	// any state changes, we transition to SUSPENDED state
	stateTTL := c.opts.connectionStateTTL()
	stateDeadlineCtx, cancelStateDeadline := context.WithCancel(ctx)
	defer cancelStateDeadline()
	stateDeadline := c.opts.After(stateDeadlineCtx, stateTTL)

	nextCtx, cancelNext := context.WithCancel(ctx)
	next := c.opts.After(nextCtx, retryIn)
	reset := func() {
		lg.Debugf("Retry in %v", retryIn)
		cancelNext()
		nextCtx, cancelNext = context.WithCancel(ctx)
		next = c.opts.After(nextCtx, retryIn)
	}

loop:
	for {
		select {
		case _, ok := <-stateDeadline:
			if !ok {
				return nil, ctx.Err()
			}
			break loop
		case _, ok := <-next:
			if !ok {
				return nil, ctx.Err()
			}
			// Prioritize stateDeadline.
			select {
			case _, ok := <-stateDeadline:
				if !ok {
					return nil, ctx.Err()
				}
			default:
			}

			lg.Debug("Attemting to open connection")
			res, err := c.connectWith(arg)
			if err == nil {
				return res, nil
			}
			if recoverable(err) {
				lg.Errorf("Received recoverable error %v", err)
				// No need to reset stateDeadline as we are still in DISCONNECTED state. so
				// another DISCONNECTED implies we haven't transitioned from DISCONNECTED
				// state
				c.setState(ConnectionStateDisconnected, err, retryIn)
				reset()
				continue
			}
			return nil, c.setState(ConnectionStateFailed, err, 0)
		}
	}

	// (RTN14e)
	lg.Debug("Transition to SUSPENDED state")
	err = fmt.Errorf(connectionStateTTLErrFmt, stateTTL)
	c.setState(ConnectionStateSuspended, err, c.opts.suspendedRetryTimeout())
	// (RTN14f)
	lg.Debug("Reached SUSPENDED state while opening connection")
	return c.connectAfterSuspension(arg)
}

func (c *Connection) connectWith(arg connArgs) (result, error) {
	c.mtx.Lock()
	if !c.isActive() {
		c.lockSetState(ConnectionStateConnecting, nil, 0)
	}
	c.mtx.Unlock()

	u, err := url.Parse(c.opts.realtimeURL())
	if err != nil {
		return nil, err
	}
	var res result
	if arg.result {
		res = c.internalEmitter.listenResult(
			ConnectionStateConnected, // expected state
			ConnectionStateFailed,
			ConnectionStateDisconnected,
		)
	}
	query, err := c.params(arg.mode)
	if err != nil {
		return nil, err
	}
	u.RawQuery = query.Encode()
	proto := c.opts.protocol()
	conn, err := c.dial(proto, u)
	if err != nil {
		return nil, err
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.logger().Is(LogVerbose) {
		c.setConn(verboseConn{conn: conn, logger: c.logger()})
	} else {
		c.setConn(conn)
	}
	c.reconnecting = arg.mode == recoveryMode || arg.mode == resumeMode
	c.arg = arg
	return res, nil
}

func (c *Connection) connectionStateTTL() time.Duration {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.arg.connDetails != nil && c.arg.connDetails.ConnectionStateTTL != 0 {
		return time.Duration(c.arg.connDetails.ConnectionStateTTL)
	}
	return c.opts.connectionStateTTL()
}

func (c *Connection) close() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	switch c.state {
	case ConnectionStateClosing, ConnectionStateClosed, ConnectionStateFailed:
	case ConnectionStateConnected: // RTN12a
		c.lockSetState(ConnectionStateClosing, nil, 0)
		c.sendClose()
	case ConnectionStateConnecting: // RTN12f
		c.lockSetState(ConnectionStateClosing, nil, 0)
	default: // RTN12d
		c.lockSetState(ConnectionStateClosed, nil, 0)
	}
}

func (c *Connection) sendClose() {
	msg := &proto.ProtocolMessage{Action: proto.ActionClose}

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
// ConnectionStateConnected event previously obtained ID is no longer valid.
func (c *Connection) ID() string {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.id
}

// Key gives unique key string obtained from Ably upon successful connection.
// The key may change due to reconnection and recovery; on every received
// StatConnConnected event previously obtained Key is no longer valid.
func (c *Connection) Key() string {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.key
}

// Ping issues a ping request against configured endpoint and returns TTR times
// for ping request and pong response.
//
// Ping returns non-nil error without any attempt of communication with Ably
// if the connection state is ConnectionStateClosed or ConnectionStateFailed.
func (c *Connection) Ping() (ping, pong time.Duration, err error) {
	return 0, 0, errors.New("TODO")
}

// ErrorReason gives last known error that caused connection transit to
// ConnectionStateFailed state.
func (c *Connection) ErrorReason() *ErrorInfo {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.errorReason
}

func (c *Connection) RecoveryKey() string {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.key == "" {
		return ""
	}
	return strings.Join([]string{c.key, fmt.Sprint(c.serial), fmt.Sprint(c.msgSerial)}, ":")
}

// Serial gives serial number of a message received most recently. Last known
// serial number is used when recovering connection state.
func (c *Connection) Serial() int64 {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.serial
}

// State returns current state of the connection.
func (c *Connection) State() ConnectionState {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.state
}

type connStateChanges chan ConnectionStateChange

func (c connStateChanges) Receive(change ConnectionStateChange) {
	c <- change
}

type ConnectionEventEmitter struct {
	emitter *eventEmitter
}

// On registers an event handler for connection events of a specific kind.
//
// See package-level documentation on Event Emitter for details.
func (em ConnectionEventEmitter) On(e ConnectionEvent, handle func(ConnectionStateChange)) (off func()) {
	return em.emitter.On(e, func(change emitterData) {
		handle(change.(ConnectionStateChange))
	})
}

// OnAll registers an event handler for all connection events.
//
// See package-level documentation on Event Emitter for details.
func (em ConnectionEventEmitter) OnAll(handle func(ConnectionStateChange)) (off func()) {
	return em.emitter.OnAll(func(change emitterData) {
		handle(change.(ConnectionStateChange))
	})
}

// Once registers an one-off event handler for connection events of a specific kind.
//
// See package-level documentation on Event Emitter for details.
func (em ConnectionEventEmitter) Once(e ConnectionEvent, handle func(ConnectionStateChange)) (off func()) {
	return em.emitter.Once(e, func(change emitterData) {
		handle(change.(ConnectionStateChange))
	})
}

// OnceAll registers an one-off event handler for all connection events.
//
// See package-level documentation on Event Emitter for details.
func (em ConnectionEventEmitter) OnceAll(handle func(ConnectionStateChange)) (off func()) {
	return em.emitter.OnceAll(func(change emitterData) {
		handle(change.(ConnectionStateChange))
	})
}

// Off deregisters event handlers for connection events of a specific kind.
//
// See package-level documentation on Event Emitter for details.
func (em ConnectionEventEmitter) Off(e ConnectionEvent) {
	em.emitter.Off(e)
}

// Off deregisters all event handlers.
//
// See package-level documentation on Event Emitter for details.
func (em ConnectionEventEmitter) OffAll() {
	em.emitter.OffAll()
}

func (c *Connection) advanceSerial() {
	const maxint64 = 1<<63 - 1
	c.msgSerial = (c.msgSerial + 1) % maxint64
}

func (c *Connection) send(msg *proto.ProtocolMessage, listen chan<- error) {
	c.mtx.Lock()
	switch state := c.state; state {
	default:
		c.mtx.Unlock()
		listen <- connStateError(state, nil)

	case ConnectionStateInitialized, ConnectionStateConnecting, ConnectionStateDisconnected:
		c.mtx.Unlock()
		if c.opts.NoQueueing {
			listen <- connStateError(state, errQueueing)
		}
		c.queue.Enqueue(msg, listen)

	case ConnectionStateConnected:
		if err := c.verifyAndUpdateMessages(msg); err != nil {
			c.mtx.Unlock()
			listen <- err
			return
		}
		msg.MsgSerial = c.msgSerial
		err := c.conn.Send(msg)
		if err != nil {
			// An error here means there has been some transport-level failure in the
			// connection. The connection itself is probably discarded, which causes the
			// concurrent Receive in eventloop to fail, which in turn starts the
			// reconnection logic. But in case it isn't, force that by closing the
			// connection. Otherwise, the message we enqueue here may be in the queue
			// indefinitely.
			c.conn.Close()
			c.mtx.Unlock()
			c.queue.Enqueue(msg, listen)
		} else {
			c.advanceSerial()
			if listen != nil {
				c.pending.Enqueue(msg, listen)
			}
			c.mtx.Unlock()
		}
	}
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
	return c.state == ConnectionStateConnecting || c.state == ConnectionStateConnected
}

func (c *Connection) lockCanReceiveMessages() bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.state == ConnectionStateConnecting || c.state == ConnectionStateConnected || c.state == ConnectionStateClosing
}

func (c *Connection) lockIsActive() bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.isActive()
}

func (c *Connection) setConn(conn proto.Conn) {
	c.connMtx.Lock()
	c.conn = conn
	c.connMtx.Unlock()
	go c.eventloop()
}

func (c *Connection) logger() *LoggerOptions {
	return c.auth.logger()
}

func (c *Connection) setSerial(serial int64) {
	c.serial = serial
}

func (c *Connection) resendPending() {
	lg := c.logger().sugar()
	c.mtx.Lock()
	cx := make([]msgCh, len(c.pending.queue))
	copy(cx, c.pending.queue)
	c.pending.queue = []msgCh{}
	c.mtx.Unlock()
	lg.Debugf("resending %d messages waiting for ACK/NACK", len(cx))
	for _, v := range cx {
		c.send(v.msg, v.ch)
	}
}

func (c *Connection) eventloop() {
	var lastActivityAt time.Time
	var connDetails *proto.ConnectionDetails
	for c.lockCanReceiveMessages() {
		receiveTimeout := c.opts.realtimeRequestTimeout()
		if connDetails != nil {
			maxIdleInterval := time.Duration(connDetails.MaxIdleInterval)
			receiveTimeout += maxIdleInterval // RTN23a
		}
		c.connMtx.Lock()
		msg, err := c.conn.Receive(c.opts.Now().Add(receiveTimeout))
		c.connMtx.Unlock()
		if err != nil {
			c.mtx.Lock()
			if c.state == ConnectionStateClosed {
				c.mtx.Unlock()
				return
			}

			// RTN23a
			c.lockSetState(ConnectionStateDisconnected, err, 0)
			c.mtx.Unlock()
			arg := connArgs{
				lastActivityAt: lastActivityAt,
				connDetails:    connDetails,
			}
			c.reconnect(arg)
			return
		}
		lastActivityAt = c.opts.Now()
		if msg.ConnectionSerial != 0 {
			c.mtx.Lock()
			c.setSerial(msg.ConnectionSerial)
			c.mtx.Unlock()
		}
		switch msg.Action {
		case proto.ActionHeartbeat:
		case proto.ActionAck:
			c.mtx.Lock()
			c.pending.Ack(msg, newErrorFromProto(msg.Error))
			c.setSerial(c.serial + 1)
			c.mtx.Unlock()
		case proto.ActionNack:
			c.mtx.Lock()
			c.pending.Nack(msg, newErrorFromProto(msg.Error))
			c.mtx.Unlock()
		case proto.ActionError:

			if msg.Channel != "" {
				c.callbacks.onChannelMsg(msg)
				break
			}

			c.mtx.Lock()
			reauthorizing := c.reauthorizing
			c.reauthorizing = false
			if isTokenError(msg.Error) {
				if reauthorizing {
					c.lockedReauthorizationFailed(newErrorFromProto(msg.Error))
					c.mtx.Unlock()
					return
				}
				// RTN14b
				c.mtx.Unlock()
				c.reauthorize(connArgs{
					lastActivityAt: lastActivityAt,
					connDetails:    connDetails,
					dialOnce:       true,
				})
				return
			}
			c.mtx.Unlock()

			c.failedConnSideEffects(msg.Error)
		case proto.ActionConnected:
			c.mtx.Lock()

			// we need to get this before we set c.key so as to be sure if we were
			// resuming or recovering the connection.
			mode := c.getMode()
			if msg.ConnectionDetails != nil {
				connDetails = msg.ConnectionDetails
				c.key = connDetails.ConnectionKey //(RTN15e) (RTN16d)

				// Spec RSA7b3, RSA7b4, RSA12a
				c.auth.updateClientID(connDetails.ClientID)
			}
			reconnecting := c.reconnecting
			if reconnecting {
				// reset the mode
				c.reconnecting = false
				c.reauthorizing = false
			}
			previousID := c.id
			c.id = msg.ConnectionID
			c.msgSerial = 0
			if reconnecting && mode == recoveryMode && msg.Error == nil {
				// we are setting msgSerial as per (RTN16f)
				msgSerial, err := strconv.ParseInt(strings.Split(c.opts.Recover, ":")[2], 10, 64)
				if err != nil {
					//TODO: how to handle this? Panic?
				}
				c.msgSerial = msgSerial
			}
			c.setSerial(-1)

			if c.state == ConnectionStateClosing {
				// RTN12f
				c.sendClose()
				c.mtx.Unlock()
				return
			}

			c.mtx.Unlock()

			if reconnecting {
				// (RTN15c1) (RTN15c2)
				c.mtx.Lock()
				c.lockSetState(ConnectionStateConnected, newErrorFromProto(msg.Error), 0)
				c.mtx.Unlock()
				// (RTN15c3)
				// we are calling this outside of locks to avoid deadlock because in the
				// RealtimeClient client where this callback is implemented we do some ops
				// with this Conn where we re acquire Conn.Lock again.
				c.callbacks.onReconnected(previousID != msg.ConnectionID)
			} else {
				// preserve old behavior.
				c.mtx.Lock()
				c.lockSetState(ConnectionStateConnected, nil, 0)
				c.mtx.Unlock()
			}
			c.queue.Flush()
		case proto.ActionDisconnected:
			if !isTokenError(msg.Error) {
				// The spec doesn't say what to do in this case, so do nothing.
				// Ably is supposed to then close the transport, which will
				// trigger a transition to DISCONNECTED.
				continue
			}

			if !c.auth.isTokenRenewable() {
				// RTN15h1
				c.failedConnSideEffects(msg.Error)
				return
			}

			// RTN15h2
			c.reauthorize(connArgs{
				lastActivityAt: lastActivityAt,
				connDetails:    connDetails,
			})
			return
		case proto.ActionClosed:
			c.mtx.Lock()
			c.lockSetState(ConnectionStateClosed, nil, 0)
			c.mtx.Unlock()
			if c.conn != nil {
				c.conn.Close()
			}
		default:
			c.callbacks.onChannelMsg(msg)
		}
	}
}

func (c *Connection) failedConnSideEffects(err *proto.ErrorInfo) {
	c.mtx.Lock()
	if c.reconnecting {
		c.reconnecting = false
		c.reauthorizing = false
		c.callbacks.onReconnectionFailed(err)
	}
	c.lockSetState(ConnectionStateFailed, newErrorFromProto(err), 0)
	c.mtx.Unlock()
	c.queue.Fail(newErrorFromProto(err))
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Connection) reauthorize(arg connArgs) {
	c.mtx.Lock()
	_, err := c.auth.reauthorize(c.opts.BaseCtx)

	if err != nil {
		c.lockedReauthorizationFailed(err)
		c.mtx.Unlock()
		return
	}

	// The reauthorize above will have set the new token in c.auth, so
	// reconnecting will use the new token.
	c.reauthorizing = true
	c.mtx.Unlock()
	c.reconnect(arg)
}

func (c *Connection) lockedReauthorizationFailed(err error) {
	c.lockSetState(ConnectionStateDisconnected, err, 0)
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

func (c *Connection) setState(state ConnectionState, err error, retryIn time.Duration) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.lockSetState(state, err, retryIn)
}

func (c *Connection) lockSetState(state ConnectionState, err error, retryIn time.Duration) error {
	if state == ConnectionStateClosed {
		c.key, c.id = "", "" //(RTN16c)
	}

	previous := c.state
	changed := c.state != state
	c.state = state
	c.errorReason = connStateError(state, err)
	change := ConnectionStateChange{
		Current:  c.state,
		Previous: previous,
		Reason:   c.errorReason,
		RetryIn:  retryIn,
	}
	if !changed {
		change.Event = ConnectionEventUpdate
	} else {
		change.Event = ConnectionEvent(change.Current)
	}
	c.internalEmitter.emitter.Emit(change.Event, change)
	c.emitter.Emit(change.Event, change)
	return c.errorReason.unwrapNil()
}

func (c *Connection) ctxCancelOnStateTransition() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	off := c.internalEmitter.OnceAll(func(ConnectionStateChange) {
		cancel()
	})

	return ctx, func() {
		off()
		cancel()
	}
}
