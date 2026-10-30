package ably

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
)

var (
	errQueueing = errors.New("unable to send messages in current state with disabled queueing")
)

// connectionMode is the mode in which the connection is operating.
type connectionMode uint

const (
	// normalMode is set when the Connection operating normally.
	normalMode connectionMode = iota
	// resumeMode is set when the Connection is trying to resume.
	resumeMode
	// recoveryMode is set when the Connection is trying to recover.
	recoveryMode
)

// Connection represents a single connection Realtime instantiates for communication with Ably servers.
// It also enables the management of a connection to Ably.
type Connection struct {
	mtx sync.Mutex

	// interruptions counts the times the connection has entered SUSPENDED,
	// CLOSED or FAILED. It's accessed atomically.
	interruptions uint32
	// interruptionErr is the reason for the latest interruption. It's
	// guarded by mtx.
	interruptionErr error
	// connectGen counts the connection attempts that Connect has started
	// after Close or a failure. Retries and token renewals belonging to an
	// earlier attempt stop rather than run alongside the new one. It's
	// written with mtx held, and accessed atomically.
	connectGen uint32
	// failedAcks call the callbacks of messages awaiting an ACK that an
	// interruption failed. A state change handler calls them, without the
	// lock held, so that they can call the client. It's guarded by mtx.
	failedAcks []func()
	// disconnectedAt is when the connection last became DISCONNECTED after
	// being connected, closed or failed. It's kept through retries, and while
	// SUSPENDED, so that a token renewal that keeps failing reaches SUSPENDED
	// (RTN14e). It's guarded by mtx.
	disconnectedAt time.Time

	// connMtx - on setConn we write to conn with mtx protection, however in eventLoop we
	// read conn unprotected, this is racy because now we establish connection in a
	// separate goroutine.
	//
	// using mtx to protect reads in eventLoop causes a deadlock.
	connMtx sync.Mutex

	// ConnectionEventEmitter embeds an [ably.ConnectionEventEmitter] object (RTN4a, RTN4e, RTN4g).
	ConnectionEventEmitter

	// state is the current [ably.ConnectionState] of the connection (RTN4d).
	state ConnectionState

	// errorReason is an [ably.ErrorInfo] object describing the last error received if
	// a connection failure occurs (RTN14a, RTN15c7, RTN25).
	errorReason *ErrorInfo

	internalEmitter ConnectionEventEmitter

	// id is a unique public identifier for this connection, used to identify this member (RTN8).
	id string

	// key is a unique private connection key used to recover or resume a connection, assigned by Ably.
	// When recovering a connection explicitly, the recoveryKey is used in the recover client options as
	// it contains both the key and the last message serial. This private connection key can also be used by
	// other REST clients to publish on behalf of this client. See the publishing over REST on behalf of
	// a realtime client docs for more info (RTN9).
	key string

	msgSerial    int64
	connStateTTL durationFromMsecs
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
	client        *Realtime

	readLimit int64
	recover   string
}

type connCallbacks struct {
	onChannelMsg func(*protocolMessage)
	// onReconnected is called when we get a CONNECTED response from reconnect request. We
	// move this up because some implementation details for (RTN15c) requires
	// access to Channels, and we don't have it here, so we let RealtimeClient do the
	// work.
	onReconnected func(failedResumeOrRecover bool)
	// onReconnectionFailed is called when we get a FAILED response from a
	// reconnection request.
	onReconnectionFailed func(*errorInfo)
}

func newConn(opts *clientOptions, auth *Auth, callbacks connCallbacks, client *Realtime) *Connection {
	c := &Connection{
		ConnectionEventEmitter: ConnectionEventEmitter{newEventEmitter(auth.log())},
		state:                  ConnectionStateInitialized,
		internalEmitter:        ConnectionEventEmitter{newEventEmitter(auth.log())},

		opts:      opts,
		pending:   newPendingEmitter(auth.log()),
		auth:      auth,
		callbacks: callbacks,
		client:    client,
		readLimit: defaultReadLimit,
		recover:   opts.Recover,
	}
	auth.onExplicitAuthorize = c.onExplicitAuthorize
	c.queue = newMsgQueue(c)
	c.internalEmitter.OnAll(func(change ConnectionStateChange) {
		if isInterruption(change) {
			c.callFailedAcks()
			c.queue.FailInterrupted(interruptionError(change)) // RTN7e
		}
	})
	if !opts.NoConnect {
		c.setState(ConnectionStateConnecting, nil, 0)
		go func() {
			c.log().Info("Trying to establish a connection asynchronously")
			if _, err := c.connect(connArgs{}); err != nil && err != errClosedWhileReconnecting {
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
		conn, err = dialWebsocket(proto, u, timeout, c.opts.Agents)
	}
	if err != nil {
		c.log().Debugf("Dial Failed in %v with %v", time.Since(start), err)
		return nil, err
	}
	c.log().Debugf("Dial success in %v", time.Since(start))
	return conn, err
}

// recoverable returns true if err is recoverable for connection error
func recoverable(err error) bool {
	var e *ErrorInfo
	if errors.As(err, &e) {
		return !(40000 <= e.Code && e.Code < 50000)
	}
	return true
}

// Connect attempts to move the connection to the CONNECTED state, if it can and if it isn't already.
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

	arg := connArgs{gen: atomic.AddUint32(&c.connectGen, 1)}
	go func() {
		c.connect(arg)
	}()
}

// SetReadLimit is used to override internal websocket connection read limit.
// It sets the max number of bytes to read for a single message.
// By default, the connection has a message read limit of [ably.defaultReadLimit] or 2mb.
// When the limit is hit, the connection will be closed with StatusMessageTooBig.
func (c *Connection) SetReadLimit(readLimit int64) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.readLimit = readLimit
}

// Close causes the connection to close, entering the [ably.ConnectionStateClosing] state.
// Once connection state is [ably.ConnectionStateClosed], the library does not attempt to re-establish
// the connection without an explicit call to Connection.Connect (RTN12).
// Messages that are queued or awaiting an acknowledgement fail when the connection closes (RTN7e).
func (c *Connection) Close() {
	c.close()
}

// Connect calling this method is needed only if the NoConnect attribute of the clientOptions object is true.
// If not in connecting or connected state, this method causes the connection to open, entering the
// [ably.ConnectionStateConnecting] state (RTC1b, RTN3, RTN11).
func (c *Connection) connect(arg connArgs) (result, error) {
	return c.connectWithRetryLoop(arg)
}

type connArgs struct {
	lastActivityAt time.Time
	connDetails    *connectionDetails
	result         bool
	dialOnce       bool
	retryIn        time.Duration
	// gen is the connectGen of the connection attempt that this belongs to.
	gen uint32
}

func (c *Connection) reconnect(arg connArgs) (result, error) {
	c.mtx.Lock()

	if arg.connDetails != nil && c.opts.Now().Sub(arg.lastActivityAt) >= time.Duration(arg.connDetails.ConnectionStateTTL+arg.connDetails.MaxIdleInterval) {
		// RTN15g
		c.msgSerial = 0
		c.key = ""
		// c.id isn't cleared since it's used later to determine if the
		// reconnection resulted in a new transport-level connection.
	}

	c.mtx.Unlock()
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
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.key != "" {
		return resumeMode
	}
	if c.recover != "" {
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
		"v":         []string{ablyProtocolVersion},
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
		query.Set("resume", c.key) // RTN15b
	case recoveryMode:
		recoveryKeyContext, err := DecodeRecoveryKey(c.recover)
		if err != nil {
			// Ignoring error since no recover will be used for new connection
			c.log().Errorf("Error decoding recovery key, %v", err)
			c.log().Errorf("Trying a fresh connection instead")
		} else {
			query.Set("recover", recoveryKeyContext.ConnectionKey) // RTN16k
		}
	}
	return query, nil
}

const connectionStateTTLErrFmt = "Exceeded connectionStateTtl=%v while in DISCONNECTED state"

var errClosedWhileReconnecting = errors.New("connection explicitly closed while trying to reconnect")

func (c *Connection) connectWithRetryLoop(arg connArgs) (result, error) {
	res, err := c.connectWith(arg)
	if err == nil || err == errClosedWhileReconnecting {
		return res, err
	}
	if arg.dialOnce || !recoverable(err) {
		return nil, c.setStateUnlessClosing(arg.gen, ConnectionStateFailed, err, 0)
	}

	c.log().Errorf("Received recoverable error %v", err)
	// A retry from SUSPENDED, or one after failing to renew the token, may
	// already have been DISCONNECTED for longer than the connection state TTL.
	c.mtx.Lock()
	idleState, retryIn := c.lockRetryState()
	c.mtx.Unlock()
	if err := c.setStateUnlessClosing(arg.gen, idleState, err, retryIn); err == errClosedWhileReconnecting {
		return nil, err
	}

	// If we spend more than the connection state TTL retrying, we move from
	// DISCONNECTED to SUSPENDED, which also changes the retry timeout period.
	stateTTLCtx, cancelStateTTLTimer := context.WithCancel(context.Background())
	defer cancelStateTTLTimer()
	stateTTLTimer := c.opts.After(stateTTLCtx, c.suspendIn())

	for {
		// If the connection transitions, it's because Connect or Close was called
		// explicitly. In that case, skip the wait and either retry connecting
		// immediately (RTN11c) or exit the loop (RTN12d).
		timerCtx, cancelTimer := c.ctxCancelOnStateTransition()
		if c.State() == idleState { // else it transitioned before we listened
			<-c.opts.After(timerCtx, retryIn)
		}
		cancelTimer()

		switch state := c.State(); state {
		case ConnectionStateConnecting, ConnectionStateDisconnected, ConnectionStateSuspended:
		case ConnectionStateClosing, ConnectionStateClosed:
			// Close was explicitly called, so stop trying to connect (RTN12d,
			// RTN12f).
			c.mtx.Lock()
			c.lockFinishClose()
			c.mtx.Unlock()
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
				if err := c.setStateUnlessClosing(arg.gen, ConnectionStateSuspended, err, c.opts.suspendedRetryTimeout()); err == errClosedWhileReconnecting {
					return nil, err
				}
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
		if err == nil || err == errClosedWhileReconnecting {
			return res, err
		}
		if recoverable(err) {
			// Go back to previous state and wait again until the next
			// connection attempt.
			c.log().Errorf("Received recoverable error %v", err)
			if err := c.setStateUnlessClosing(arg.gen, idleState, err, retryIn); err == errClosedWhileReconnecting {
				return nil, err
			}
			continue
		}
		return nil, c.setStateUnlessClosing(arg.gen, ConnectionStateFailed, err, 0)
	}
}

// setStateUnlessClosing sets state after a failed connection attempt or a
// server DISCONNECTED, unless Close was called. Then it finishes the close
// instead and returns errClosedWhileReconnecting (RTN12f). If Connect has
// since started a new attempt, it leaves the state to that attempt.
func (c *Connection) setStateUnlessClosing(gen uint32, state ConnectionState, err error, retryIn time.Duration) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.superseded(gen) || c.lockFinishClose() {
		return errClosedWhileReconnecting
	}
	return c.lockSetState(state, err, retryIn)
}

// superseded reports whether Connect has started a new connection attempt,
// after Close or a failure, since the one numbered gen. That attempt owns the
// connection now.
func (c *Connection) superseded(gen uint32) bool {
	return gen != atomic.LoadUint32(&c.connectGen)
}

// lockFinishClose moves a CLOSING connection to CLOSED, and reports whether
// the connection has been closed.
func (c *Connection) lockFinishClose() bool {
	switch c.state {
	case ConnectionStateClosing:
		c.lockSetState(ConnectionStateClosed, nil, 0)
		return true
	case ConnectionStateClosed:
		return true
	}
	return false
}

func (c *Connection) connectWith(arg connArgs) (result, error) {
	connectMode := c.getMode()

	c.mtx.Lock()
	if c.superseded(arg.gen) || c.lockFinishClose() { // RTN12d, RTN12f - closed by client, don't try to connect
		c.mtx.Unlock()
		return nil, errClosedWhileReconnecting
	}
	// set ably connection state to connecting, connecting state exists regardless of whether raw connection is successful or not
	if !c.isActive() { // check if already in connecting state
		c.lockSetState(ConnectionStateConnecting, nil, 0)
	}
	c.mtx.Unlock()

	var res result
	if arg.result {
		res = c.internalEmitter.listenResult(
			ConnectionStateConnected, // expected state
			ConnectionStateFailed,
			ConnectionStateDisconnected,
		)
	}

	var conn conn
	primaryHost := c.opts.getRealtimeHost()
	hosts := []string{primaryHost}
	fallbackHosts, err := c.opts.getFallbackHosts()
	if err != nil {
		c.log().Warn(err)
	} else {
		hosts = append(hosts, ablyutil.Shuffle(fallbackHosts)...)
	}
	// Always try primary host first and then fallback hosts for realtime conn
	for hostCounter, host := range hosts {
		u, err := url.Parse(c.opts.realtimeURL(host))
		if err != nil {
			return nil, err
		}
		query, err := c.params(connectMode)
		if err != nil {
			return nil, err
		}
		u.RawQuery = query.Encode()
		proto := c.opts.protocol()

		c.mtx.Lock()
		closed := c.superseded(arg.gen) || c.lockFinishClose()
		c.mtx.Unlock()
		if closed { // RTN12d, RTN12f - if connection is closed by client, don't try to reconnect
			return nil, errClosedWhileReconnecting
		}
		// if err is nil, raw connection with server is successful
		conn, err = c.dial(proto, u)
		if err != nil {
			resp := extractHttpResponseFromError(err)
			if hostCounter < len(hosts)-1 && canFallBack(err, resp) && c.opts.hasActiveInternetConnection() { // RTN17d, RTN17c
				continue
			}
			return nil, err
		}
		if host != primaryHost { // RTN17e
			c.client.rest.setActiveRealtimeHost(host)
		} else if !empty(c.client.rest.activeRealtimeHost) {
			c.client.rest.setActiveRealtimeHost("") // reset to default
		}
		break
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.superseded(arg.gen) {
		// Close and then Connect were called while dialing, so the newer
		// attempt dials its own transport.
		conn.Close()
		return nil, errClosedWhileReconnecting
	}
	vc := verboseConn{conn: conn, logger: c.log()}
	c.setConn(vc)
	// Start eventloop
	go c.eventloop(vc, arg.gen)

	c.reconnecting = connectMode == recoveryMode || connectMode == resumeMode
	c.arg = arg
	return res, nil
}

func (c *Connection) connectionStateTTL() time.Duration {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.lockConnectionStateTTL()
}

func (c *Connection) lockConnectionStateTTL() time.Duration {
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

// ErrorReason gives last known error that caused connection transition to [ably.ConnectionStateFailed] state.
func (c *Connection) ErrorReason() *ErrorInfo {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.errorReason
}

// Deprecated: this property is deprecated, use CreateRecoveryKey method instead.
func (c *Connection) RecoveryKey() string {
	c.log().Warn("RecoveryKey is deprecated, use CreateRecoveryKey method instead")
	return c.CreateRecoveryKey()
}

// CreateRecoveryKey is an attribute composed of the connectionKey, messageSerial and channelSerials (RTN16g, RTN16g1, RTN16h).
func (c *Connection) CreateRecoveryKey() string {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	// RTN16g2
	if empty(c.key) || c.state == ConnectionStateClosing ||
		c.state == ConnectionStateClosed ||
		c.state == ConnectionStateFailed ||
		c.state == ConnectionStateSuspended {
		return ""
	}
	recoveryContext := RecoveryKeyContext{
		ConnectionKey:  c.key,
		MsgSerial:      c.msgSerial,
		ChannelSerials: c.client.Channels.GetChannelSerials(),
	}
	recoveryKey, err := recoveryContext.Encode()
	if err != nil {
		c.log().Errorf("Error while encoding recoveryKey %v", err)
	}
	return recoveryKey
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
// See package-level documentation => [ably] Event Emitters for more details.
func (em ConnectionEventEmitter) On(e ConnectionEvent, handle func(ConnectionStateChange)) (off func()) {
	return em.emitter.On(e, func(change emitterData) {
		handle(change.(ConnectionStateChange))
	})
}

// OnAll registers an event handler for all connection events.
//
// See package-level documentation => [ably] Event Emitters for more details.
func (em ConnectionEventEmitter) OnAll(handle func(ConnectionStateChange)) (off func()) {
	return em.emitter.OnAll(func(change emitterData) {
		handle(change.(ConnectionStateChange))
	})
}

// Once registers an one-off event handler for connection events of a specific kind.
//
// See package-level documentation => [ably] Event Emitters for more details.
func (em ConnectionEventEmitter) Once(e ConnectionEvent, handle func(ConnectionStateChange)) (off func()) {
	return em.emitter.Once(e, func(change emitterData) {
		handle(change.(ConnectionStateChange))
	})
}

// OnceAll registers an one-off event handler for all connection events.
//
// See package-level documentation => [ably] Event Emitters for more details.
func (em ConnectionEventEmitter) OnceAll(handle func(ConnectionStateChange)) (off func()) {
	return em.emitter.OnceAll(func(change emitterData) {
		handle(change.(ConnectionStateChange))
	})
}

// Off deregisters event handlers for connection events of a specific kind.
//
// See package-level documentation => [ably] Event Emitters for more details.
func (em ConnectionEventEmitter) Off(e ConnectionEvent) {
	em.emitter.Off(e)
}

// OffAll deregisters all event handlers.
//
// See package-level documentation => [ably] Event Emitters for more details.
func (em ConnectionEventEmitter) OffAll() {
	em.emitter.OffAll()
}

func (c *Connection) advanceSerial() {
	const maxint64 = 1<<63 - 1
	c.msgSerial = (c.msgSerial + 1) % maxint64
}

// send sends a message with an ackCallback.
func (c *Connection) send(msg *protocolMessage, ackCallback *msgAckCallback) {
	hasMsgSerial := msg.Action == actionMessage || msg.Action == actionPresence || msg.Action == actionObject
	c.mtx.Lock()
	// RTP16a - in case of presence msg send, check for connection status and send accordingly
	switch state := c.state; state {
	default:
		c.mtx.Unlock()
		if ackCallback != nil {
			if c.state == ConnectionStateClosed {
				ackCallback.call(nil, errClosed)
			} else {
				ackCallback.call(nil, connStateError(state, nil))
			}
		}

	case ConnectionStateInitialized, ConnectionStateConnecting, ConnectionStateDisconnected:
		interruptions := c.interruptionCount()
		c.mtx.Unlock()
		if c.opts.NoQueueing {
			if ackCallback != nil {
				ackCallback.call(nil, connStateError(state, errQueueing))
			}
		} else {
			c.queue.Enqueue(msg, ackCallback, interruptions) // RTL4i
		}
	case ConnectionStateConnected:
		if err := c.verifyAndUpdateMessages(msg); err != nil {
			c.mtx.Unlock()
			if ackCallback != nil {
				ackCallback.call(nil, err)
			}
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
			c.log().Warnf("transport level failure while sending message, %v", err)
			c.conn.Close()
			interruptions := c.interruptionCount()
			c.mtx.Unlock()
			c.queue.Enqueue(msg, ackCallback, interruptions)
		} else {
			if hasMsgSerial {
				c.advanceSerial()
			}
			if ackCallback != nil {
				c.pending.Enqueue(msg, ackCallback)
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
	oldConn := c.conn
	c.conn = conn
	c.connMtx.Unlock()
	if oldConn != nil {
		// We don't want to block new connections waiting for the previous connection
		// to close. The `conn` interface does not allow us to pass in a `context.Context`
		// so we can't apply a timeout to this go-routine. The websocket library does apply
		// its own 5s timeout to close the connection, so we can be confident this go-routine
		// will not leak.
		// see https://github.com/coder/websocket/blob/v1.8.7/write.go#L233
		go oldConn.Close()
	}
}

func (c *Connection) log() logger {
	return c.auth.log()
}

func (c *Connection) resendAcks() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.log().Debugf("resending %d messages waiting for ACK/NACK", len(c.pending.queue))
	for _, v := range c.pending.queue {
		c.conn.Send(v.msg)
	}
}

func (c *Connection) resendPending() {
	c.mtx.Lock()
	cx := c.pending.Dismiss()
	c.mtx.Unlock()
	c.log().Debugf("resending %d messages waiting for ACK/NACK", len(cx))
	for _, v := range cx {
		c.send(v.msg, v.ackCallback)
	}
}

// eventloop receives from conn until it fails or the connection stops
// receiving messages. Then it closes conn, which nothing else would close, and
// only then reconnects if need be, so that conn isn't kept open while the
// reconnection retries.
func (c *Connection) eventloop(conn conn, gen uint32) {
	next := c.receive(conn, gen)
	conn.Close()
	if next != nil {
		next()
	}
}

// receive handles messages from conn until it fails or the connection stops
// receiving messages. It returns what to do once conn is closed, if anything.
func (c *Connection) receive(conn conn, gen uint32) (next func()) {
	var lastActivityAt time.Time
	var connDetails *connectionDetails
	for c.lockCanReceiveMessages() {
		receiveTimeout := c.opts.realtimeRequestTimeout()
		if connDetails != nil {
			maxIdleInterval := time.Duration(connDetails.MaxIdleInterval)
			receiveTimeout += maxIdleInterval // RTN23a
		}
		c.connMtx.Lock()
		msg, err := conn.Receive(c.opts.Now().Add(receiveTimeout))
		c.connMtx.Unlock()
		if c.superseded(gen) {
			// Close and then Connect were called, so conn belongs to a closed
			// connection and the newer attempt owns the state.
			return nil
		}
		if err != nil {
			c.mtx.Lock()
			if c.state == ConnectionStateClosing {
				// RTN12b, RTN12c
				c.lockSetState(ConnectionStateClosed, err, 0)
				c.mtx.Unlock()
				return nil
			}
			if c.state == ConnectionStateClosed {
				c.mtx.Unlock()
				return nil
			}
			if isMessageTooBigErr(err) {
				// A message exceeding the read limit is unrecoverable: resuming
				// would redeliver the same oversized message and fail again, so
				// reconnecting just spins. Fail the connection instead.
				c.lockSetState(ConnectionStateFailed, err, 0)
				c.mtx.Unlock()
				return nil
			}
			// RTN23a, RTN15a
			c.lockSetState(ConnectionStateDisconnected, err, 0)
			c.mtx.Unlock()
			arg := connArgs{
				lastActivityAt: lastActivityAt,
				connDetails:    connDetails,
				gen:            gen,
			}
			return func() { c.reconnect(arg) }
		}
		lastActivityAt = c.opts.Now()
		msg.updateInnerMessagesEmptyFields() // TM2a, TM2c, TM2f
		switch msg.Action {
		case actionHeartbeat:
		case actionAck, actionNack:
			c.mtx.Lock()
			callAcks := c.pending.Ack(msg, newErrorFromProto(msg.Error))
			c.mtx.Unlock()
			callAcks()
		case actionError:

			if msg.Channel != "" {
				c.callbacks.onChannelMsg(msg)
				break
			}

			c.mtx.Lock()
			reauthorizing := c.reauthorizing
			c.reauthorizing = false
			if isTokenError(msg.Error) {
				// RSA4a2: with no means to renew the token, the connection
				// fails, even while closing, as for DISCONNECTED (RTN15h1).
				if !c.auth.isTokenRenewable() {
					c.mtx.Unlock()
					c.failedConnSideEffects(msg.Error)
					return nil
				}
				if c.lockFinishClose() {
					// Close was called, so don't renew the token and
					// reconnect (RTN12f). Any other ERROR is fatal and
					// fails the connection even while closing (RTN14g).
					c.mtx.Unlock()
					return nil
				}
				if reauthorizing {
					c.lockedReauthorizationFailed(newErrorFromProto(msg.Error))
					c.mtx.Unlock()
					arg := connArgs{
						lastActivityAt: lastActivityAt,
						connDetails:    connDetails,
						gen:            gen,
					}
					return func() { c.retryAfterReauthorizationFailed(arg) }
				}
				// RTN14b
				c.mtx.Unlock()
				arg := connArgs{
					lastActivityAt: lastActivityAt,
					connDetails:    connDetails,
					gen:            gen,
					dialOnce:       true,
				}
				return func() { c.reauthorize(arg) }
			}
			c.mtx.Unlock()

			c.failedConnSideEffects(msg.Error)
			return nil
		case actionConnected:
			c.mtx.Lock()

			// recover is used when set via clientOptions#recover initially, resume will be used for all reconnects.
			isConnectionResumeOrRecoverAttempt := !empty(c.key) || !empty(c.recover)
			c.recover = "" // RTN16k, explicitly setting null so it won't be used for subsequent connection requests

			// we need to get this before we set c.key so as to be sure if we were
			// resuming or recovering the connection.
			if msg.ConnectionDetails != nil { // RTN21
				connDetails = msg.ConnectionDetails
				c.key = connDetails.ConnectionKey //(RTN15e) (RTN16d)
				c.connStateTTL = connDetails.ConnectionStateTTL
				// Spec RSA7b3, RSA7b4, RSA12a
				c.auth.updateClientID(connDetails.ClientID)
			}
			err := setConnectionReadLimit(c.conn, c.readLimit)
			if err != nil {
				c.readLimit = -1
				c.log().Errorf("%v, using default readlimit instead", err)
			} else {
				c.log().Verbosef("connection readlimit set to %v", c.readLimit)
			}
			reconnecting := c.reconnecting
			if reconnecting {
				// reset the mode
				c.reconnecting = false
				c.reauthorizing = false
			}

			isNewID := c.id != msg.ConnectionID
			c.id = msg.ConnectionID

			failedResumeOrRecover := isNewID && msg.Error != nil // RTN15c7, RTN16d

			if isConnectionResumeOrRecoverAttempt && failedResumeOrRecover {
				c.msgSerial = 0
			}

			if c.state == ConnectionStateClosing {
				// RTN12f
				c.sendClose()
				c.mtx.Unlock()
				continue
			}

			// RTN24, RTN15c6, RTN15c7 - if error, set on connection and part of emitted connected event
			c.lockSetState(ConnectionStateConnected, newErrorFromProto(msg.Error), 0)
			c.mtx.Unlock()

			if reconnecting {
				c.callbacks.onReconnected(failedResumeOrRecover)
			}
			c.queue.Flush()
		case actionDisconnected: // RTN15h
			if isTokenError(msg.Error) {
				// RTN15h1, even while closing: the connection fails rather
				// than closes, but won't retry either way.
				if !c.auth.isTokenRenewable() {
					c.failedConnSideEffects(msg.Error)
					return nil
				}
				// RTN15h2, RTN22a. Ably closes the transport after
				// DISCONNECTED (RTN15h), so if Close was called, finish
				// closing rather than reconnecting (RTN12c, RTN12f).
				if c.setStateUnlessClosing(gen, ConnectionStateDisconnected, newErrorFromProto(msg.Error), 0) == errClosedWhileReconnecting {
					return nil
				}
				arg := connArgs{
					lastActivityAt: lastActivityAt,
					connDetails:    connDetails,
					gen:            gen,
				}
				return func() { c.reauthorize(arg) }
			}
			// RTN15h3, and RTN12c and RTN12f as above.
			if c.setStateUnlessClosing(gen, ConnectionStateDisconnected, newErrorFromProto(msg.Error), 0) == errClosedWhileReconnecting {
				return nil
			}
			arg := connArgs{
				lastActivityAt: lastActivityAt,
				connDetails:    connDetails,
				gen:            gen,
			}
			return func() { c.reconnect(arg) }

		case actionClosed:
			c.mtx.Lock()
			c.lockSetState(ConnectionStateClosed, nil, 0)
			c.mtx.Unlock()
		case actionAuth: // RTN22
			canceledCtx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel context to unblock current eventloop to receieve new messages
			c.auth.Authorize(canceledCtx, c.auth.params)
		default:
			c.callbacks.onChannelMsg(msg)
		}
	}
	return nil
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
}

func (c *Connection) reauthorize(arg connArgs) {
	// Fetching a token takes as long as the authCallback or authUrl does, so
	// don't hold the connection lock meanwhile, which would block Close.
	_, err := c.auth.reauthorize(context.Background())

	c.mtx.Lock()
	if c.superseded(arg.gen) || c.lockFinishClose() { // RTN12f
		c.mtx.Unlock()
		return
	}
	if err != nil {
		c.lockedReauthorizationFailed(err)
		c.mtx.Unlock()
		c.retryAfterReauthorizationFailed(arg)
		return
	}

	// The reauthorize above will have set the new token in c.auth, so
	// reconnecting will use the new token.
	c.reauthorizing = true
	c.mtx.Unlock()
	if _, err := c.reconnect(arg); err == errClosedWhileReconnecting {
		// No reconnection response will arrive to clear this.
		c.mtx.Lock()
		c.reauthorizing = false
		c.mtx.Unlock()
	}
}

func (c *Connection) onExplicitAuthorize(ctx context.Context, token *TokenDetails) error {
	switch state := c.State(); state {
	case ConnectionStateConnecting:
		// RTC8b says: "all current connection attempts should be halted, and
		// after obtaining a new token the library should immediately initiate a
		// connection attempt using the new token". But the WebSocket library
		// doesn't really allow us to halt the connection attempt. Instead, once
		// the connection transitions out of CONNECTING (either to CONNECTED or
		// to a failure state), we attempt to connect again, which will use
		// the new token.
		c.log().Info("client-requested authorization while CONNECTING. Will reconnect with new token.")
		done := make(chan error)

		c.internalEmitter.OnceAll(func(_ ConnectionStateChange) {
			done <- c.onExplicitAuthorize(ctx, token)
		})

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-done:
			return err
		}

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
			return ctx.Err()
		case change := <-changes:
			return change.Reason.unwrapNil()
		}

	case
		ConnectionStateDisconnected,
		ConnectionStateSuspended,
		ConnectionStateFailed,
		ConnectionStateClosed:
		c.log().Infof("client-requested authorization while %s: connecting with new token", state)

		done := make(chan error)
		c.internalEmitter.OnceAll(func(change ConnectionStateChange) {
			if change.Current == ConnectionStateConnecting {
				done <- c.onExplicitAuthorize(ctx, token)
			} else {
				done <- change.Reason.unwrapNil()
			}
		})

		c.Connect()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-done:
			return err
		}
	}

	return nil
}

// lockedReauthorizationFailed leaves the connection DISCONNECTED or SUSPENDED
// to retry.
func (c *Connection) lockedReauthorizationFailed(err error) {
	state, retryIn := c.lockRetryState()
	c.lockSetState(state, err, retryIn)
}

// lockRetryState returns the state to retry from after failing to connect,
// and its retry timeout: DISCONNECTED, or SUSPENDED once the connection has
// been DISCONNECTED for longer than connectionStateTTL (RTN14e). Retries start
// new retry loops, so this goes by when the connection became DISCONNECTED
// rather than by a loop's own timer.
func (c *Connection) lockRetryState() (ConnectionState, time.Duration) {
	if !c.disconnectedAt.IsZero() && c.lockSuspendIn() <= 0 {
		return ConnectionStateSuspended, c.opts.suspendedRetryTimeout()
	}
	return ConnectionStateDisconnected, c.opts.disconnectedRetryTimeout()
}

// suspendIn returns how long until the connection has been DISCONNECTED for
// longer than connectionStateTTL.
func (c *Connection) suspendIn() time.Duration {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.lockSuspendIn()
}

func (c *Connection) lockSuspendIn() time.Duration {
	ttl := c.lockConnectionStateTTL()
	if c.disconnectedAt.IsZero() {
		return ttl
	}
	return ttl - c.opts.Now().Sub(c.disconnectedAt)
}

// retryAfterReauthorizationFailed reconnects after the retry timeout, once a
// token renewal has failed and left the connection DISCONNECTED or SUSPENDED
// (RTN14b, RTN15h2), as from any other DISCONNECTED or SUSPENDED state
// (RTN14d, RTN14f), unless Close is called first.
func (c *Connection) retryAfterReauthorizationFailed(arg connArgs) {
	timerCtx, cancelTimer := c.ctxCancelOnStateTransition()
	switch c.State() {
	case ConnectionStateDisconnected:
		<-c.opts.After(timerCtx, c.opts.disconnectedRetryTimeout())
	case ConnectionStateSuspended:
		<-c.opts.After(timerCtx, c.opts.suspendedRetryTimeout())
	}
	cancelTimer()

	c.mtx.Lock()
	// Connect from DISCONNECTED or SUSPENDED moves to CONNECTING expecting a
	// retry loop to be running, so reconnect from there too.
	stale := c.superseded(arg.gen)
	retry := !stale &&
		(c.state == ConnectionStateDisconnected || c.state == ConnectionStateSuspended ||
			c.state == ConnectionStateConnecting)
	if !stale {
		c.lockFinishClose()
	}
	c.mtx.Unlock()
	if retry {
		arg.dialOnce = false
		c.reconnect(arg)
	}
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

	if msg.Error != nil {
		vc.logger.Errorf("Realtime Connection: error received %s", msg)
	} else {
		vc.logger.Verbosef("Realtime Connection: received %s", msg)
	}
	return msg, nil
}

func (vc verboseConn) Close() error {
	vc.logger.Verbosef("Realtime Connection: closed")
	return vc.conn.Close()
}

func (vc verboseConn) Unwrap() conn {
	return vc.conn
}

func (c *Connection) setState(state ConnectionState, err error, retryIn time.Duration) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.lockSetState(state, err, retryIn)
}

// interruptionCount returns the number of times the connection has entered
// SUSPENDED, CLOSED or FAILED.
func (c *Connection) interruptionCount() uint32 {
	return atomic.LoadUint32(&c.interruptions)
}

// latestInterruption returns the interruption count and the reason for the
// latest interruption.
func (c *Connection) latestInterruption() (uint32, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.interruptionCount(), c.interruptionErr
}

// stateAndInterruptions returns the connection's state and interruption
// count together, for deciding to queue a message (see msgQueue.Enqueue).
func (c *Connection) stateAndInterruptions() (ConnectionState, uint32) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.state, c.interruptionCount()
}

// lockFailPending counts an interruption and fails the messages awaiting an
// ACK, which won't be acknowledged now (RTN7e). Their callbacks are called by
// the state change handler, which also fails queued messages, using the
// interruption count to keep messages queued since a reopen.
func (c *Connection) lockFailPending(err error) {
	atomic.AddUint32(&c.interruptions, 1)
	c.interruptionErr = err
	for _, m := range c.pending.Dismiss() {
		m := m
		c.failedAcks = append(c.failedAcks, func() { m.ackCallback.call(nil, err) })
	}
}

// callFailedAcks calls the callbacks of messages that lockFailPending failed.
func (c *Connection) callFailedAcks() {
	c.mtx.Lock()
	calls := c.failedAcks
	c.failedAcks = nil
	c.mtx.Unlock()
	for _, call := range calls {
		call()
	}
}

func (c *Connection) lockSetState(state ConnectionState, err error, retryIn time.Duration) error {
	if (c.state == ConnectionStateClosing || c.state == ConnectionStateClosed) &&
		(state == ConnectionStateDisconnected || state == ConnectionStateSuspended) {
		// After Close, a failure must not move the connection back into a
		// state that retries (RTN12f), so finish the close instead. FAILED is
		// allowed, as fatal errors still fail it (RTN14g, RTN15h1). This is a
		// backstop: failed connection attempts use setStateUnlessClosing.
		if c.state == ConnectionStateClosed {
			return nil
		}
		state, err, retryIn = ConnectionStateClosed, nil, 0
	}
	if state == ConnectionStateClosing || state == ConnectionStateClosed ||
		state == ConnectionStateSuspended || state == ConnectionStateFailed {
		c.key, c.id = "", "" //(RTN8c, RTN9c)
	}

	previous := c.state
	changed := c.state != state
	c.state = state
	switch state {
	case ConnectionStateDisconnected:
		if c.disconnectedAt.IsZero() {
			c.disconnectedAt = c.opts.Now()
		}
	case ConnectionStateConnecting, ConnectionStateSuspended:
	default:
		c.disconnectedAt = time.Time{}
	}
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
	if isInterruption(change) {
		c.lockFailPending(interruptionError(change))
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
