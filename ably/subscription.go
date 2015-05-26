package ably

import (
	"reflect"
	"sync"

	"github.com/ably/ably-go/ably/proto"
)

var (
	subscriptionMessages         = reflect.TypeOf((chan *proto.Message)(nil))
	subscriptionPresenceMessages = reflect.TypeOf((chan *proto.ProtocolMessage)(nil))
)

// Subscription queues messages received from a realtime channel.
type Subscription struct {
	typ         reflect.Type
	mtx         sync.Mutex
	channel     interface{}
	sleep       chan struct{}
	queue       []interface{}
	unsubscribe func(*Subscription)
	stopped     bool
}

func newSubscription(typ reflect.Type, unsubscribe func(*Subscription)) *Subscription {
	sub := &Subscription{
		typ:         typ,
		channel:     reflect.MakeChan(typ, 0).Interface(),
		sleep:       make(chan struct{}, 1),
		unsubscribe: unsubscribe,
	}
	go sub.loop()
	return sub
}

// MessageChannel gives a channel on which the messages are delivered.
// It panics when sub was not subscribed to receive channel's messages.
func (sub *Subscription) MessageChannel() <-chan *proto.Message {
	if sub.typ != subscriptionMessages {
		panic(errInvalidType{typ: sub.typ})
	}
	ch := sub.channel.(chan *proto.Message)
	return ch
}

// PresenceChannel gives a channel on which the presence messages are delivered.
// It panics when sub was not subscribed to receive channel's presence messages.
func (sub *Subscription) PresenceChannel() <-chan *proto.PresenceMessage {
	if sub.typ != subscriptionPresenceMessages {
		panic(errInvalidType{typ: sub.typ})
	}
	ch := sub.channel.(chan *proto.PresenceMessage)
	return ch
}

// Close unsubscribes from the realtime channel the sub was previously subscribed.
// It closes the chan returned by C method.
func (sub *Subscription) Close() error {
	return sub.close(true)
}

func (sub *Subscription) dry() {
	switch channel := sub.channel.(type) {
	case chan *proto.Message:
		for {
			select {
			case <-channel:
			default:
				close(channel)
				return
			}
		}
	case chan *proto.PresenceMessage:
		for {
			select {
			case <-channel:
			default:
				close(channel)
				return
			}
		}
	default:
		panic(errInvalidType{typ: sub.typ})
	}
}

func (sub *Subscription) close(unsubscribe bool) error {
	if unsubscribe {
		sub.unsubscribe(sub)
	}
	sub.mtx.Lock()
	if sub.stopped {
		sub.mtx.Unlock()
		return nil
	}
	sub.stopped = true
	sub.queue = nil
	close(sub.sleep)
	sub.mtx.Unlock()
	sub.dry() // dry sub.channel to stop loop goroutine.
	return nil
}

// Len gives a number of messages currently queued.
func (sub *Subscription) Len() int {
	sub.mtx.Lock()
	defer sub.mtx.Unlock()
	return len(sub.queue)
}

func (sub *Subscription) enqueue(msg interface{}) {
	sub.mtx.Lock()
	defer sub.mtx.Unlock()
	if sub.stopped {
		return
	}
	sleeping := len(sub.queue) == 0
	sub.queue = append(sub.queue, msg)
	if sleeping {
		sub.sleep <- struct{}{}
	}
}

func (sub *Subscription) pop() (msg interface{}, n int) {
	sub.mtx.Lock()
	defer sub.mtx.Unlock()
	if n = len(sub.queue); n == 0 {
		return nil, 0
	}
	msg, sub.queue = sub.queue[0], sub.queue[1:]
	return msg, n
}

func (sub *Subscription) loop() {
	channel := reflect.ValueOf(sub.channel)
	for range sub.sleep {
		for msg, n := sub.pop(); n != 0; msg, n = sub.pop() {
			channel.Send(reflect.ValueOf(msg))
		}
	}
}

var (
	subsAll     struct{}
	subsAllKeys = []interface{}{subsAll}
)

func namesToKeys(names []string) []interface{} {
	if len(names) == 0 {
		return nil
	}
	keys := make([]interface{}, 0, len(names))
	for _, name := range names {
		// Ignore empty names.
		if name != "" {
			keys = append(keys, name)
		}
	}
	return keys
}

func statesToKeys(states []proto.PresenceState) []interface{} {
	if len(states) == 0 {
		return nil
	}
	keys := make([]interface{}, 0, len(states))
	for _, state := range states {
		keys = append(keys, state)
	}
	return keys
}

type subscriptions struct {
	mtx sync.Mutex
	all map[interface{}]map[*Subscription]struct{}
}

func newSubscriptions() *subscriptions {
	return &subscriptions{
		all: make(map[interface{}]map[*Subscription]struct{}),
	}
}

func (subs *subscriptions) close() {
	for _, subs := range subs.all {
		for sub := range subs {
			// Stop is idempotent, no need to keep track which sub was already
			// stopped.
			sub.close(false)
		}
	}
	// Unsubscribe all channels by creating new sub map for future use.
	subs.all = make(map[interface{}]map[*Subscription]struct{})
}

func (subs *subscriptions) subscribe(keys ...interface{}) (*Subscription, error) {
	unsubscribe := func(sub *Subscription) { subs.unsubscribe(false, sub, keys...) }
	sub := newSubscription(subscriptionMessages, unsubscribe)
	if len(keys) == 0 {
		keys = subsAllKeys
	}
	subs.mtx.Lock()
	for _, key := range keys {
		all, ok := subs.all[key]
		if !ok {
			all = make(map[*Subscription]struct{})
			subs.all[key] = all
		}
		all[sub] = struct{}{}
	}
	subs.mtx.Unlock()
	return sub, nil
}

func (subs *subscriptions) unsubscribe(stop bool, sub *Subscription, keys ...interface{}) {
	if len(keys) == 0 {
		keys = subsAllKeys
	}
	subs.mtx.Lock()
	for _, key := range keys {
		delete(subs.all[key], sub)
		if len(subs.all[key]) == 0 {
			delete(subs.all, key)
		}
	}
	// Don't try to stop when we got here from the (*Subscription).Close method.
	if stop {
		count := 0
		for _, all := range subs.all {
			if _, ok := all[sub]; ok {
				count++
			}
		}
		// Stop subscription if it no longer listens for messages.
		if count == 0 {
			sub.close(false)
		}
	}
	subs.mtx.Unlock()
}

func (subs *subscriptions) messageEnqueue(msg *proto.ProtocolMessage) {
	subs.mtx.Lock()
	for _, msg := range msg.Messages {
		if subs, ok := subs.all[subsAll]; ok {
			for sub := range subs {
				sub.enqueue(msg)
			}
		}
		if subs, ok := subs.all[msg.Name]; ok {
			for sub := range subs {
				sub.enqueue(msg)
			}
		}
	}
	subs.mtx.Unlock()
}

func (subs *subscriptions) presenceEnqueue(msg *proto.ProtocolMessage) {
	subs.mtx.Lock()
	for _, msg := range msg.Presence {
		if subs, ok := subs.all[subsAll]; ok {
			for sub := range subs {
				sub.enqueue(msg)
			}
		}
		if subs, ok := subs.all[msg.State]; ok {
			for sub := range subs {
				sub.enqueue(msg)
			}
		}
	}
	subs.mtx.Unlock()
}
