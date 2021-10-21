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

	id           string
	key          string
	serial       *int64
	msgSerial    int64
	connStateTTL durationFromMsecs
	err          error
	conn         conn
	opts         *clientOptions
	pending      pendingEmitter
	queue        *msgQueue
	auth         *Auth

	callbacks connCallbacks
	// reconnecting tracks if we have issued a reconnection request. If we receive any message
	// with this set to true then it's the first message/response after issuing the
	// reconnection request.
	reconnecting bool
	// reauthorizing tracks if the current reconnection attempt is happening
	// after a reauthorization, to avoid re-reauthorizing.
	reauthorizing bool
	arg           connArgs
}

type connCallbacks struct {
	onChannelMsg func(*protocolMessage)
	// onReconnected is called when we get a CONNECTED response from reconnect request. We
	// move this up because some implementation details for (RTN15c) requires
	// access to Channels, and we don't have it here, so we let RealtimeClient do the
	// work.
	onReconnected func(isNewID bool)
	// onReconnectionFailed is called when we get a FAILED response from a
	// reconnection request.
	onReconnectionFailed func(*errorInfo)
}

func newConn(opts *clientOptions, auth *Auth, callbacks connCallbacks) *Connection {
	c := &Connection{
		ConnectionEventEmitter: ConnectionEventEmitter{newEventEmitter(auth.log())},
		state:                  ConnectionStateInitialized,
		internalEmitter:        ConnectionEventEmitter{newEventEmitter(auth.log())},

		opts:      opts,
		pending:   newPendingEmitter(auth.log()),
		auth:      auth,
		callbacks: callbacks,
	}
	auth.onExplicitAuthorize = c.onClientAuthorize
	c.queue = newMsgQueue(c)
	if !opts.NoConnect {
		c.setState(ConnectionStateConnecting, nil, 0)
		go func() {
			c.log().Info("Trying to establish a connection asynchronously")
			if _, err := c.connect(connArgs{}); err != nil {
				c.log().Errorf("Failed to open connection with err:%v", err)
			}
		}()
	}
	return c
}

func (c *Connection) dial(proto string, u *url.URL) (conn conn, err error) {
	start := time.Now()
	c.log().Debugf("Dial protocol=%q url %q ", proto, u.String())
	// (RTN23b)
	query := u.Query()
	query.Add("heartbeats", "true")
	u.RawQuery = query.Encode()
	timeout := c.opts.realtimeRequestTimeout()
	if c.opts.Dial != nil {
		conn, err = c.opts.Dial(proto, u, timeout)
	} else {
		conn, err = dialWebsocket(proto, u, timeout)
	}
	if err != nil {
		c.log().Debugf("Dial Failed in %v with %v", time.Since(start), err)
		return nil, err
	}
	c.log().Debugf("Dial success in %v", time.Since(start))
	return conn, err
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

	IsInReconnectionLoop := c.state == ConnectionStateDisconnected || c.state == ConnectionStateSuspended

	// set state to connecting for initial connect
	c.lockSetState(ConnectionStateConnecting, nil, 0)

	if IsInReconnectionLoop {
		return
	}

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
	connDetails    *connectionDetails
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
		"v":         []string{ablyVersion},
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
	if err := c.auth.authQuery(context.Background(), query); err != nil {
		return nil, err
	}
	switch mode {
	case resumeMode:
		query.Set("resume", c.key)
		if c.serial != nil {
			query.Set("connectionSerial", fmt.Sprint(*c.serial))
		}
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

var errClosedWhileReconnecting = errors.New("connection explicitly closed while trying to reconnect")

func (c *Connection) connectWithRetryLoop(arg connArgs) (result, error) {
	res, err := c.connectWith(arg)
	if err == nil {
		return res, nil
	}
	if arg.dialOnce || !recoverable(err) {
		return nil, c.setState(ConnectionStateFailed, err, 0)
	}

	c.log().Errorf("Received recoverable error %v", err)
	retryIn := c.opts.disconnectedRetryTimeout()
	c.setState(ConnectionStateDisconnected, err, retryIn)
	idleState := ConnectionStateDisconnected

	// If we spend more than the connection state TTL retrying, we move from
	// DISCONNECTED to SUSPENDED, which also changes the retry timeout period.
	stateTTLCtx, cancelStateTTLTimer := context.WithCancel(context.Background())
	defer cancelStateTTLTimer()
	stateTTLTimer := c.opts.After(stateTTLCtx, c.connectionStateTTL())

	for {
		// If the connection transitions, it's because Connect or Close was called
		// explicitly. In that case, skip the wait and either retry connecting
		// immediately (RTN11c) or exit the loop (RTN12d).
		timerCtx, cancelTimer := c.ctxCancelOnStateTransition()
		<-c.opts.After(timerCtx, retryIn)
		cancelTimer()

		switch state := c.State(); state {
		case ConnectionStateConnecting, ConnectionStateDisconnected, ConnectionStateSuspended:
		case ConnectionStateClosed:
			// Close was explicitly called, so stop trying to connect (RTN12d).
			return nil, errClosedWhileReconnecting
		default:
			panic(fmt.Errorf("unexpected state transition: %v -> %v", idleState, state))
		}

		// Before attempting to connect, move from DISCONNECTED to SUSPENDED if
		// more than connectionStateTTL has passed.
		if idleState == ConnectionStateDisconnected {
			select {
			case <-stateTTLTimer:
				// (RTN14e)
				err = fmt.Errorf(connectionStateTTLErrFmt, c.opts.connectionStateTTL())
				c.setState(ConnectionStateSuspended, err, c.opts.suspendedRetryTimeout())
				idleState = ConnectionStateSuspended
				// (RTN14f)
				c.log().Debug("Reached SUSPENDED state while opening connection")
				retryIn = c.opts.suspendedRetryTimeout()
				continue // wait for re-connection with new retry timeout for suspended
			default:
			}
		}

		c.log().Debug("Attempting to open connection")
		res, err := c.connectWith(arg)
		if err == nil {
			return res, nil
		}
		if recoverable(err) {
			// Go back to previous state and wait again until the next
			// connection attempt.
			c.log().Errorf("Received recoverable error %v", err)
			c.setState(idleState, err, retryIn)
			continue
		}
		return nil, c.setState(ConnectionStateFailed, err, 0)
	}
}

func (c *Connection) connectWith(arg connArgs) (result, error) {
	c.mtx.Lock()
	// set ably connection state to connecting, connecting state exists regardless of whether raw connection is successful or not
	if !c.isActive() { // check if already in connecting state
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

	if c.State() == ConnectionStateClosed { // RTN12d - if connection is closed by client, don't try to reconnect
		return nopResult, nil
	}

	// if err is nil, raw connection with server is successful
	conn, err := c.dial(proto, u)
	if err != nil {
		return nil, err
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.setConn(verboseConn{conn: conn, logger: c.log()})
	// Start eventloop
	go c.eventloop()

	c.reconnecting = arg.mode == recoveryMode || arg.mode == resumeMode
	c.arg = arg
	return res, nil
}

func (c *Connection) connectionStateTTL() time.Duration {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.connStateTTL != 0 { // RTN21
		return time.Duration(c.connStateTTL)
	}
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
	msg := &protocolMessage{Action: actionClose}

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
// RTN13
//func (c *Connection) Ping() (ping, pong time.Duration, err error) {
//	return 0, 0, errors.New("TODO")
//}

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
	return strings.Join([]string{c.key, fmt.Sprint(*c.serial), fmt.Sprint(c.msgSerial)}, ":")
}

// Serial gives serial number of a message received most recently. Last known
// serial number is used when recovering connection state.
func (c *Connection) Serial() *int64 {
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

// OffAll deregisters all event handlers.
//
// See package-level documentation on Event Emitter for details.
func (em ConnectionEventEmitter) OffAll() {
	em.emitter.OffAll()
}

func (c *Connection) advanceSerial() {
	const maxint64 = 1<<63 - 1
	c.msgSerial = (c.msgSerial + 1) % maxint64
}

func (c *Connection) send(msg *protocolMessage, listen chan<- error) {
	hasMsgSerial := msg.Action == actionMessage || msg.Action == actionPresence
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
		c.queue.Enqueue(msg, listen) // RTL4i

	case ConnectionStateConnected:
		if err := c.verifyAndUpdateMessages(msg); err != nil {
			c.mtx.Unlock()
			listen <- err
			return
		}
		if hasMsgSerial {
			msg.MsgSerial = c.msgSerial
		}
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
			if hasMsgSerial {
				c.advanceSerial()
			}
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
func (c *Connection) verifyAndUpdateMessages(msg *protocolMessage) (err error) {
	clientID := c.auth.clientIDForCheck()
	connectionID := c.id
	switch msg.Action {
	case actionMessage:
		for _, msg := range msg.Messages {
			if !isClientIDAllowed(clientID, msg.ClientID) {
				return newError(90000, fmt.Errorf("unable to send message as %q", msg.ClientID))
			}
			if clientID == msg.ClientID {
				msg.ClientID = ""
			}
			msg.ConnectionID = connectionID
		}
	case actionPresence:
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

func (c *Connection) setConn(conn conn) {
	c.connMtx.Lock()
	c.conn = conn
	c.connMtx.Unlock()
}

func (c *Connection) log() logger {
	return c.auth.log()
}

func (c *Connection) setSerial(serial *int64) {
	c.serial = serial
}

func (c *Connection) resendPending() {
	c.mtx.Lock()
	cx := c.pending.Dismiss()
	c.mtx.Unlock()
	c.log().Debugf("resending %d messages waiting for ACK/NACK", len(cx))
	for _, v := range cx {
		c.send(v.msg, v.ch)
	}
}

func (c *Connection) eventloop() {
	var lastActivityAt time.Time
	var connDetails *connectionDetails
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
			if c.state == ConnectionStateClosing {
				// RTN12b, RTN12c
				c.lockSetState(ConnectionStateClosed, err, 0)
				c.mtx.Unlock()
				return
			}
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
			c.setSerial(&msg.ConnectionSerial)
			c.mtx.Unlock()
		}
		switch msg.Action {
		case actionHeartbeat:
		case actionAck:
			c.mtx.Lock()
			c.pending.Ack(msg, newErrorFromProto(msg.Error))
			c.mtx.Unlock()
		case actionNack:
			c.mtx.Lock()
			c.pending.Ack(msg, newErrorFromProto(msg.Error))
			c.mtx.Unlock()
		case actionError:

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
		case actionConnected:
			c.mtx.Lock()

			// we need to get this before we set c.key so as to be sure if we were
			// resuming or recovering the connection.
			mode := c.getMode()
			if msg.ConnectionDetails != nil { // RTN21
				connDetails = msg.ConnectionDetails
				c.key = connDetails.ConnectionKey //(RTN15e) (RTN16d)
				c.connStateTTL = connDetails.ConnectionStateTTL
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
			isNewID := previousID != msg.ConnectionID
			if reconnecting && mode == recoveryMode && msg.Error == nil {
				// we are setting msgSerial as per (RTN16f)
				msgSerial, err := strconv.ParseInt(strings.Split(c.opts.Recover, ":")[2], 10, 64)
				if err != nil {
					//TODO: how to handle this? Panic?
				}
				c.msgSerial = msgSerial
			} else if isNewID {
				c.msgSerial = 0
			}

			if c.state == ConnectionStateClosing {
				// RTN12f
				c.sendClose()
				c.mtx.Unlock()
				continue
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
				c.callbacks.onReconnected(isNewID)
			} else {
				// preserve old behavior.
				c.mtx.Lock()
				// RTN24
				c.lockSetState(ConnectionStateConnected, newErrorFromProto(msg.Error), 0)
				c.mtx.Unlock()
			}
			c.queue.Flush()
		case actionDisconnected:
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
		case actionClosed:
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

func (c *Connection) failedConnSideEffects(err *errorInfo) {
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
	_, err := c.auth.reauthorize(context.Background())

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

func (c *Connection) onClientAuthorize(ctx context.Context, token *TokenDetails) {
	switch c.State() {
	case ConnectionStateConnected:
		c.log().Verbosef("starting client-requested reauthorization with token: %+v", token)

		changes := make(connStateChanges, 2)
		{
			off := c.internalEmitter.Once(ConnectionEventUpdate, changes.Receive)
			defer off()
		}
		{
			off := c.internalEmitter.Once(ConnectionEventFailed, changes.Receive)
			defer off()
		}

		c.send(&protocolMessage{
			Action: actionAuth,
			Auth:   &authDetails{AccessToken: token.Token},
		}, nil)

		select {
		case <-ctx.Done():
		case <-changes:
		}
	}
}

func (c *Connection) lockedReauthorizationFailed(err error) {
	c.lockSetState(ConnectionStateDisconnected, err, 0)
}

type verboseConn struct {
	conn   conn
	logger logger
}

func (vc verboseConn) Send(msg *protocolMessage) error {
	vc.logger.Verbosef("Realtime Connection: sending %s", msg)
	return vc.conn.Send(msg)
}

func (vc verboseConn) Receive(deadline time.Time) (*protocolMessage, error) {
	msg, err := vc.conn.Receive(deadline)
	if err != nil {
		return nil, err
	}
	vc.logger.Verbosef("Realtime Connection: received %s", msg)
	return msg, nil
}

func (vc verboseConn) Close() error {
	vc.logger.Verbosef("Realtime Connection: closed")
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

// ctxCancelOnStateTransition returns a context that is canceled when the
// connection transitions to any state.
//
// This is useful for stopping timers when
// another event has caused the connection to transition, thus invalidating the
// original connection state at the time the timer was set.
func (c *Connection) ctxCancelOnStateTransition() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	off := c.internalEmitter.OnceAll(func(change ConnectionStateChange) {
		cancel()
	})

	return ctx, func() {
		off()
		cancel()
	}
}
