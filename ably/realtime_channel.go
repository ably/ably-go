package ably

import (
	"context"
	"errors"
	"fmt"
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
	client *Realtime
	chans  map[string]*RealtimeChannel
}

func newChannels(client *Realtime) *Channels {
	return &Channels{
		client: client,
		chans:  make(map[string]*RealtimeChannel),
	}
}

// ChannelOptions is a set of options for a channel.
type ChannelOptions []ChannelOption

// A ChannelOption configures a channel. Options are set by calling methods
// on ChannelOptions.
type ChannelOption func(*channelOptions)

// channelOptions wraps proto.ChannelOptions. It exists so that users can't
// implement their own ChannelOption.
type channelOptions proto.ChannelOptions

// CipherKey is like Cipher with an AES algorithm and CBC mode.
func (o ChannelOptions) CipherKey(key []byte) ChannelOptions {
	return append(o, func(o *channelOptions) {
		o.Cipher = proto.CipherParams{
			Algorithm: proto.DefaultCipherAlgorithm,
			Key:       key,
			KeyLength: proto.DefaultKeyLength,
			Mode:      proto.DefaultCipherMode,
		}
	})
}

// Cipher sets cipher parameters for encrypting messages on a channel.
func (o ChannelOptions) Cipher(params proto.CipherParams) ChannelOptions {
	return append(o, func(o *channelOptions) {
		o.Cipher = params
	})
}

// Get looks up a channel given by the name and creates it if it does not exist
// already.
//
// It is safe to call Get from multiple goroutines - a single channel is
// guaranteed to be created only once for multiple calls to Get from different
// goroutines.
func (ch *Channels) Get(name string, options ...ChannelOption) *RealtimeChannel {
	// TODO: options
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
	c, ok := ch.chans[name]
	if !ok {
		return nil
	}
	err := wait(c.Detach())
	if err != nil {
		return err
	}
	delete(ch.chans, name)
	return nil
}

func (ch *Channels) broadcastConnStateChange(change ConnectionStateChange) {
	ch.mtx.Lock()
	defer ch.mtx.Unlock()
	for _, c := range ch.chans {
		c.onConnStateChange(change)
	}
}

// RealtimeChannel represents a single named message channel.
type RealtimeChannel struct {
	mtx sync.Mutex

	ChannelEventEmitter
	Name     string            // name used to create the channel
	Presence *RealtimePresence //

	state           ChannelState
	errorReason     *ErrorInfo
	internalEmitter ChannelEventEmitter

	client *Realtime
	subs   *subscriptions
	queue  *msgQueue
}

func newRealtimeChannel(name string, client *Realtime) *RealtimeChannel {
	c := &RealtimeChannel{
		ChannelEventEmitter: ChannelEventEmitter{newEventEmitter(client.logger())},
		Name:                name,

		state:           ChannelStateInitialized,
		internalEmitter: ChannelEventEmitter{newEventEmitter(client.logger())},

		client: client,
		subs:   newSubscriptions(subscriptionMessages, client.logger()),
	}
	c.Presence = newRealtimePresence(c)
	c.queue = newMsgQueue(client.Connection)
	return c
}

func (c *RealtimeChannel) onConnStateChange(change ConnectionStateChange) {
	c.mtx.Lock()
	active := c.isActive()
	c.mtx.Unlock()
	switch change.Current {
	case ConnectionStateFailed:
		if active {
			c.setState(ChannelStateFailed, change.Reason)
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

func (c *RealtimeChannel) attach(result bool) (Result, error) {
	return c.mayAttach(result, true)
}

func (c *RealtimeChannel) mayAttach(result, checkActive bool) (Result, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if checkActive {
		if c.isActive() {
			return nopResult, nil
		}
	}
	return c.lockAttach(result, nil)
}

func (c *RealtimeChannel) lockAttach(result bool, err error) (Result, error) {
	if !c.client.Connection.lockIsActive() {
		return nil, c.lockSetState(ChannelStateFailed, errAttach)
	}
	c.lockSetState(ChannelStateAttaching, err)
	var res Result
	if result {
		res = c.internalEmitter.listenResult(ChannelStateAttached, ChannelStateFailed)
	}
	msg := &proto.ProtocolMessage{
		Action:  proto.ActionAttach,
		Channel: c.Name,
	}
	err = c.client.Connection.send(msg, nil)
	if err != nil {
		return nil, c.lockSetState(ChannelStateFailed, err)
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

func (c *RealtimeChannel) detach(result bool) (Result, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	switch {
	case c.state == ChannelStateFailed:
		return nil, channelStateError(ChannelStateFailed, errDetach)
	case !c.isActive():
		return nopResult, nil
	}
	if !c.client.Connection.lockIsActive() {
		return nil, c.lockSetState(ChannelStateFailed, errDetach)
	}
	c.lockSetState(ChannelStateDetaching, nil)
	var res Result
	if result {
		res = c.internalEmitter.listenResult(ChannelStateDetached, ChannelStateFailed)
	}
	msg := &proto.ProtocolMessage{
		Action:  proto.ActionDetach,
		Channel: c.Name,
	}
	err := c.client.Connection.send(msg, nil)
	if err != nil {
		return nil, c.lockSetState(ChannelStateFailed, err)
	}
	return res, nil
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

type channelStateChanges chan ChannelStateChange

func (c channelStateChanges) Receive(change ChannelStateChange) {
	c <- change
}

type ChannelEventEmitter struct {
	emitter *eventEmitter
}

// On registers an event handler for connection events of a specific kind.
//
// See package-level documentation on Event Emitter for details.
func (em ChannelEventEmitter) On(e ChannelEvent, handle func(ChannelStateChange)) (off func()) {
	return em.emitter.On(e, func(change emitterData) {
		handle(change.(ChannelStateChange))
	})
}

// OnAll registers an event handler for all connection events.
//
// See package-level documentation on Event Emitter for details.
func (em ChannelEventEmitter) OnAll(handle func(ChannelStateChange)) (off func()) {
	return em.emitter.OnAll(func(change emitterData) {
		handle(change.(ChannelStateChange))
	})
}

// Once registers an one-off event handler for connection events of a specific kind.
//
// See package-level documentation on Event Emitter for details.
func (em ChannelEventEmitter) Once(e ChannelEvent, handle func(ChannelStateChange)) (off func()) {
	return em.emitter.On(e, func(change emitterData) {
		handle(change.(ChannelStateChange))
	})
}

// OnceAll registers an one-off event handler for all connection events.
//
// See package-level documentation on Event Emitter for details.
func (em ChannelEventEmitter) OnceAll(handle func(ChannelStateChange)) (off func()) {
	return em.emitter.OnceAll(func(change emitterData) {
		handle(change.(ChannelStateChange))
	})
}

// Off deregisters event handlers for connection events of a specific kind.
//
// See package-level documentation on Event Emitter for details.
func (em ChannelEventEmitter) Off(e ChannelEvent) {
	em.emitter.Off(e)
}

// Off deregisters all event handlers.
//
// See package-level documentation on Event Emitter for details.
func (em ChannelEventEmitter) OffAll() {
	em.emitter.OffAll()
}

// Publish publishes a message on the channel, which is send on separate
// goroutine. Publish does not block.
//
// This implicitly attaches the channel if it's not already attached.
func (c *RealtimeChannel) Publish(name string, data interface{}) (Result, error) {
	return c.PublishAll([]*proto.Message{{Name: name, Data: data}})
}

// PublishAll publishes all given messages on the channel at once.
// PublishAll does not block.
//
// This implicitly attaches the channel if it's not already attached.
func (c *RealtimeChannel) PublishAll(messages []*proto.Message) (Result, error) {
	id := c.client.Auth.clientIDForCheck()
	for _, v := range messages {
		if v.ClientID != "" && id != wildcardClientID && v.ClientID != id {
			// Spec RSL1g3,RSL1g4
			return nil, fmt.Errorf("Unable to publish message containing a clientId (%s) that is incompatible with the library clientId (%s)", v.ClientID, id)
		}
	}
	msg := &proto.ProtocolMessage{
		Action:   proto.ActionMessage,
		Channel:  c.Name,
		Messages: messages,
	}
	return c.send(msg)
}

// History gives the channel's message history according to the given parameters.
// The returned result can be inspected for the messages via the Messages()
// method.
func (c *RealtimeChannel) History(params *PaginateParams) (*PaginatedResult, error) {
	return c.client.rest.Channels.Get(c.Name).History(params)
}

func (c *RealtimeChannel) send(msg *proto.ProtocolMessage) (Result, error) {
	if _, err := c.attach(false); err != nil {
		return nil, err
	}
	res, listen := newErrResult()
	switch c.State() {
	case ChannelStateInitialized, ChannelStateAttaching:
		c.queue.Enqueue(msg, listen)
		return res, nil
	case ChannelStateAttached:
	default:
		return nil, &ErrorInfo{Code: 90001}
	}
	if err := c.client.Connection.send(msg, listen); err != nil {
		return nil, err
	}
	return res, nil
}

// State gives the current state of the channel.
func (c *RealtimeChannel) State() ChannelState {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.state
}

// Reason gives the last error that caused channel transition to failed state.
func (c *RealtimeChannel) Reason() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.errorReason
}

func (c *RealtimeChannel) notify(msg *proto.ProtocolMessage) {
	switch msg.Action {
	case proto.ActionAttached:
		c.Presence.onAttach(msg)
		c.setState(ChannelStateAttached, nil)
		c.queue.Flush()
	case proto.ActionDetached:
		c.mtx.Lock()

		err := error(newErrorProto(msg.Error))
		switch c.state {
		case ChannelStateDetaching:
			c.lockSetState(ChannelStateDetached, err)
			c.mtx.Unlock()
			return
		case ChannelStateAttached: // TODO: Also SUSPENDED; RTL13a
			var res Result
			res, err = c.lockAttach(true, err)
			if err != nil {
				break
			}

			c.mtx.Unlock()
			go func() {
				// We need to wait in another goroutine to allow more messages
				// to reach the connection.

				err = res.Wait()
				if err == nil {
					return
				}
				c.mtx.Lock()

				c.lockStartRetryAttachLoop(err)
			}()
			return
		case
			ChannelStateAttaching,
			ChannelStateDetached: // TODO: Should be SUSPENDED
		default:
			c.mtx.Unlock()
			return
		}

		c.lockStartRetryAttachLoop(err)
	case proto.ActionSync:
		c.Presence.processIncomingMessage(msg, syncSerial(msg))
	case proto.ActionPresence:
		c.Presence.processIncomingMessage(msg, "")
	case proto.ActionError:
		c.setState(ChannelStateFailed, newErrorProto(msg.Error))
		c.queue.Fail(newErrorProto(msg.Error))
	case proto.ActionMessage:
		c.subs.messageEnqueue(msg)
	default:
	}
}

func (c *RealtimeChannel) lockStartRetryAttachLoop(err error) {
	// TODO: Move to SUSPENDED; move it to DETACHED for now.
	c.lockSetState(ChannelStateDetached, err)
	c.mtx.Unlock()

	go func() {
		// TODO: The listener should be set before unlocking the state.
		// Will do once we have an EventEmitter.
		stateChange := make(channelStateChanges, 10)
		off := c.internalEmitter.OnAll(stateChange.Receive)
		defer off()

		for !c.retryAttach(stateChange) {
		}
	}()
}

func (c *RealtimeChannel) retryAttach(stateChange channelStateChanges) (done bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	select {
	case <-c.opts().After(ctx, c.opts().ChannelRetryTimeout):
	case <-stateChange:
		// Any concurrent state change cancels the retry.
		return true
	}

	if c.client.Connection.State() != ConnectionStateConnected {
		// RTL13c: If no longer CONNECTED, RTL3 takes over.
		return true
	}

	err := wait(c.mayAttach(true, false))
	if err == nil {
		return true
	}
	// TODO: Move to SUSPENDED; move it to DETACHED for now.
	c.setState(ChannelStateDetached, err)
	return false
}

func (c *RealtimeChannel) isActive() bool {
	return c.state == ChannelStateAttaching || c.state == ChannelStateAttached
}

func (c *RealtimeChannel) opts() *clientOptions {
	return c.client.opts()
}

func (c *RealtimeChannel) logger() *LoggerOptions {
	return c.client.logger()
}

func (c *RealtimeChannel) setState(state ChannelState, err error) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.lockSetState(state, err)
}

func (c *RealtimeChannel) lockSetState(state ChannelState, err error) error {
	previous := c.state
	changed := c.state != state
	c.state = state
	c.errorReason = channelStateError(state, err)
	change := ChannelStateChange{
		Current:  c.state,
		Previous: previous,
		Reason:   c.errorReason,
	}
	if !changed {
		change.Event = ChannelEventUpdated
	} else {
		change.Event = ChannelEvent(change.Current)
	}
	c.internalEmitter.emitter.Emit(change.Event, change)
	c.emitter.Emit(change.Event, change)
	if c.errorReason == nil {
		return nil
	}
	return c.errorReason
}
