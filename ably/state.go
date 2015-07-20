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

// State describes a single state transition of either realtime connection or channel
// that occured due to some external condition (dropped connection, retried etc.).
//
// Each realtime connection and channel maintains its state to ensure high availability
// and resilience, which is inherently asynchronous. In order to listen to transition
// between states for both realtime connection and realtime channel user may provide
// a channel, which will get notified with single State value for each transition
// than takes place.
type State struct {
	Channel string    // channel name or empty if Type is StateConn
	Err     error     // eventual error value associated with transition
	State   StateEnum // state which connection or channel has transitioned to
	Type    StateType // whether transition happened on connection or channel
}

type stateEmitter struct {
	sync.Mutex
	channel   string
	listeners map[StateEnum]map[chan<- State]struct{}
	onetime   map[StateEnum]map[chan<- State]struct{}
	err       error
	current   StateEnum
	typ       StateType
}

func newStateEmitter(typ StateType, startState StateEnum, channel string) *stateEmitter {
	if !typ.Contains(startState) {
		panic(`invalid start state: "` + startState.String() + `"`)
	}
	return &stateEmitter{
		channel:   channel,
		listeners: make(map[StateEnum]map[chan<- State]struct{}),
		onetime:   make(map[StateEnum]map[chan<- State]struct{}),
		current:   startState,
		typ:       typ,
	}
}

func (s *stateEmitter) set(state StateEnum, err error) error {
	st := State{
		Channel: s.channel,
		Err:     err,
		State:   state,
		Type:    s.typ,
	}
	s.current = state
	s.err = err
	for ch := range s.listeners[state] {
		select {
		case ch <- st:
		default:
			Log.Printf(LogWarn, "dropping %s due to slow receiver", st)
		}
	}
	onetime := s.onetime[state]
	if len(onetime) != 0 {
		delete(s.onetime, state)
		for ch := range onetime {
			select {
			case ch <- st:
			default:
				Log.Printf(LogWarn, "dropping %s due to slow receiver", st)
			}
			for _, l := range s.onetime {
				delete(l, ch)
			}
		}
	}
	return s.err
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
type pendingEmitter []serialCh

type serialCh struct {
	serial int64
	ch     chan<- error
}

func (q pendingEmitter) Len() int {
	return len(q)
}

func (q pendingEmitter) Less(i, j int) bool {
	return q[i].serial < q[j].serial
}

func (q pendingEmitter) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

func (q pendingEmitter) Search(serial int64) int {
	return sort.Search(q.Len(), func(i int) bool { return q[i].serial >= serial })
}

func (q *pendingEmitter) Enqueue(serial int64, ch chan<- error) {
	switch i := q.Search(serial); {
	case i == q.Len():
		*q = append(*q, serialCh{serial, ch})
	case (*q)[i].serial == serial:
		panic(fmt.Sprintf("duplicated message serial: %d", serial))
	default:
		*q = append(*q, serialCh{})
		copy((*q)[i+1:], (*q)[i:])
		(*q)[i] = serialCh{serial, ch}
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
	case (*q)[i].serial == serial:
		nack = i
		ack = min(i+count, q.Len())
	default:
		nack = i + 1
		ack = min(i+1+count, q.Len())
	}
	if err == nil {
		err = newError(50000, err)
	}
	for _, sch := range (*q)[:nack] {
		select {
		case sch.ch <- err:
		default:
			Log.Printf(LogWarn, "dropping nack for message serial %d due to slow receiver: %v", sch.serial, err)
		}
	}
	for _, sch := range (*q)[nack:ack] {
		select {
		case sch.ch <- nil:
		default:
			Log.Printf(LogWarn, "dropping ack for message serial %d due to slow receiver", sch.serial)
		}
	}
	*q = (*q)[ack:]
}

func (q *pendingEmitter) Nack(serial int64, count int, err error) {
	if q.Len() == 0 {
		return
	}
	nack := 0
	switch i := q.Search(serial); {
	case i == q.Len():
		nack = q.Len()
	case (*q)[i].serial == serial:
		nack = min(i+count, q.Len())
	default:
		nack = min(i+1+count, q.Len())
	}
	if err == nil {
		err = newError(50000, err)
	}
	for _, sch := range (*q)[:nack] {
		select {
		case sch.ch <- err:
		default:
			Log.Printf(LogWarn, "dropping nack for message serial %d due to slow receiver: %v", sch.serial, err)
		}
	}
	*q = (*q)[nack:]
}

type msgch struct {
	msg *proto.ProtocolMessage
	ch  chan<- error
}

type msgQueue struct {
	mtx   sync.Mutex
	queue []msgch
	conn  *Conn
}

func newMsgQueue(conn *Conn) *msgQueue {
	return &msgQueue{conn: conn}
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
			select {
			case msgch.ch <- newError(90000, err):
			default:
				Log.Printf(LogWarn, "dropped message serial %d due to slow receiver", msgch.msg.MsgSerial)
			}
		}
	}
	q.queue = nil
	q.mtx.Unlock()
}

func (q *msgQueue) Fail(err error) {
	q.mtx.Lock()
	for _, msgch := range q.queue {
		select {
		case msgch.ch <- newError(90000, err):
		default:
			Log.Printf(LogWarn, "dropped message serial %d due to slow receiver", msgch.msg.MsgSerial)
		}
	}
	q.queue = nil
	q.mtx.Unlock()
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
		case state.Err != nil:
			res.err = state.Err
		case state.State == res.expected:
		default:
			code := 50001
			if state.Type == StateConn {
				code = 50002
			}
			res.err = &Error{
				Code: code,
				Err:  fmt.Errorf("failed %s state: %s", state.Type, state.State),
			}
		}
		res.listen = nil
	}
	return res.err
}
