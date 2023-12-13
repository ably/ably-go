package ably

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// result awaits completion of asynchronous operation.
type result interface {
	// Wait blocks until asynchronous operation is completed. Upon its completion,
	// the method returns nil error if it was successful and non-nil error otherwise.
	// It's allowed to call Wait multiple times.
	Wait(context.Context) error
}

func wait(ctx context.Context) func(result, error) error {
	return func(res result, err error) error {
		if err != nil {
			return err
		}
		return res.Wait(ctx)
	}
}

// goWaiter immediately calls the given function in a separate goroutine. The
// returned Result waits for its completion and returns its error.
func goWaiter(f func() error) result {
	err := make(chan error, 1)
	go func() {
		defer close(err)
		err <- f()
	}()
	return resultFunc(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-err:
			return err
		}
	})
}

var (
	errDisconnected   = newErrorf(ErrDisconnected, "Connection temporarily unavailable")
	errSuspended      = newErrorf(ErrConnectionSuspended, "Connection unavailable")
	errClosed         = newErrorf(ErrConnectionClosed, "Connection unavailable")
	errFailed         = newErrorf(ErrConnectionFailed, "Connection failed")
	errNeverConnected = newErrorf(ErrConnectionSuspended, "Unable to establish connection")

	errNACKWithoutError = newErrorf(ErrInternalError, "NACK without error")
	errImplictNACK      = newErrorf(ErrInternalError, "implicit NACK")
)

var connStateErrors = map[ConnectionState]ErrorInfo{
	ConnectionStateInitialized:  *errNeverConnected,
	ConnectionStateDisconnected: *errDisconnected,
	ConnectionStateFailed:       *errFailed,
	ConnectionStateSuspended:    *errSuspended,
}

func connStateError(state ConnectionState, err error) *ErrorInfo {
	// Set default error information associated with the target state.
	e, ok := err.(*ErrorInfo)
	if ok && e != nil {
		return e
	}
	if e, ok := connStateErrors[state]; ok {
		if err != nil {
			e.err = err
		}
		err = &e
	}
	if err == nil {
		return nil
	}
	return newError(0, err)
}

var (
	errChannelFailed = newErrorf(ErrChannelOperationFailed, "Channel state is failed")
)

var channelStateErrors = map[ChannelState]ErrorInfo{
	ChannelStateFailed: *errChannelFailed,
}

func channelStateError(state ChannelState, err error) *ErrorInfo {
	// Set default error information associated with the target state.
	e, ok := err.(*ErrorInfo)
	if ok && e != nil {
		return e
	}
	if e, ok := channelStateErrors[state]; ok {
		if err != nil {
			e.err = err
		}
		err = &e
	}
	if err == nil {
		return nil
	}
	return newError(0, err)
}

// pendingEmitter emits confirmation events triggered by ACK or NACK messages.
type pendingEmitter struct {
	queue []msgWithAckCallback
	log   logger
}

func newPendingEmitter(log logger) pendingEmitter {
	return pendingEmitter{
		log: log,
	}
}

// Dismiss lets go of the channels that are waiting for an error on this queue.
// The queue can continue sending messages.
func (q *pendingEmitter) Dismiss() []msgWithAckCallback {
	cx := make([]msgWithAckCallback, len(q.queue))
	copy(cx, q.queue)
	q.queue = nil
	return cx
}

func (q *pendingEmitter) Enqueue(msg *protocolMessage, onAck func(err error)) {
	if len(q.queue) > 0 {
		expected := q.queue[len(q.queue)-1].msg.MsgSerial + 1
		if got := msg.MsgSerial; expected != got {
			panic(fmt.Sprintf("protocol violation: expected next enqueued message to have msgSerial %d; got %d", expected, got))
		}
	}
	q.queue = append(q.queue, msgWithAckCallback{msg, onAck})
}

func (q *pendingEmitter) Ack(msg *protocolMessage, errInfo *ErrorInfo) {
	// The msgSerial from the server may not be the same we're waiting. If the
	// server skipped some messages, they get implicitly NACKed. If the server
	// ACKed some messages again, we ignore those. In both cases, we just need
	// to correct the number of messages that get ACKed by that difference.
	queueLength := len(q.queue)

	// We've seen a panic on test runs which is likely from q.queue[0] below:
	// panic: runtime error: index out of range [0] with length 0
	// So this check is here to attempt to identify if that is the case so we can work on a fix.
	// This is also why messageChannel is discretely declared in case we see a failure despite this check.
	if queueLength < 1 {
		panic("Ack called but queue is empty.")
	}
	messageChannel := q.queue[0]

	serialShift := int(msg.MsgSerial - messageChannel.msg.MsgSerial)
	count := msg.Count + serialShift
	if count > queueLength {
		panic(fmt.Sprintf("protocol violation: ACKed %d messages, but only %d pending", count, len(q.queue)))
	} else if count < 1 {
		// We have encountered negative counts during load testing, and
		// don't currently have a good explanation for them.
		//
		// Whilst the behaviour needs to be understood, it does not
		// have any user facing impact since the ACK for earlier
		// messages can safely be ignored, so we just emit a debug
		// log to aid in further investigation.
		q.log.Debugf("protocol violation: received ACK for %d messages from serial %d, but current message serial is %d", msg.Count, msg.MsgSerial, messageChannel.msg.MsgSerial)
		return
	}
	acked := q.queue[:count]
	q.queue = q.queue[count:]

	err := errInfo.unwrapNil()
	if msg.Action == actionNack && err == nil {
		err = errNACKWithoutError
	}

	for i, sch := range acked {
		err := err
		if i < serialShift {
			err = errImplictNACK
		}
		q.log.Verbosef("received %v for message serial %d", msg.Action, sch.msg.MsgSerial)
		if sch.onAck != nil {
			sch.onAck(err)
		}
	}
}

type msgWithAckCallback struct {
	msg   *protocolMessage
	onAck func(err error)
}

type msgQueue struct {
	mtx   sync.Mutex
	queue []msgWithAckCallback
	conn  *Connection
}

func newMsgQueue(conn *Connection) *msgQueue {
	return &msgQueue{
		conn: conn,
	}
}

func (q *msgQueue) Enqueue(msg *protocolMessage, onAck func(err error)) {
	q.mtx.Lock()
	// TODO(rjeczalik): reorder the queue so Presence / Messages can be merged
	q.queue = append(q.queue, msgWithAckCallback{msg, onAck})
	q.mtx.Unlock()
}

func (q *msgQueue) Flush() {
	q.mtx.Lock()
	for _, queueMsg := range q.queue {
		q.conn.send(queueMsg.msg, queueMsg.onAck)
	}
	q.queue = nil
	q.mtx.Unlock()
}

func (q *msgQueue) Fail(err error) {
	q.mtx.Lock()
	for _, queueMsg := range q.queue {
		q.log().Errorf("failure sending message (serial=%d): %v", queueMsg.msg.MsgSerial, err)
		if queueMsg.onAck != nil {
			queueMsg.onAck(newError(90000, err))
		}
	}
	q.queue = nil
	q.mtx.Unlock()
}

func (q *msgQueue) log() logger {
	return q.conn.log()
}

var nopResult *errResult

type errResult struct {
	err    error
	listen <-chan error
}

func newErrResult() (result, chan<- error) {
	listen := make(chan error, 1)
	res := &errResult{listen: listen}
	return res, listen
}

// Wait implements the Result interface.
func (res *errResult) Wait(ctx context.Context) error {
	if res == nil {
		return nil
	}
	if l := res.listen; l != nil {
		res.listen = nil
		select {
		case res.err = <-l:
		case <-ctx.Done():
			res.err = ctx.Err()
		}
	}
	return res.err
}

type resultFunc func(context.Context) error

func (f resultFunc) Wait(ctx context.Context) error {
	return f(ctx)
}

func (e ChannelEventEmitter) listenResult(expected ChannelState, failed ...ChannelState) result {
	// Make enough room not to block the sender if the Result is never waited on.
	changes := make(channelStateChanges, 1+len(failed))

	var offs []func()
	offs = append(offs, e.Once(ChannelEvent(expected), changes.Receive))
	for _, ev := range failed {
		offs = append(offs, e.Once(ChannelEvent(ev), changes.Receive))
	}

	return resultFunc(func(ctx context.Context) error {
		defer func() {
			for _, off := range offs {
				off()
			}
		}()

		var change ChannelStateChange
		select {
		case <-ctx.Done():
			return ctx.Err()
		case change = <-changes:
		}

		switch {
		case change.Current == expected:
		case change.Reason != nil:
			return change.Reason
		default:
			code := ErrInternalChannelError
			return newError(code, fmt.Errorf("failed channel change: %s", change.Current))
		}

		return nil
	})
}

func (e ConnectionEventEmitter) listenResult(expected ConnectionState, failed ...ConnectionState) result {
	// Make enough room not to block the sender if the Result is never waited on.
	changes := make(connStateChanges, 1+len(failed))

	var offs []func()
	offs = append(offs, e.Once(ConnectionEvent(expected), changes.Receive))
	for _, ev := range failed {
		offs = append(offs, e.Once(ConnectionEvent(ev), changes.Receive))
	}

	return resultFunc(func(ctx context.Context) error {
		defer func() {
			for _, off := range offs {
				off()
			}
		}()

		var change ConnectionStateChange
		select {
		case <-ctx.Done():
			return ctx.Err()
		case change = <-changes:
		}

		switch {
		case change.Current == expected:
		case change.Reason != nil:
			return change.Reason
		default:
			code := ErrInternalConnectionError
			return newError(code, fmt.Errorf("failed connection change: %s", change.Current))
		}

		return nil
	})
}

// ConnectionState describes the realtime [ably.Connection] object states.
type ConnectionState struct {
	name string
}

var (
	// ConnectionStateInitialized - A connection with this state has been initialized
	// but no connection has yet been attempted.
	ConnectionStateInitialized ConnectionState = ConnectionState{name: "INITIALIZED"}

	// ConnectionStateConnecting - A connection attempt has been initiated.
	// The connecting state is entered as soon as the library has completed initialization,
	// and is reentered each time connection is re-attempted following disconnection.
	ConnectionStateConnecting ConnectionState = ConnectionState{name: "CONNECTING"}

	// ConnectionStateConnected - A connection exists and is active.
	ConnectionStateConnected ConnectionState = ConnectionState{name: "CONNECTED"}

	// ConnectionStateDisconnected - A temporary failure condition. No current connection exists
	// because there is no network connectivity or no host is available.
	// The disconnected state is entered if an established connection is dropped,
	// or if a connection attempt was unsuccessful. In the disconnected state the library will periodically attempt
	// to open a new connection (approximately every 15 seconds), anticipating that the connection will
	// be re-established soon and thus connection and channel continuity will be possible.
	// In this state, developers can continue to publish messages as they are automatically placed in a local queue,
	// to be sent as soon as a connection is reestablished. Messages published by other clients
	// while this client is disconnected will be delivered to it upon reconnection, so long as
	// the connection was resumed within 2 minutes. After 2 minutes have elapsed, recovery is no longer possible
	// and the connection will move to the SUSPENDED state.
	ConnectionStateDisconnected ConnectionState = ConnectionState{name: "DISCONNECTED"}

	// ConnectionStateSuspended - A long term failure condition. No current connection exists
	// because there is no network connectivity or no host is available. The suspended state is entered
	// after a failed connection attempt if there has then been no connection for a period of two minutes.
	// In the suspended state, the library will periodically attempt to open a new connection every 30 seconds.
	// Developers are unable to publish messages in this state. A new connection attempt can also be triggered
	// by an explicit call to Connection.Connect. Once the connection has been re-established,
	// channels will be automatically re-attached. The client has been disconnected for too long for them
	// to resume from where they left off, so if it wants to catch up on messages published by other clients
	// while it was disconnected, it needs to use the History API.
	ConnectionStateSuspended ConnectionState = ConnectionState{name: "SUSPENDED"}

	// ConnectionStateClosing is when an explicit request by the developer to close the connection has been sent to
	// the Ably service. If a reply is not received from Ably within a short period of time,
	// the connection is forcibly terminated and the connection state becomes CLOSED.
	ConnectionStateClosing ConnectionState = ConnectionState{name: "CLOSING"}

	// ConnectionStateClosed - The connection has been explicitly closed by the client.
	// In the closed state, no reconnection attempts are made automatically by the library,
	// and clients may not publish messages. No connection state is preserved by the service or by the library.
	// A new connection attempt can be triggered by an explicit call to Connection.Connect,
	// which results in a new connection.
	ConnectionStateClosed ConnectionState = ConnectionState{name: "CLOSED"}

	// ConnectionStateFailed - This state is entered if the client library encounters a failure condition
	// that it cannot recover from. This may be a fatal connection error received from the Ably service,
	// for example an attempt to connect with an incorrect API key, or a local terminal error,
	// for example the token in use has expired and the library does not have any way to renew it.
	// In the failed state, no reconnection attempts are made automatically by the library, and
	// clients may not publish messages. A new connection attempt can be triggered by an explicit call
	// to Connection.Connect.
	ConnectionStateFailed ConnectionState = ConnectionState{name: "FAILED"}
)

func (e ConnectionState) String() string {
	return e.name
}

// ConnectionEvent describes the events emitted by a [ably.Connection] object.
// An event is either an [ably.ConnectionEventUpdate] or [ably.ConnectionState} change event.
type ConnectionEvent struct {
	name string
}

func (ConnectionEvent) isEmitterEvent() {}

var (
	// ConnectionEventInitialized - A connection with this state has been initialized
	// but no connection has yet been attempted.
	ConnectionEventInitialized ConnectionEvent = ConnectionEvent(ConnectionStateInitialized)

	// ConnectionEventConnecting - A connection attempt has been initiated.
	// The connecting state is entered as soon as the library has completed initialization,
	// and is reentered each time connection is re-attempted following disconnection.
	ConnectionEventConnecting ConnectionEvent = ConnectionEvent(ConnectionStateConnecting)

	// ConnectionEventConnected - A connection exists and is active.
	ConnectionEventConnected ConnectionEvent = ConnectionEvent(ConnectionStateConnected)

	// ConnectionEventDisconnected - A temporary failure condition. No current connection exists
	// because there is no network connectivity or no host is available.
	// The disconnected state is entered if an established connection is dropped,
	// or if a connection attempt was unsuccessful. In the disconnected state the library will periodically attempt
	// to open a new connection (approximately every 15 seconds), anticipating that the connection will
	// be re-established soon and thus connection and channel continuity will be possible.
	// In this state, developers can continue to publish messages as they are automatically placed in a local queue,
	// to be sent as soon as a connection is reestablished. Messages published by other clients
	// while this client is disconnected will be delivered to it upon reconnection, so long as
	// the connection was resumed within 2 minutes. After 2 minutes have elapsed, recovery is no longer possible
	// and the connection will move to the SUSPENDED state.
	ConnectionEventDisconnected ConnectionEvent = ConnectionEvent(ConnectionStateDisconnected)

	// ConnectionEventSuspended - A long term failure condition. No current connection exists
	// because there is no network connectivity or no host is available. The suspended state is entered
	// after a failed connection attempt if there has then been no connection for a period of two minutes.
	// In the suspended state, the library will periodically attempt to open a new connection every 30 seconds.
	// Developers are unable to publish messages in this state. A new connection attempt can also be triggered
	// by an explicit call to Connection.Connect. Once the connection has been re-established,
	// channels will be automatically re-attached. The client has been disconnected for too long for them
	// to resume from where they left off, so if it wants to catch up on messages published by other clients
	// while it was disconnected, it needs to use the History API.
	ConnectionEventSuspended ConnectionEvent = ConnectionEvent(ConnectionStateSuspended)

	// ConnectionEventClosing is when an explicit request by the developer to close the connection has been sent to
	// the Ably service. If a reply is not received from Ably within a short period of time,
	// the connection is forcibly terminated and the connection state becomes CLOSED.
	ConnectionEventClosing ConnectionEvent = ConnectionEvent(ConnectionStateClosing)

	// ConnectionEventClosed - The connection has been explicitly closed by the client.
	// In the closed state, no reconnection attempts are made automatically by the library,
	// and clients may not publish messages. No connection state is preserved by the service or by the library.
	// A new connection attempt can be triggered by an explicit call to Connection.Connect,
	// which results in a new connection.
	ConnectionEventClosed ConnectionEvent = ConnectionEvent(ConnectionStateClosed)

	// ConnectionEventFailed - This state is entered if the client library encounters a failure condition
	// that it cannot recover from. This may be a fatal connection error received from the Ably service,
	// for example an attempt to connect with an incorrect API key, or a local terminal error,
	// for example the token in use has expired and the library does not have any way to renew it.
	// In the failed state, no reconnection attempts are made automatically by the library, and
	// clients may not publish messages. A new connection attempt can be triggered by an explicit call
	// to Connection.Connect.
	ConnectionEventFailed ConnectionEvent = ConnectionEvent(ConnectionStateFailed)

	// ConnectionEventUpdate is an event for changes to connection conditions for which the
	// [ably.ConnectionState] does not change (RTN4h).
	ConnectionEventUpdate ConnectionEvent = ConnectionEvent{name: "UPDATE"}
)

func (e ConnectionEvent) String() string {
	return e.name
}

// ConnectionStateChange Contains [ably.ConnectionState]change information emitted by the [ably.Connection] object.
// A ConnectionStateChange is the data associated with a ConnectionEvent.
type ConnectionStateChange struct {
	// Current is the new [ably.ConnectionState] (TA2).
	Current ConnectionState

	// Event is the event that triggered this [ably.ConnectionState] change (TA5).
	Event ConnectionEvent

	// Previous is the previous [ably.ConnectionState]. For the [ConnectionEventUpdate] event,
	// this is equal to the current [ably.ConnectionState] (TA2).
	Previous ConnectionState

	// RetryIn is duration in milliseconds,
	// after which the client retries a connection where applicable (RTN14d, TA2).
	RetryIn time.Duration

	// Reason is an [ably.ErrorInfo] object that caused the state change (RTN4f, TA3).
	Reason *ErrorInfo
}

func (ConnectionStateChange) isEmitterData() {}

// ChannelState describes the possible states of a [ably.RESTChannel] or [ably.RealtimeChannel] object.
type ChannelState struct {
	name string
}

var (
	// ChannelStateInitialized is when the channel has been initialized but no attach has yet been attempted.
	ChannelStateInitialized ChannelState = ChannelState{name: "INITIALIZED"}

	// ChannelStateAttaching is when the attach has been initiated by sending a request to Ably.
	// This is a transient state, followed either by a transition to ATTACHED, SUSPENDED, or FAILED.
	ChannelStateAttaching ChannelState = ChannelState{name: "ATTACHING"}

	// ChannelStateAttached is when the attach has succeeded.
	// In the ATTACHED state a client may publish and subscribe to messages, or be present on the channel.
	ChannelStateAttached ChannelState = ChannelState{name: "ATTACHED"}

	// ChannelStateDetaching is when a detach has been initiated on an ATTACHED channel by sending a request to Ably.
	// This is a transient state, followed either by a transition to DETACHED or FAILED.
	ChannelStateDetaching ChannelState = ChannelState{name: "DETACHING"}

	// ChannelStateDetached os when the channel, having previously been ATTACHED, has been detached by the user.
	ChannelStateDetached ChannelState = ChannelState{name: "DETACHED"}

	// ChannelStateSuspended is when the channel, having previously been ATTACHED, has lost continuity,
	// usually due to the client being disconnected from Ably for longer than two minutes.
	// It will automatically attempt to reattach as soon as connectivity is restored.
	ChannelStateSuspended ChannelState = ChannelState{name: "SUSPENDED"}

	// ChannelStateFailed is an indefinite failure condition.
	// This state is entered if a channel error has been received from the Ably service,
	// such as an attempt to attach without the necessary access rights.
	ChannelStateFailed ChannelState = ChannelState{name: "FAILED"}
)

func (e ChannelState) String() string {
	return e.name
}

// ChannelEvent describes the events emitted by a [ably.RESTChannel] or [ably.RealtimeChannel] object.
// An event is either an UPDATE or a [ably.ChannelState].
type ChannelEvent struct {
	name string
}

func (ChannelEvent) isEmitterEvent() {}

var (

	// ChannelEventInitialized is when the channel has been initialized but no attach has yet been attempted.
	ChannelEventInitialized ChannelEvent = ChannelEvent(ChannelStateInitialized)

	// ChannelEventAttaching is when the attach has been initiated by sending a request to Ably.
	// This is a transient state, followed either by a transition to ATTACHED, SUSPENDED, or FAILED.
	ChannelEventAttaching ChannelEvent = ChannelEvent(ChannelStateAttaching)

	// ChannelEventAttached is when the attach has succeeded.
	// In the ATTACHED state a client may publish and subscribe to messages, or be present on the channel.
	ChannelEventAttached ChannelEvent = ChannelEvent(ChannelStateAttached)

	// ChannelEventDetaching is when a detach has been initiated on an ATTACHED channel by sending a request to Ably.
	// This is a transient state, followed either by a transition to DETACHED or FAILED.
	ChannelEventDetaching ChannelEvent = ChannelEvent(ChannelStateDetaching)

	// ChannelEventDetached os when the channel, having previously been ATTACHED, has been detached by the user.
	ChannelEventDetached ChannelEvent = ChannelEvent(ChannelStateDetached)

	// ChannelEventSuspended is when the channel, having previously been ATTACHED, has lost continuity,
	// usually due to the client being disconnected from Ably for longer than two minutes.
	// It will automatically attempt to reattach as soon as connectivity is restored.
	ChannelEventSuspended ChannelEvent = ChannelEvent(ChannelStateSuspended)

	// ChannelEventFailed is an indefinite failure condition.
	// This state is entered if a channel error has been received from the Ably service,
	// such as an attempt to attach without the necessary access rights.
	ChannelEventFailed ChannelEvent = ChannelEvent(ChannelStateFailed)

	// ChannelEventUpdate is an event for changes to channel conditions that do not result
	// in a change in [ably.ChannelState] (RTL2g).
	ChannelEventUpdate ChannelEvent = ChannelEvent{name: "UPDATE"}
)

func (e ChannelEvent) String() string {
	return e.name
}

// ChannelStateChange contains state change information emitted by [ably.RESTChannel] and [ably.RealtimeChannel] objects.
// A ChannelStateChange is the data associated with a ChannelEvent.
type ChannelStateChange struct {

	// Current is the new current [ably.ChannelState] (RTL2a, RTL2b).
	Current ChannelState

	// Event is the event that triggered this [ably.ChannelState] change (TH5).
	Event ChannelEvent

	// Previous is the previous state.
	// For the [ably.ChannelEventUpdate] event, this is equal to the current [ably.ChannelState] (RTL2a, RTL2b).
	Previous ChannelState

	// Reason is an [ErrorInfo] object that caused the state change (RTL2e, TH3).
	Reason *ErrorInfo

	// Resumed is set to true for Attached and Update events when channel state has been maintained
	// without interruption in the server, so there has been no loss of message continuity.
	// Indicates whether message continuity on this channel is preserved, see Nonfatal channel errors for more info.
	// (RTL2f, TH4)
	Resumed bool
}

func (ChannelStateChange) isEmitterData() {}
