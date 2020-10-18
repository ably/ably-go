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
	cancel        context.CancelFunc
	ctx           context.Context
}

type connCallbacks struct {
	onChannelMsg func(*proto.ProtocolMessage)
	// onReconnected is called when we get a CONNECTED response from reconnect request. We
	// move this up because some implementation details for (RTN15c) requires
	// access to Channels and we dont have it here so we let RealtimeClient do the
	// work.
	onReconnected func(_ *proto.ErrorInfo, isNewID bool)
	// onReconnectionFailed is called when we get a FAILED response from a
	// reconnection request.
	onReconnectionFailed func(*proto.ErrorInfo)
}

func newConn(opts *clientOptions, auth *Auth, callbacks connCallbacks) (*Connection, error) {
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
	c.ctx, c.cancel = context.WithCancel(context.Background())
	go c.watchDisconnect(c.ctx)
	if !opts.NoConnect {
		if _, err := c.connect(connArgs{}); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Connection) dial(proto string, u *url.URL) (conn proto.Conn, err error) {
	lg := c.logger().Sugar()
	start := time.Now()
	lg.Debugf("Dial protocol=%q url %q ", proto, u.String())
	// (RTN23b)
	query := u.Query()
	query.Add("heartbeats", "true")
	u.RawQuery = query.Encode()
	// We first try a single dial to see if we are successful. There is no need to
	// have this only in the for loop because most cases we will succeed after the
	// first attempt, let's be prudent here.
	conn, err = c.dialInternal(proto, u, c.opts.realtimeRequestTimeout())
	if err != nil {
		lg.Debugf("Dial Failed in %v with %v", time.Since(start), err)
		return nil, err
	}
	lg.Debugf("Dial success in %v", time.Since(start))
	return conn, err
}

func (c *Connection) connectAfterSuspension(arg connArgs) (Result, error) {
	timeout := c.opts.suspendedRetryTimeout()
	lg := c.logger().Sugar()
	lg.Debugf("Attemting to periodically establish connection after suspension with timeout %v", timeout)
	tick := time.NewTicker(timeout)
	for {
		select {
		case <-c.ctx.Done():
			lg.Debug("exiting connetAfterSuspension loop")
			return nil, c.ctx.Err()
		case <-tick.C:
			res, err := c.connectWith(arg)
			if err != nil {
				if recoverable(err) {
					continue
				}
				return nil, err
			}
			return res, nil
		}
	}
}

func (c *Connection) dialInternal(protocol string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
	if c.opts.Dial != nil {
		return c.opts.Dial(protocol, u, timeout)
	}
	return ablyutil.DialWebsocket(protocol, u, timeout)
}

// recoverable returns true if err is recoverable, err is from making a
// connection
func recoverable(err error) bool {
	if info, ok := err.(*ErrorInfo); ok {
		err = info.err
	}
	switch err {
	case context.DeadlineExceeded:
		return true
	default:
		return false
	}
}

// Connect attempts to move the connection to the CONNECTED state, if it
// can and if it isn't already.
func (c *Connection) Connect() {
	c.mtx.Lock()
	isActive := c.isActive()
	c.mtx.Unlock()
	if isActive {
		return
	}
	c.connect(connArgs{})
}

// Close attempts to move the connection to the CLOSED state, if it can and if
// it isn't already.
func (c *Connection) Close() {
	c.close()
}

func (c *Connection) connect(arg connArgs) (Result, error) {
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
}

func (c *Connection) reconnect(arg connArgs) (Result, error) {
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
	query := url.Values{
		"timestamp": []string{strconv.FormatInt(unixMilli(c.opts.Now()), 10)},
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

func (c *Connection) connectWithRetryLoop(arg connArgs) (Result, error) {
	lg := c.logger().Sugar()
	res, err := c.connectWith(arg)
	if err != nil {
		if !arg.dialOnce && recoverable(err) {
			lg.Errorf("Received recoverable error %v", err)
			c.setState(ConnectionStateDisconnected, err)

			suspend := make(chan ConnectionStateChange)
			off := c.On(ConnectionEventSuspended, func(csc ConnectionStateChange) {
				suspend <- csc
			})
			defer off()

			next := time.NewTimer(c.opts.disconnectedRetryTimeout())
			reset := func() {
				ttl := c.opts.disconnectedRetryTimeout()
				lg.Debugf("Retry in %v", ttl)
				next.Reset(ttl)
			}
			defer next.Stop()
			for {
				select {
				case <-c.ctx.Done():
					lg.Debug("exiting dial retry loop")
					return nopResult, nil
				case <-suspend:
					// (RTN14f)
					lg.Debug("Reached SUSPENDED state while reconnecting")
					return c.connectAfterSuspension(arg)
				case <-next.C:
					lg.Debug("Attemting to dial")
					res, err := c.connectWith(arg)
					if err != nil {
						if recoverable(err) {
							lg.Errorf("Received recoverable error %v", err)
							c.setState(ConnectionStateDisconnected, err)
							reset()
							continue
						}
						return nil, c.setState(ConnectionStateFailed, err)
					}
					return res, nil
				}

			}
		}
		return nil, c.setState(ConnectionStateFailed, err)
	}
	return res, nil
}

func (c *Connection) connectWith(arg connArgs) (Result, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if !c.isActive() {
		c.lockSetState(ConnectionStateConnecting, nil)
	}
	u, err := url.Parse(c.opts.realtimeURL())
	if err != nil {
		return nil, err
	}
	var res Result
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

func (c *Connection) watchDisconnect(ctx context.Context) {
	lg := c.logger().Sugar()
	lg.Debug("Start loop for watching DISCONNECTED events")
	deadline := time.NewTimer(c.connectionStateTTL())
	defer deadline.Stop()
	var watching bool
	watch := make(chan ConnectionStateChange)
	off := c.OnAll(func(csc ConnectionStateChange) {
		watch <- csc
	})
	defer off()
	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			lg.Debugf("DISCONNECTED watch timed out watching=%v", watching)
			if !watching {
				continue
			}
			// (RTN14e)
			lg.Debug("Transition to SUSPENDED state")
			c.setState(ConnectionStateSuspended, context.DeadlineExceeded)
			deadline.Reset(c.connectionStateTTL())
			watching = false
		case change := <-watch:
			switch change.Current {
			case ConnectionStateDisconnected:
				ttl := c.connectionStateTTL()
				lg.Debugf("Received DISCONNECTED event resetting the wtach timer to %v", ttl)
				deadline.Reset(ttl)
				watching = true
			default:
				if !watching {
					continue
				}
				lg.Debugf("Received %v state while watching for DISCONNECTED", change.Current)
				deadline.Reset(c.connectionStateTTL())
				watching = false
			}
		}
	}
}

func (c *Connection) close() {
	c.mtx.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.key, c.id = "", "" //(RTN16c)
	defer c.mtx.Unlock()
	switch c.state {
	case
		ConnectionStateClosing,
		ConnectionStateClosed,
		ConnectionStateInitialized,
		ConnectionStateFailed,
		ConnectionStateDisconnected:

		return
	}

	c.lockSetState(ConnectionStateClosing, nil)
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

func (c *Connection) updateSerial(msg *proto.ProtocolMessage, listen chan<- error) {
	const maxint64 = 1<<63 - 1
	msg.MsgSerial = c.msgSerial
	c.msgSerial = (c.msgSerial + 1) % maxint64
	if listen != nil {
		c.pending.Enqueue(msg.MsgSerial, listen)
	}
}

func (c *Connection) send(msg *proto.ProtocolMessage, listen chan<- error) error {
	c.mtx.Lock()
	switch state := c.state; state {
	case ConnectionStateInitialized, ConnectionStateConnecting, ConnectionStateDisconnected:
		c.mtx.Unlock()
		if c.opts.NoQueueing {
			return connStateError(state, errQueueing)
		}
		c.queue.Enqueue(msg, listen)
		return nil
	case ConnectionStateConnected:
	default:
		c.mtx.Unlock()
		return connStateError(state, nil)
	}
	if err := c.verifyAndUpdateMessages(msg); err != nil {
		c.mtx.Unlock()
		return err
	}
	c.updateSerial(msg, listen)
	c.mtx.Unlock()
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
	var lastActivityAt time.Time
	var connDetails *proto.ConnectionDetails
	for c.lockCanReceiveMessages() {
		var deadline time.Time
		if connDetails != nil {
			maxIdleInterval := time.Duration(connDetails.MaxIdleInterval)
			receiveTimeout := c.opts.realtimeRequestTimeout() + maxIdleInterval // RTN23a
			deadline = c.opts.Now().Add(receiveTimeout)                         // RTNf23a
		}
		msg, err := c.conn.Receive(deadline)
		if err != nil {
			c.mtx.Lock()
			if c.state == ConnectionStateClosed {
				c.mtx.Unlock()
				return
			}

			// RTN23a
			c.lockSetState(ConnectionStateDisconnected, err)
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
			c.pending.Ack(msg.MsgSerial, msg.Count, newErrorProto(msg.Error))
			c.setSerial(c.serial + 1)
			c.mtx.Unlock()
		case proto.ActionNack:
			c.mtx.Lock()
			c.pending.Nack(msg.MsgSerial, msg.Count, newErrorProto(msg.Error))
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
					c.lockedReauthorizationFailed(newErrorProto(msg.Error))
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
			if reconnecting && mode == recoveryMode {
				// we are setting msgSerial as per (RTN16f)
				msgSerial, err := strconv.ParseInt(strings.Split(c.opts.Recover, ":")[2], 10, 64)
				if err != nil {
					//TODO: how to handle this? Panic?
				}
				c.msgSerial = msgSerial
			}
			c.setSerial(-1)
			c.mtx.Unlock()
			if reconnecting {
				// (RTN15c1) (RTN15c2)
				c.mtx.Lock()
				c.lockSetState(ConnectionStateConnected, newErrorProto(msg.Error))
				c.mtx.Unlock()
				if previousID != msg.ConnectionID {
					// (RTN15c3)
					// we are calling this outside of locks to avoid deadlock because in the
					// RealtimeClient client where this callback is implemented we do some ops
					// with this Conn where we re acquire Conn.Lock again.
					c.callbacks.onReconnected(msg.Error, true)
				}
			} else {
				// preserve old behavior.
				c.mtx.Lock()
				c.lockSetState(ConnectionStateConnected, nil)
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
			c.id, c.key = "", "" //(RTN16c)
			c.lockSetState(ConnectionStateClosed, nil)
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
	c.lockSetState(ConnectionStateFailed, newErrorProto(err))
	c.mtx.Unlock()
	c.queue.Fail(newErrorProto(err))
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Connection) reauthorize(arg connArgs) {
	c.mtx.Lock()
	_, err := c.auth.reauthorize()

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
	c.lockSetState(ConnectionStateDisconnected, err)
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

func (c *Connection) setState(state ConnectionState, err error) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.lockSetState(state, err)
}

func (c *Connection) lockSetState(state ConnectionState, err error) error {
	previous := c.state
	changed := c.state != state
	c.state = state
	c.errorReason = connStateError(state, err)
	change := ConnectionStateChange{
		Current:  c.state,
		Previous: previous,
		Reason:   c.errorReason,
	}
	if !changed {
		change.Event = ConnectionEventUpdated
	} else {
		change.Event = ConnectionEvent(change.Current)
	}
	c.internalEmitter.emitter.Emit(change.Event, change)
	c.emitter.Emit(change.Event, change)
	if c.errorReason == nil {
		return nil
	}
	return c.errorReason
}
