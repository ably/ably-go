package ably

import (
	"errors"
	"sort"
	"sync"

	"github.com/ably/ably-go/ably/proto"
)

var (
	errAttach = errors.New("attempted to attach channel to inactive connection")
	errDetach = errors.New("attempted to detach channel from inactive connection")
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
	Name     string            // name used to create the channel
	Presence *RealtimePresence //

	client *RealtimeClient
	state  *stateEmitter
	subs   *subscriptions
	queue  *msgQueue
	listen chan State
}

func newRealtimeChannel(name string, client *RealtimeClient) *RealtimeChannel {
	c := &RealtimeChannel{
		Name:   name,
		client: client,
		state:  newStateEmitter(StateChan, StateChanInitialized, name, client.logger()),
		subs:   newSubscriptions(subscriptionMessages, client.logger()),
		listen: make(chan State, 1),
	}
	c.Presence = newRealtimePresence(c)
	c.queue = newMsgQueue(client.Connection)
	if c.opts().Listener != nil {
		c.On(c.opts().Listener)
	}
	c.client.Connection.On(c.listen, StateConnFailed, StateConnClosed)
	go c.listenLoop()
	return c
}

func (c *RealtimeChannel) listenLoop() {
	for state := range c.listen {
		c.state.Lock()
		active := c.isActive()
		c.state.Unlock()
		switch state.State {
		case StateConnFailed:
			if active {
				c.state.syncSet(StateChanFailed, state.Err)
			}
		case StateConnClosed:
			if active {
				c.state.syncSet(StateChanClosed, state.Err)
			}
		}
	}
}

// Attach initiates attach request, which is being processed on a separate
// goroutine.
//
// If channel is already attached, this method is a nop.
// If sending attach message failed, the returned error value is non-nil.
// If sending attach message succeed, the returned Result value can be used
// to wait until ack from server is received.
func (c *RealtimeChannel) Attach() (Result, error) {
	return c.attach(true)
}

var attachResultStates = []StateEnum{
	StateChanAttached, // expected state
	StateChanClosing,
	StateChanClosed,
	StateChanFailed,
}

func (c *RealtimeChannel) attach(result bool) (Result, error) {
	c.state.Lock()
	defer c.state.Unlock()
	if c.isActive() {
		return nopResult, nil
	}
	if !c.client.Connection.lockIsActive() {
		return nil, c.state.set(StateChanFailed, errAttach)
	}
	c.state.set(StateChanAttaching, nil)
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
		return nil, c.state.set(StateChanFailed, err)
	}
	return res, nil
}

// Detach initiates detach request, which is being processed on a separate
// goroutine.
//
// If channel is already detached, this method is a nop.
// If sending detach message failed, the returned error value is non-nil.
// If sending detach message succeed, the returned Result value can be used
// to wait until ack from server is received.
func (c *RealtimeChannel) Detach() (Result, error) {
	return c.detach(true)
}

var detachResultStates = []StateEnum{
	StateChanDetached, // expected state
	StateChanClosing,
	StateChanClosed,
	StateChanFailed,
}

func (c *RealtimeChannel) detach(result bool) (Result, error) {
	c.state.Lock()
	defer c.state.Unlock()
	switch {
	case c.state.current == StateChanFailed:
		return nil, stateError(StateChanFailed, errDetach)
	case !c.isActive():
		return nopResult, nil
	}
	if !c.client.Connection.lockIsActive() {
		return nil, c.state.set(StateChanFailed, errDetach)
	}
	c.state.set(StateChanDetaching, nil)
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
		return nil, c.state.set(StateChanFailed, err)
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
	c.subs.close()
	if err != nil {
		return c.state.syncSet(StateChanClosed, err)
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
	return c.subs.subscribe(namesToKeys(names)...)
}

// Unsubscribe removes previous Subscription for the given message names.
//
// Unsubscribe panics if the given sub was subscribed for presence messages and
// not for regular channel messages.
//
// If sub was already unsubscribed, the method is a nop.
func (c *RealtimeChannel) Unsubscribe(sub *Subscription, names ...string) {
	if sub.typ != subscriptionMessages {
		panic(errInvalidType{typ: sub.typ})
	}
	c.subs.unsubscribe(true, sub, namesToKeys(names)...)
}

// On relays request channel states to c; on state transition
// connection will not block sending to c - the caller must ensure the incoming
// values are read at proper pace or the c is sufficiently buffered.
//
// If no states are given, c is registered for all of them.
// If c is nil, the method panics.
// If c is already registered, its state set is expanded.
func (c *RealtimeChannel) On(ch chan<- State, states ...StateEnum) {
	c.state.on(ch, states...)
}

// Off removes c from listening on the given channel state transitions.
//
// If no states are given, c is removed for all of the connection's states.
// If c is nil, the method panics.
func (c *RealtimeChannel) Off(ch chan<- State, states ...StateEnum) {
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
	msg := &proto.ProtocolMessage{
		Action:   proto.ActionMessage,
		Channel:  c.state.channel,
		Messages: messages,
	}
	return c.send(msg)
}

// History gives the channel's message history according to the given parameters.
// The returned result can be inspected for the messages via the Messages()
// method.
func (c *RealtimeChannel) History(params *PaginateParams) (*PaginatedResult, error) {
	return c.client.rest.Channel(c.Name).History(params)
}

func (c *RealtimeChannel) send(msg *proto.ProtocolMessage) (Result, error) {
	if _, err := c.attach(false); err != nil {
		return nil, err
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
func (c *RealtimeChannel) State() StateEnum {
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
		c.Presence.onAttach(msg)
		c.state.syncSet(StateChanAttached, nil)
		c.queue.Flush()
	case proto.ActionDetached:
		c.state.syncSet(StateChanDetached, nil)
	case proto.ActionSync:
		c.Presence.processIncomingMessage(msg, syncSerial(msg))
	case proto.ActionPresence:
		c.Presence.processIncomingMessage(msg, "")
	case proto.ActionError:
		c.state.syncSet(StateChanFailed, newErrorProto(msg.Error))
		c.queue.Fail(newErrorProto(msg.Error))
	case proto.ActionMessage:
		c.subs.messageEnqueue(msg)
	default:
	}
}

func (c *RealtimeChannel) isActive() bool {
	return c.state.current == StateChanAttaching || c.state.current == StateChanAttached
}

func (c *RealtimeChannel) opts() *ClientOptions {
	return c.client.opts()
}

func (c *RealtimeChannel) logger() *LoggerOptions {
	return c.client.logger()
}
