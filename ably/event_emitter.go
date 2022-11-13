package ably

import (
	"runtime/debug"
	"sync"
)

// **CANONICAL**
// A generic interface for event registration and delivery used in a number of the types in the Realtime client library. For example, the [Connection]{@link} object emits events for connection state using the EventEmitter pattern.
type eventEmitter struct {
	sync.Mutex
	listeners listenersForEvent
	log       logger
}

type emitterEvent interface {
	isEmitterEvent()
}

type emitterData interface {
	isEmitterData()
}

type listenersForEvent map[emitterEvent]listenerSet
type listenerSet map[*eventListener]struct{}

type eventListener struct {
	handler func(emitterData)
	once    bool

	queueMtx sync.Mutex
	queue    []emitterData
}

func (l *eventListener) handle(e emitterData, log logger) {
	// The goroutine that finds the queue empty launches a goroutine for emptying it,
	// i.e. process its emitterData and afterwards any emitterData that concurrent
	// goroutines may have left in the queue in the meantime.
	//
	// Other goroutines just enqueue their emitterData.
	//
	// This ensures our guarantee that a single handler runs concurrently with
	// other handlers, but calls to the same handler are sequential and ordered,
	// while emitting an event doesn't block the emitting goroutine.

	var isBusy bool

	l.queueMtx.Lock()
	isBusy = len(l.queue) > 0
	l.queue = append(l.queue, e)
	l.queueMtx.Unlock()

	if isBusy {
		return
	}

	go func() {
		done := false
		for !done {
			l.queueMtx.Lock()
			e := l.queue[0]
			l.queueMtx.Unlock()

			safeHandle(e, l.handler, log)

			l.queueMtx.Lock()
			l.queue = l.queue[1:]
			done = len(l.queue) == 0
			l.queueMtx.Unlock()
		}
	}()
}

func safeHandle(e emitterData, handle func(emitterData), log logger) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		log.Errorf("EventEmitter: panic in event handler: %v\n%s", r, debug.Stack())
	}()

	handle(e)
}

func newEventEmitter(log logger) *eventEmitter {
	return &eventEmitter{
		listeners: listenersForEvent{
			nil: listenerSet{},
		},
		log: log,
	}
}

// **LEGACY**
// On registers an event listener. The event must be comparable to the
// eventEmitter's event type, and only events equal to it will trigger the
// listener.
//
// It returns a function to deregister the listener.
// **CANONICAL**
// Registers the provided listener for the specified event. If on() is called more than once with the same listener and event, the listener is added multiple times to its listener registry. Therefore, as an example, assuming the same listener is registered twice using on(), and an event is emitted once, the listener would be invoked twice.
// RTE4
func (em *eventEmitter) On(event emitterEvent, handle func(emitterData)) (off func()) {
	return em.on(event, handle, false)
}

// **LEGACY**
// OnAll is like On, except the listener is triggered by all events.
// **CANONICAL**
// Registers the provided listener all events. If on() is called more than once with the same listener and event, the listener is added multiple times to its listener registry. Therefore, as an example, assuming the same listener is registered twice using on(), and an event is emitted once, the listener would be invoked twice.
// RTE4
func (em *eventEmitter) OnAll(handle func(emitterData)) (off func()) {
	return em.on(nil, handle, false)
}

// **LEGACY**
// Once is like On, except the listener is deregistered once first triggered.
// **CANONICAL**
// Registers the provided listener for the first occurrence of a single named event specified as the Event argument. If once() is called more than once with the same listener, the listener is added multiple times to its listener registry. Therefore, as an example, assuming the same listener is registered twice using once(), and an event is emitted once, the listener would be invoked twice. However, all subsequent events emitted would not invoke the listener as once() ensures that each registration is only invoked once.
// RTE4
func (em *eventEmitter) Once(event emitterEvent, handle func(emitterData)) (off func()) {
	return em.on(event, handle, true)
}

// **LEGACY**
// OnceAll is like Once, except the listener is triggered by all events.
// **CANONICAL**
// Registers the provided listener for the first event that is emitted. If once() is called more than once with the same listener, the listener is added multiple times to its listener registry. Therefore, as an example, assuming the same listener is registered twice using once(), and an event is emitted once, the listener would be invoked twice. However, all subsequent events emitted would not invoke the listener as once() ensures that each registration is only invoked once.
// RTE4
func (em *eventEmitter) OnceAll(handle func(emitterData)) (off func()) {
	return em.on(nil, handle, true)
}

func (em *eventEmitter) on(event emitterEvent, handle func(emitterData), once bool) (off func()) {
	em.Lock()
	defer em.Unlock()

	l := &eventListener{
		handler: handle,
		once:    once,
	}

	listeners := em.listeners[event]
	if listeners == nil {
		listeners = listenerSet{}
		em.listeners[event] = listeners
	}

	listeners[l] = struct{}{}

	return func() {
		em.Lock()
		defer em.Unlock()

		listeners := em.listeners[event]
		if listeners != nil {
			delete(listeners, l)
		}
	}
}

// **LEGACY**
// Off deregisters event listeners. The event must be comparable to the
// eventEmitter's event type, and only listeners that were associated with that
// event will be removed.
// **CANONICAL**
// Removes all listeners matching the given event.
func (em *eventEmitter) Off(event emitterEvent) {
	em.off(event)
}

// **LEGACY**
// OffAll is like Off, except is deregisters all event listeners.
// **CANONICAL**
// Deregisters all registrations, for all events and listeners.
// RTE5
func (em *eventEmitter) OffAll() {
	em.off(nil)
}

func (em *eventEmitter) off(event emitterEvent) {
	em.Lock()
	defer em.Unlock()

	if event != nil {
		delete(em.listeners, event)
	} else {
		em.listeners = listenersForEvent{
			nil: listenerSet{},
		}
	}
}

// **CANONICAL**
// Emits an event, calling registered listeners with the given event name and any other given arguments. If an exception is raised in any of the listeners, the exception is caught by the EventEmitter and the exception is logged to the Ably logger.
// event - The named event.
// data - The event listener.
// internal, RTE6
func (em *eventEmitter) Emit(event emitterEvent, data emitterData) {
	// Let's first collect the handlers, and then call them outside the lock.
	// This allows the handler functions to call again into the event emitter,
	// which would otherwise deadlock.

	for _, handle := range em.handlersForEvent(event) {
		handle(data, em.log)
	}
}

func (em *eventEmitter) handlersForEvent(event emitterEvent) (handlers []func(emitterData, logger)) {
	em.Lock()
	defer em.Unlock()

	sets := []listenerSet{em.listeners[nil]}
	if event != nil {
		sets = append(sets, em.listeners[event])
	}

	for _, listeners := range sets {
		for l, _ := range listeners {
			if l.once {
				delete(listeners, l)
			}
			handlers = append(handlers, l.handle)
		}
	}

	return handlers
}
