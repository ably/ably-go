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

// PresenceMessageChannel gives a channel on which the presence messages are delivered.
// It panics when sub was not subscribed to receive channel's presence messages.
func (sub *Subscription) PresenceMessageChannel() <-chan *proto.PresenceMessage {
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
