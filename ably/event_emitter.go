package ably

import (
	"runtime/debug"
	"sync"
)

type eventEmitter struct {
	sync.Mutex
	listeners listenersForEvent
	logger    Logger
}

type emitterEvent = interface{}
type emitterData = interface{}

type listenersForEvent map[emitterEvent]listenerSet
type listenerSet map[*eventListener]struct{}

type eventListener struct {
	handler func(emitterData)
	once    bool

	queueMtx sync.Mutex
	queue    []emitterData
}

func (l *eventListener) handle(e emitterData, log Logger) {
	// The goroutine that finds the queue empty launches a goroutine for emptying it,
	// ie. process its emitterData and afterwards any emitterData that concurrent
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

func safeHandle(e emitterData, handle func(emitterData), log Logger) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		log.Printf(LogError, "EventEmitter: panic in event handler: %v\n%s", r, debug.Stack())
	}()

	handle(e)
}

func newEventEmitter(logger Logger) *eventEmitter {
	return &eventEmitter{
		listeners: listenersForEvent{
			nil: listenerSet{},
		},
		logger: logger,
	}
}

// On registers an event listener. If event is nil, all events will trigger
// the listener. Otherwise, it must be comparable to the eventEmitter's event
// type, and only events equal to it will trigger the listener.
//
// It returns a function to deregister the listener.
func (em *eventEmitter) On(event emitterEvent, handle func(emitterData)) (off func()) {
	return em.on(event, handle, false)
}

// Once is like On, except the listener is deregistered once first triggered.
func (em *eventEmitter) Once(event emitterEvent, handle func(emitterData)) (off func()) {
	return em.on(event, handle, true)
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

// Off deregisters event listeners. If event is nil, all event listeners are
// deregistered. Otherwise, it must be comparable to the eventEmitter's event
// type, and only listeners that were associated with that event will be
// removed.
func (em *eventEmitter) Off(event emitterEvent) {
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

func (em *eventEmitter) Emit(event emitterEvent, data emitterData) {
	// Let's first collect the handlers, and then call them outside the lock.
	// This allows the handler functions to call again into the event emitter,
	// which would otherwise deadlock.

	for _, handle := range em.handlersForEvent(event) {
		handle(data, em.logger)
	}
}

func (em *eventEmitter) handlersForEvent(event emitterEvent) (handlers []func(emitterData, Logger)) {
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
