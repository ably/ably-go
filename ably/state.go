package ably

import (
	"fmt"
	"sort"
	"sync"

	"github.com/ably/ably-go/ably/proto"
)

// StateType specifies which group of states is relevant in given context;
// either:
//
//   - StateConn* group describing Conn states
//   - StateChan* group describing RealtimeChannel states
//
type StateType int

const (
	StateConn StateType = 1 + iota
	StateChan
)

// Strings implements the fmt.Stringer interface.
func (st StateType) String() string {
	switch st {
	case StateConn:
		return "connection"
	case StateChan:
		return "channel"
	default:
		return "invalid"
	}
}

// Contains returns true when the state belongs to the given type.
func (st StateType) Contains(state StateEnum) bool {
	return stateMasks[st]&state == state
}

// StateEnum is an enumeration type for connection and channel states.
type StateEnum int

// String implements the fmt.Stringer interface.
func (sc StateEnum) String() string {
	if s, ok := stateText[sc]; ok {
		return s
	}
	return "invalid"
}

// StateConn describes states of realtime connection.
const (
	StateConnInitialized StateEnum = 1 << iota
	StateConnConnecting
	StateConnConnected
	StateConnDisconnected
	StateConnSuspended
	StateConnClosing
	StateConnClosed
	StateConnFailed
)

// StateChan describes states of realtime channel.
const (
	StateChanInitialized StateEnum = 1 << (iota + 8)
	StateChanAttaching
	StateChanAttached
	StateChanDetaching
	StateChanDetached
	StateChanClosing
	StateChanClosed
	StateChanFailed
)

// Result awaits completion of asynchronous operation.
type Result interface {
	// Wait blocks until asynchronous operation is completed. Upon its completion,
	// the method returns nil error if it was successful and non-nil error otherwise.
	// It's allowed to call Wait multiple times.
	Wait() error
}

func wait(res Result, err error) error {
	if err != nil {
		return err
	}
	return res.Wait()
}

var stateText = map[StateEnum]string{
	StateConnInitialized:  "ably.StateConnInitialized",
	StateConnConnecting:   "ably.StateConnConnecting",
	StateConnConnected:    "ably.StateConnConnected",
	StateConnDisconnected: "ably.StateConnDisconnected",
	StateConnSuspended:    "ably.StateConnSuspended",
	StateConnClosing:      "ably.StateConnClosing",
	StateConnClosed:       "ably.StateConnClosed",
	StateConnFailed:       "ably.StateConnFailed",
	StateChanInitialized:  "ably.StateChanInitialized",
	StateChanAttaching:    "ably.StateChanAttaching",
	StateChanAttached:     "ably.StateChanAttached",
	StateChanDetaching:    "ably.StateChanDetaching",
	StateChanDetached:     "ably.StateChanDetached",
	StateChanClosing:      "ably.StateChanClosing",
	StateChanClosed:       "ably.StateChanClosed",
	StateChanFailed:       "ably.StateChanFailed",
}

// stateAll lists all valid connection and channel state values.
var stateAll = map[StateType][]StateEnum{
	StateConn: {
		StateConnInitialized,
		StateConnConnecting,
		StateConnConnected,
		StateConnDisconnected,
		StateConnSuspended,
		StateConnClosing,
		StateConnClosed,
		StateConnFailed,
	},
	StateChan: {
		StateChanInitialized,
		StateChanAttaching,
		StateChanAttached,
		StateChanDetaching,
		StateChanClosed,
		StateChanDetached,
		StateChanFailed,
	},
}

// stateMasks is used for testing connection and channel state values.
var stateMasks = map[StateType]StateEnum{
	StateConn: StateConnInitialized | StateConnConnecting | StateConnConnected |
		StateConnDisconnected | StateConnSuspended | StateConnClosing | StateConnClosed |
		StateConnFailed,
	StateChan: StateChanInitialized | StateChanAttaching | StateChanAttached |
		StateChanDetaching | StateChanDetached | StateChanClosing | StateChanClosed |
		StateChanFailed,
}

var (
	errDisconnected   = newErrorf(80003, "Connection temporarily unavailable")
	errSuspended      = newErrorf(80002, "Connection unavailable")
	errFailed         = newErrorf(80000, "Connection failed")
	errNeverConnected = newErrorf(80002, "Unable to establish connection")
)

var stateErrors = map[StateEnum]ErrorInfo{
	StateConnInitialized:  *errNeverConnected,
	StateConnDisconnected: *errDisconnected,
	StateConnFailed:       *errFailed,
	StateConnSuspended:    *errSuspended,
	StateChanFailed:       *errFailed,
}

func stateError(state StateEnum, err error) *ErrorInfo {
	// Set default error information associated with the target state.
	e, ok := err.(*ErrorInfo)
	if ok {
		return e
	}
	if e, ok := stateErrors[state]; ok {
		if err != nil {
			e.err = err
		}
		err = &e
	}
	return newError(0, err)
}

// State describes a single state transition of either realtime connection or channel
// that occurred due to some external condition (dropped connection, retried etc.).
//
// Each realtime connection and channel maintains its state to ensure high availability
// and resilience, which is inherently asynchronous. In order to listen to transition
// between states for both realtime connection and realtime channel user may provide
// a channel, which will get notified with single State value for each transition
// than takes place.
type State struct {
	Channel string     // channel name or empty if Type is StateConn
	Err     *ErrorInfo // eventual error value associated with transition
	State   StateEnum  // state which connection or channel has transitioned to
	Type    StateType  // whether transition happened on connection or channel
}

type stateEmitter struct {
	sync.Mutex
	channel   string
	listeners map[StateEnum]map[chan<- State]struct{}
	onetime   map[StateEnum]map[chan<- State]struct{}
	err       *ErrorInfo
	current   StateEnum
	typ       StateType
	logger    *LoggerOptions

	eventEmitter *eventEmitter
}

func newStateEmitter(typ StateType, startState StateEnum, channel string, log *LoggerOptions) *stateEmitter {
	if !typ.Contains(startState) {
		panic(`invalid start state: "` + startState.String() + `"`)
	}
	return &stateEmitter{
		channel:   channel,
		listeners: make(map[StateEnum]map[chan<- State]struct{}),
		onetime:   make(map[StateEnum]map[chan<- State]struct{}),
		current:   startState,
		typ:       typ,
		logger:    log,

		eventEmitter: newEventEmitter(log),
	}
}

func (s *stateEmitter) set(state StateEnum, err error) error {
	previous := s.current
	changed := s.current != state
	s.current = state
	s.err = stateError(state, err)
	if changed {
		s.emit(State{
			Channel: s.channel,
			Err:     s.err,
			State:   s.current,
			Type:    s.typ,
		})
	}

	if StateConn.Contains(state) {
		previous := mapOldToNewConnState(previous)
		change := ConnectionStateChange{
			Current:  mapOldToNewConnState(s.current),
			Previous: previous,
			Reason:   s.err,
		}
		if !changed {
			change.Event = ConnectionEventUpdated
		} else {
			change.Event = ConnectionEvent(change.Current)
		}
		s.eventEmitter.Emit(change.Event, change)
	}

	return s.err
}

func (s *stateEmitter) emit(st State) {
	for ch := range s.listeners[st.State] {
		select {
		case ch <- st:
		default:
			s.logger.Printf(LogWarning, "dropping %s due to slow receiver", st)
		}
	}
	onetime := s.onetime[st.State]
	if len(onetime) != 0 {
		delete(s.onetime, st.State)
		for ch := range onetime {
			select {
			case ch <- st:
			default:
				s.logger.Printf(LogWarning, "dropping %s due to slow receiver", st)
			}
			for _, l := range s.onetime {
				delete(l, ch)
			}
		}
	}
}

func (s *stateEmitter) syncSet(state StateEnum, err error) error {
	s.Lock()
	defer s.Unlock()
	return s.set(state, err)
}

func (s *stateEmitter) once(ch chan<- State, states ...StateEnum) {
	if len(states) == 0 {
		states = stateAll[s.typ]
	}
	for _, state := range states {
		l, ok := s.onetime[state]
		if !ok {
			l = make(map[chan<- State]struct{})
			s.onetime[state] = l
		}
		l[ch] = struct{}{}
	}
}

func (s *stateEmitter) on(ch chan<- State, states ...StateEnum) {
	if ch == nil {
		panic(fmt.Sprintf("ably: %s On using nil channel", s.typ))
	}
	if len(states) == 0 {
		states = stateAll[s.typ]
	}
	s.Lock()
	for _, state := range states {
		if !s.typ.Contains(state) {
			panic(fmt.Sprintf("ably: %s On using invalid state value: %s", s.typ, state.String()))
		}
		l, ok := s.listeners[state]
		if !ok {
			l = make(map[chan<- State]struct{})
			s.listeners[state] = l
		}
		l[ch] = struct{}{}
	}
	s.Unlock()
}

func (s *stateEmitter) off(ch chan<- State, states ...StateEnum) {
	if ch == nil {
		panic(fmt.Sprintf("ably: %s Off using nil channel", s.typ))
	}
	if len(states) == 0 {
		states = stateAll[s.typ]
	}
	s.Lock()
	for _, state := range states {
		if !s.typ.Contains(state) {
			panic(fmt.Sprintf("ably: %s Off using invalid state value: %s", s.typ, state.String()))
		}
		delete(s.listeners[state], ch)
		if len(s.listeners[state]) == 0 {
			delete(s.listeners, state)
		}
	}
	s.Unlock()
}

// queuedEmitter emits confirmation events triggered by ACK or NACK messages.
type pendingEmitter struct {
	queue  []serialCh
	logger *LoggerOptions
}

func newPendingEmitter(log *LoggerOptions) pendingEmitter {
	return pendingEmitter{
		logger: log,
	}
}

type serialCh struct {
	serial int64
	ch     chan<- error
}

func (q pendingEmitter) Len() int {
	return len(q.queue)
}

func (q pendingEmitter) Less(i, j int) bool {
	return q.queue[i].serial < q.queue[j].serial
}

func (q pendingEmitter) Swap(i, j int) {
	q.queue[i], q.queue[j] = q.queue[j], q.queue[i]
}

func (q pendingEmitter) Search(serial int64) int {
	return sort.Search(q.Len(), func(i int) bool { return q.queue[i].serial >= serial })
}

func (q *pendingEmitter) Enqueue(serial int64, ch chan<- error) {
	switch i := q.Search(serial); {
	case i == q.Len():
		q.queue = append(q.queue, serialCh{serial, ch})
	case q.queue[i].serial == serial:
		q.logger.Printf(LogWarning, "duplicated message serial: %d", serial)
	default:
		q.queue = append(q.queue, serialCh{})
		copy(q.queue[i+1:], q.queue[i:])
		q.queue[i] = serialCh{serial, ch}
	}
}

func (q *pendingEmitter) Ack(serial int64, count int, err error) {
	if q.Len() == 0 {
		return
	}
	ack, nack := 0, 0
	// Ensure range [serial,serial+count] fits inside q.
	switch i := q.Search(serial); {
	case i == q.Len():
		nack = q.Len()
	case q.queue[i].serial == serial:
		nack = i
		ack = min(i+count, q.Len())
	default:
		nack = i + 1
		ack = min(i+1+count, q.Len())
	}
	if err == nil {
		err = newError(50000, err)
	}
	for _, sch := range q.queue[:nack] {
		q.logger.Printf(LogVerbose, "received NACK for message serial %d", sch.serial)
		sch.ch <- err
	}
	for _, sch := range q.queue[nack:ack] {
		q.logger.Printf(LogVerbose, "received ACK for message serial %d", sch.serial)
		sch.ch <- nil
	}
	q.queue = q.queue[ack:]
}

func (q *pendingEmitter) Nack(serial int64, count int, err error) {
	if q.Len() == 0 {
		return
	}
	nack := 0
	switch i := q.Search(serial); {
	case i == q.Len():
		nack = q.Len()
	case q.queue[i].serial == serial:
		nack = min(i+count, q.Len())
	default:
		nack = min(i+1+count, q.Len())
	}
	if err == nil {
		err = newError(50000, err)
	}
	for _, sch := range q.queue[:nack] {
		q.logger.Printf(LogVerbose, "received NACK for message serial %d", sch.serial)
		sch.ch <- err
	}
	q.queue = q.queue[nack:]
}

type msgch struct {
	msg *proto.ProtocolMessage
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

func (q *msgQueue) Enqueue(msg *proto.ProtocolMessage, listen chan<- error) {
	q.mtx.Lock()
	// TODO(rjeczalik): reorder the queue so Presence / Messages can be merged
	q.queue = append(q.queue, msgch{msg, listen})
	q.mtx.Unlock()
}

func (q *msgQueue) Flush() {
	q.mtx.Lock()
	for _, msgch := range q.queue {
		err := q.conn.send(msgch.msg, msgch.ch)
		if err != nil {
			q.logger().Printf(LogError, "failure sending message (serial=%d): %v", msgch.msg.MsgSerial, err)
			msgch.ch <- newError(90000, err)
		}
	}
	q.queue = nil
	q.mtx.Unlock()
}

func (q *msgQueue) Fail(err error) {
	q.mtx.Lock()
	for _, msgch := range q.queue {
		q.logger().Printf(LogError, "failure sending message (serial=%d): %v", msgch.msg.MsgSerial, err)
		msgch.ch <- newError(90000, err)
	}
	q.queue = nil
	q.mtx.Unlock()
}

func (q *msgQueue) logger() *LoggerOptions {
	return q.conn.logger()
}

var nopResult *errResult

type errResult struct {
	err    error
	listen <-chan error
}

func newErrResult() (Result, chan<- error) {
	listen := make(chan error, 1)
	res := &errResult{listen: listen}
	return res, listen
}

// Wait implements the Result interface.
func (res *errResult) Wait() error {
	if res == nil {
		return nil
	}
	if res.listen != nil {
		res.err = <-res.listen
		res.listen = nil
	}
	return res.err
}

type stateResult struct {
	err      error
	listen   <-chan State
	expected StateEnum
}

func newResult(expected StateEnum) (Result, chan<- State) {
	listen := make(chan State, 1)
	res := &stateResult{listen: listen, expected: expected}
	return res, listen
}

func (s *stateEmitter) listenResult(states ...StateEnum) Result {
	res, listen := newResult(states[0])
	s.once(listen, states...)
	return res
}

// Wait implements the Result interface.
func (res *stateResult) Wait() error {
	if res == nil {
		return nil
	}
	if res.listen != nil {
		switch state := <-res.listen; {
		case state.State == res.expected:
		case state.Err != nil:
			res.err = state.Err
		default:
			code := 50001
			if state.Type == StateConn {
				code = 50002
			}
			res.err = newError(code, fmt.Errorf("failed %s state: %s", state.Type, state.State))
		}
		res.listen = nil
	}
	return res.err
}

// A ConnectionState identifies the state of an Ably realtime connection.
type ConnectionState struct {
	name string
}

var (
	ConnectionStateInitialized  = ConnectionState{name: "INITIALIZED"}
	ConnectionStateConnecting   = ConnectionState{name: "CONNECTING"}
	ConnectionStateConnected    = ConnectionState{name: "CONNECTED"}
	ConnectionStateDisconnected = ConnectionState{name: "DISCONNECTED"}
	ConnectionStateSuspended    = ConnectionState{name: "SUSPENDED"}
	ConnectionStateClosing      = ConnectionState{name: "CLOSING"}
	ConnectionStateClosed       = ConnectionState{name: "CLOSED"}
	ConnectionStateFailed       = ConnectionState{name: "FAILED"}
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
	ConnectionEventInitialized  = ConnectionEvent(ConnectionStateInitialized)
	ConnectionEventConnecting   = ConnectionEvent(ConnectionStateConnecting)
	ConnectionEventConnected    = ConnectionEvent(ConnectionStateConnected)
	ConnectionEventDisconnected = ConnectionEvent(ConnectionStateDisconnected)
	ConnectionEventSuspended    = ConnectionEvent(ConnectionStateSuspended)
	ConnectionEventClosing      = ConnectionEvent(ConnectionStateClosing)
	ConnectionEventClosed       = ConnectionEvent(ConnectionStateClosed)
	ConnectionEventFailed       = ConnectionEvent(ConnectionStateFailed)
	ConnectionEventUpdated      = ConnectionEvent{name: "UPDATED"}
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
	// Reason, if any, is an error that caused the state change.
	Reason *ErrorInfo
}

func (ConnectionStateChange) isEmitterData() {}

func mapOldToNewConnState(old StateEnum) ConnectionState {
	switch old {
	case StateConnInitialized:
		return ConnectionStateInitialized
	case StateConnConnecting:
		return ConnectionStateConnecting
	case StateConnConnected:
		return ConnectionStateConnected
	case StateConnDisconnected:
		return ConnectionStateDisconnected
	case StateConnSuspended:
		return ConnectionStateSuspended
	case StateConnClosing:
		return ConnectionStateClosing
	case StateConnClosed:
		return ConnectionStateClosed
	case StateConnFailed:
		return ConnectionStateFailed
	default:
		panic(fmt.Errorf("unexpected StateEnum: %v", old))
	}
}
