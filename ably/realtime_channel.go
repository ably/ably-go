package ably

import (
	"errors"
	"sort"
	"sync"

	"github.com/ably/ably-go/ably/proto"
)

type chanSlice []*RealtimeChannel

func (ch chanSlice) Len() int           { return len(ch) }
func (ch chanSlice) Less(i, j int) bool { return ch[i].Name < ch[j].Name }
func (ch chanSlice) Swap(i, j int)      { ch[i], ch[j] = ch[j], ch[i] }
func (ch chanSlice) Sort()              { sort.Sort(ch) }

// Channels is a goroutine-safe container for realtime channels that allows
// for creating, deleting and iterating over existing channels.
type Channels struct {
	mtx    sync.Mutex
	client *RealtimeClient
	chans  map[string]*RealtimeChannel
}

func newChannels(client *RealtimeClient) *Channels {
	return &Channels{
		client: client,
		chans:  make(map[string]*RealtimeChannel),
	}
}

// Get looks up a channel given by the name and creates it if it does not exist
// already.
//
// It is safe to call Get from multiple goroutines - a single channel is
// guaranteed to be created only once for multiple calls to Get from different
// goroutines.
func (ch *Channels) Get(name string) *RealtimeChannel {
	ch.mtx.Lock()
	c, ok := ch.chans[name]
	if !ok {
		c = newRealtimeChannel(name, ch.client)
		ch.chans[name] = c
	}
	ch.mtx.Unlock()
	return c
}

// All returns a list of created channels.
//
// It is safe to call All from multiple goroutines, however there's no guarantee
// the returned list would not list a channel that was already released from
// different goroutine.
//
// The returned list is sorted by channel names.
func (ch *Channels) All() []*RealtimeChannel {
	ch.mtx.Lock()
	chans := make([]*RealtimeChannel, 0, len(ch.chans))
	for _, c := range ch.chans {
		chans = append(chans, c)
	}
	ch.mtx.Unlock()
	chanSlice(chans).Sort()
	return chans

}

// Release closes a channel looked up by the name.
//
// It is safe to call Release from multiple goroutines - if a channel happened
// to be already concurrently released, the method is a nop.
func (ch *Channels) Release(name string) error {
	ch.mtx.Lock()
	defer ch.mtx.Unlock()
	if c, ok := ch.chans[name]; ok {
		delete(ch.chans, name)
		return c.Close()
	}
	return nil
}

// RealtimeChannel represents a single named message channel.
type RealtimeChannel struct {
	Name string // name used to create the channel

	client  *RealtimeClient
	state   *stateEmitter
	err     error
	subs    map[string]map[*Subscription]struct{}
	subsMtx sync.Mutex
	queue   *msgQueue
}

func newRealtimeChannel(name string, client *RealtimeClient) *RealtimeChannel {
	c := &RealtimeChannel{
		Name:   name,
		client: client,
		state:  newStateEmitter(StateChan, StateChanInitialized, name),
		subs:   make(map[string]map[*Subscription]struct{}),
	}
	c.queue = newMsgQueue(client.Connection)
	if c.client.opts.Listener != nil {
		c.On(c.client.opts.Listener)
	}
	return c
}

var errAttach = newError(90000, errors.New("Attach() on inactive connection"))

// Attach initiates attach request, which is being processed on a separate
// goroutine.
//
// If channel is already attached, this method is a nop.
// If sending attach message failed, the returned error value is non-nil.
// If sending attach message succeed, the returned Result value can be used
// to wait until result from server is received.
func (c *RealtimeChannel) Attach() (Result, error) {
	return c.attach(true)
}

var attachResultStates = []int{
	StateChanAttached, // expected state
	StateChanClosing,
	StateChanClosed,
	StateChanFailed,
}

func (c *RealtimeChannel) attach(result bool) (Result, error) {
	c.state.Lock()
	defer c.state.Unlock()
	if c.isActive() {
		return nil, nil
	}
	c.state.set(StateChanAttaching, nil)
	if !c.client.Connection.lockIsActive() {
		return nil, c.state.set(StateChanFailed, errAttach)
	}
	var res Result
	if result {
		res = c.state.listenResult(attachResultStates...)
	}
	msg := &proto.ProtocolMessage{
		Action:  proto.ActionAttach,
		Channel: c.state.channel,
	}
	err := c.client.Connection.send(msg, nil)
	if err != nil {
		return nil, c.state.set(StateChanFailed, newErrorf(90000, "Attach() error: %s", err))
	}
	return res, nil
}

var errDetach = newError(90000, errors.New("Detach() on inactive connection"))

// Detach initiates detach request, which is being processed on a separate
// goroutine.
//
// If channel is already detached, this method is a nop.
// If sending detach message failed, the returned error value is non-nil.
// If sending detach message succeed, the returned Result value can be used
// to wait until result from server is received.
func (c *RealtimeChannel) Detach() (Result, error) {
	return c.detach(true)
}

var detachResultStates = []int{
	StateChanDetached, // expected state
	StateChanClosing,
	StateChanClosed,
	StateChanFailed,
}

func (c *RealtimeChannel) detach(result bool) (Result, error) {
	c.state.Lock()
	defer c.state.Unlock()
	if !c.isActive() {
		return nil, nil
	}
	c.state.set(StateChanDetaching, nil)
	if !c.client.Connection.lockIsActive() {
		return nil, c.state.set(StateChanFailed, errDetach)
	}
	var res Result
	if result {
		res = c.state.listenResult(detachResultStates...)
	}
	msg := &proto.ProtocolMessage{
		Action:  proto.ActionDetach,
		Channel: c.state.channel,
	}
	err := c.client.Connection.send(msg, nil)
	if err != nil {
		return nil, c.state.set(StateChanFailed, newErrorf(90000, "Detach() error: %s", err))
	}
	return res, nil
}

// Closes initiaties closing sequence for the channel; it waits until the
// operation is complete.
//
// If connection is already closed, this method is a nop.
// If sending close message succeeds, it closes and unsubscribes all channels.
func (c *RealtimeChannel) Close() error {
	err := wait(c.Detach())
	c.state.Lock()
	for _, subs := range c.subs {
		for sub := range subs {
			// Stop is idempotent, no need to keep track which sub was already
			// stopped.
			sub.close(false)
		}
	}
	// Unsubscribe all channels by creating new sub map for future use.
	c.subs = make(map[string]map[*Subscription]struct{})
	c.state.Unlock()
	if err != nil {
		return c.state.syncSet(StateChanClosed, newErrorf(90000, "Close() error: %s", err))
	}
	return nil
}

// Subscribe subscribes to a realtime channel, which makes any newly received
// messages relayed to the returned Subscription value.
//
// If no names are given, returned Subscription will receive all messages.
// If ch is non-nil and it was already registered to receive messages with different
// names than the ones given, it will be added to receive also the new ones.
func (c *RealtimeChannel) Subscribe(names ...string) (*Subscription, error) {
	if _, err := c.attach(false); err != nil {
		return nil, err
	}
	unsubscribe := func(sub *Subscription) { c.unsubscribe(false, sub, names...) }
	sub := newSubscription(subscriptionMessages, unsubscribe)
	all := false
	if len(names) == 0 {
		all = true
		names = []string{""}
	}
	c.subsMtx.Lock()
	for _, name := range names {
		// Ignore empty message names.
		if name == "" && !all {
			continue
		}
		subs, ok := c.subs[name]
		if !ok {
			subs = make(map[*Subscription]struct{})
			c.subs[name] = subs
		}
		subs[sub] = struct{}{}
	}
	c.subsMtx.Unlock()
	return sub, nil
}

// Unsubscribe removes previous Subscription for the given message names.
func (c *RealtimeChannel) Unsubscribe(sub *Subscription, names ...string) {
	c.unsubscribe(true, sub, names...)
}
func (c *RealtimeChannel) unsubscribe(stop bool, sub *Subscription, names ...string) {
	if len(names) == 0 {
		names = []string{""}
	}
	c.subsMtx.Lock()
	for _, name := range names {
		delete(c.subs[name], sub)
		if len(c.subs[name]) == 0 {
			delete(c.subs, name)
		}
	}
	// Don't try to stop when we got here from the (*Subscription).Close method.
	if stop {
		count := 0
		for _, subs := range c.subs {
			if _, ok := subs[sub]; ok {
				count++
			}
		}
		// Stop subscription if it no longer listens for messages.
		if count == 0 {
			sub.close(false)
		}
	}
	c.subsMtx.Unlock()
}

// On relays request channel states to c; on state transition
// connection will not block sending to c - the caller must ensure the incoming
// values are read at proper pace or the c is sufficiently buffered.
//
// If no states are given, c is registered for all of them.
// If c is nil, the method panics.
// If c is already registered, its state set is expanded.
func (c *RealtimeChannel) On(ch chan<- State, states ...int) {
	c.state.on(ch, states...)
}

// Off removes c from listening on the given channel state transitions.
//
// If no states are given, c is removed for all of the connection's states.
// If c is nil, the method panics.
func (c *RealtimeChannel) Off(ch chan<- State, states ...int) {
	c.state.off(ch, states...)
}

// Publish publishes a message on the channel, which is send on separate
// goroutine. Publish does not block.
//
// This implicitly attaches the channel if it's not already attached.
func (c *RealtimeChannel) Publish(name string, data string) (Result, error) {
	return c.PublishAll([]*proto.Message{{Name: name, Data: data}})
}

// PublishAll publishes all given messages on the channel at once.
// PublishAll does not block.
//
// This implicitly attaches the channel if it's not already attached.
func (c *RealtimeChannel) PublishAll(messages []*proto.Message) (Result, error) {
	if _, err := c.attach(false); err != nil {
		return nil, err
	}
	msg := &proto.ProtocolMessage{
		Action:   proto.ActionMessage,
		Channel:  c.Name,
		Messages: messages,
	}
	res, listen := newErrResult()
	switch c.State() {
	case StateChanInitialized, StateChanAttaching:
		c.queue.Enqueue(msg, listen)
		return res, nil
	case StateChanAttached:
	default:
		return nil, &Error{Code: 90001}
	}
	if err := c.client.Connection.send(msg, listen); err != nil {
		return nil, err
	}
	return res, nil
}

// State gives current state of the channel.
func (c *RealtimeChannel) State() int {
	c.state.Lock()
	defer c.state.Unlock()
	return c.state.current
}

// Reason gives the last error that caused channel transition to failed state.
func (c *RealtimeChannel) Reason() error {
	c.state.Lock()
	defer c.state.Unlock()
	return c.state.err
}

func (c *RealtimeChannel) notify(msg *proto.ProtocolMessage) {
	switch msg.Action {
	case proto.ActionAttached:
		c.state.syncSet(StateChanAttached, nil)
		c.queue.Flush()
	case proto.ActionDetached:
		c.state.syncSet(StateChanDetached, nil)
	case proto.ActionPresence:
		// TODO realtime presence
	case proto.ActionError:
		c.state.syncSet(StateChanFailed, newErrorProto(msg.Error))
		c.queue.Fail(newErrorProto(msg.Error))
		// TODO recovery
	case proto.ActionMessage:
		c.subsMtx.Lock()
		for _, msg := range msg.Messages {
			if subs, ok := c.subs[""]; ok {
				for sub := range subs {
					sub.enqueue(msg)
				}
			}
			if subs, ok := c.subs[msg.Name]; ok {
				for sub := range subs {
					sub.enqueue(msg)
				}
			}
		}
		c.subsMtx.Unlock()
	default:
	}
}

func (c *RealtimeChannel) isActive() bool {
	return c.state.current == StateChanAttaching || c.state.current == StateChanAttached
}
