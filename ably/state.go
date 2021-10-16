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

// queuedEmitter emits confirmation events triggered by ACK or NACK messages.
type pendingEmitter struct {
	queue []msgCh
	log   logger
}

func newPendingEmitter(log logger) pendingEmitter {
	return pendingEmitter{
		log: log,
	}
}

type msgCh struct {
	msg *protocolMessage
	ch  chan<- error
}

// Dismiss lets go of the channels that are waiting for an error on this queue.
// The queue can continue sending messages.
func (q *pendingEmitter) Dismiss() []msgCh {
	cx := make([]msgCh, len(q.queue))
	copy(cx, q.queue)
	q.queue = nil
	return cx
}

func (q *pendingEmitter) Enqueue(msg *protocolMessage, ch chan<- error) {
	if len(q.queue) > 0 {
		expected := q.queue[len(q.queue)-1].msg.MsgSerial + 1
		if got := msg.MsgSerial; expected != got {
			panic(fmt.Sprintf("protocol violation: expected next enqueued message to have msgSerial %d; got %d", expected, got))
		}
	}
	q.queue = append(q.queue, msgCh{msg, ch})
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
		sch.ch <- err
	}
}

type msgch struct {
	msg *protocolMessage
	ch  chan<- error
}

type msgQueue struct {
	mtx   sync.Mutex
	queue []msgch
	conn  *Connection
}

func newMsgQueue(conn *Connection) *msgQueue {
	return &msgQueue{
		conn: conn,
	}
}

func (q *msgQueue) Enqueue(msg *protocolMessage, listen chan<- error) {
	q.mtx.Lock()
	// TODO(rjeczalik): reorder the queue so Presence / Messages can be merged
	q.queue = append(q.queue, msgch{msg, listen})
	q.mtx.Unlock()
}

func (q *msgQueue) Flush() {
	q.mtx.Lock()
	for _, msgch := range q.queue {
		q.conn.send(msgch.msg, msgch.ch)
	}
	q.queue = nil
	q.mtx.Unlock()
}

func (q *msgQueue) Fail(err error) {
	q.mtx.Lock()
	for _, msgch := range q.queue {
		q.log().Errorf("failure sending message (serial=%d): %v", msgch.msg.MsgSerial, err)
		msgch.ch <- newError(90000, err)
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

// A ConnectionState identifies the state of an Ably realtime connection.
type ConnectionState struct {
	name string
}

var (
	ConnectionStateInitialized  ConnectionState = ConnectionState{name: "INITIALIZED"}
	ConnectionStateConnecting   ConnectionState = ConnectionState{name: "CONNECTING"}
	ConnectionStateConnected    ConnectionState = ConnectionState{name: "CONNECTED"}
	ConnectionStateDisconnected ConnectionState = ConnectionState{name: "DISCONNECTED"}
	ConnectionStateSuspended    ConnectionState = ConnectionState{name: "SUSPENDED"}
	ConnectionStateClosing      ConnectionState = ConnectionState{name: "CLOSING"}
	ConnectionStateClosed       ConnectionState = ConnectionState{name: "CLOSED"}
	ConnectionStateFailed       ConnectionState = ConnectionState{name: "FAILED"}
)

func (e ConnectionState) String() string {
	return e.name
}

// A ConnectionEvent identifies an event in the lifetime of an Ably realtime
// connection.
type ConnectionEvent struct {
	name string
}

func (ConnectionEvent) isEmitterEvent() {}

var (
	ConnectionEventInitialized  ConnectionEvent = ConnectionEvent(ConnectionStateInitialized)
	ConnectionEventConnecting   ConnectionEvent = ConnectionEvent(ConnectionStateConnecting)
	ConnectionEventConnected    ConnectionEvent = ConnectionEvent(ConnectionStateConnected)
	ConnectionEventDisconnected ConnectionEvent = ConnectionEvent(ConnectionStateDisconnected)
	ConnectionEventSuspended    ConnectionEvent = ConnectionEvent(ConnectionStateSuspended)
	ConnectionEventClosing      ConnectionEvent = ConnectionEvent(ConnectionStateClosing)
	ConnectionEventClosed       ConnectionEvent = ConnectionEvent(ConnectionStateClosed)
	ConnectionEventFailed       ConnectionEvent = ConnectionEvent(ConnectionStateFailed)
	ConnectionEventUpdate       ConnectionEvent = ConnectionEvent{name: "UPDATE"}
)

func (e ConnectionEvent) String() string {
	return e.name
}

// A ConnectionStateChange is the data associated with a ConnectionEvent.
//
// If the Event is a ConnectionEventUpdated, Current and Previous are the
// the same. Otherwise, the event is a state transition from Previous to
// Current.
type ConnectionStateChange struct {
	Current  ConnectionState
	Event    ConnectionEvent
	Previous ConnectionState
	RetryIn  time.Duration //RTN14d, TA2
	// Reason, if any, is an error that caused the state change.
	Reason *ErrorInfo
}

func (ConnectionStateChange) isEmitterData() {}

// A ChannelState identifies the state of an Ably realtime channel.
type ChannelState struct {
	name string
}

var (
	ChannelStateInitialized ChannelState = ChannelState{name: "INITIALIZED"}
	ChannelStateAttaching   ChannelState = ChannelState{name: "ATTACHING"}
	ChannelStateAttached    ChannelState = ChannelState{name: "ATTACHED"}
	ChannelStateDetaching   ChannelState = ChannelState{name: "DETACHING"}
	ChannelStateDetached    ChannelState = ChannelState{name: "DETACHED"}
	ChannelStateSuspended   ChannelState = ChannelState{name: "SUSPENDED"}
	ChannelStateFailed      ChannelState = ChannelState{name: "FAILED"}
)

func (e ChannelState) String() string {
	return e.name
}

// A ChannelEvent identifies an event in the lifetime of an Ably realtime
// channel.
type ChannelEvent struct {
	name string
}

func (ChannelEvent) isEmitterEvent() {}

var (
	ChannelEventInitialized ChannelEvent = ChannelEvent(ChannelStateInitialized)
	ChannelEventAttaching   ChannelEvent = ChannelEvent(ChannelStateAttaching)
	ChannelEventAttached    ChannelEvent = ChannelEvent(ChannelStateAttached)
	ChannelEventDetaching   ChannelEvent = ChannelEvent(ChannelStateDetaching)
	ChannelEventDetached    ChannelEvent = ChannelEvent(ChannelStateDetached)
	ChannelEventSuspended   ChannelEvent = ChannelEvent(ChannelStateSuspended)
	ChannelEventFailed      ChannelEvent = ChannelEvent(ChannelStateFailed)
	ChannelEventUpdate      ChannelEvent = ChannelEvent{name: "UPDATE"}
)

func (e ChannelEvent) String() string {
	return e.name
}

// A ChannelStateChange is the data associated with a ChannelEvent.
//
// If the Event is a ChannelEventUpdated, Current and Previous are the
// the same. Otherwise, the event is a state transition from Previous to
// Current.
type ChannelStateChange struct {
	Current  ChannelState
	Event    ChannelEvent
	Previous ChannelState
	// Reason, if any, is an error that caused the state change.
	Reason *ErrorInfo
	// Resumed is set to true for Attached and Update events when channel state
	// has been maintained without interruption in the server, so there has
	// been no loss of message continuity.
	Resumed bool
}

func (ChannelStateChange) isEmitterData() {}
