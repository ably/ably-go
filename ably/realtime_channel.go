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
//
// The name of the channel is read-only - assigning new name to it does not
// have any effect.
type RealtimeChannel struct {
	Name string // read-only name of the channel

	client    *RealtimeClient
	state     *stateEmitter
	err       error
	listen    map[string]map[chan *proto.Message]struct{}
	listenMtx sync.Mutex
	queue     *msgQueue
}

func newRealtimeChannel(name string, client *RealtimeClient) *RealtimeChannel {
	c := &RealtimeChannel{
		Name:   name,
		client: client,
		state:  newStateEmitter(StateChan, StateChanInitialized, name),
		listen: make(map[string]map[chan *proto.Message]struct{}),
	}
	c.queue = newMsgQueue(client.Connection)
	if c.client.opts.Listener != nil {
		c.On(c.client.opts.Listener)
	}
	return c
}

var errAttach = newError(90000, errors.New("ably: Attach on inactive connection"))

// Attach initiates attach request, which is being processed on a separate
// goroutine.
//
// In order to receive its result register a channel with the On method; upon
// successful attachment the channel will receive StatChanAttached event.
//
// If channel is already attached, this method is a nop.
func (c *RealtimeChannel) Attach() error {
	c.state.Lock()
	defer c.state.Unlock()
	if c.isActive() {
		return nil
	}
	c.state.set(StateChanAttaching, nil)
	if !c.client.Connection.isActive() {
		return c.state.set(StateChanFailed, errAttach)
	}
	msg := &proto.ProtocolMessage{Action: proto.ActionAttach, Channel: c.state.channel}
	if err := c.client.Connection.send(msg, nil); err != nil {
		return c.state.set(StateChanFailed, newErrorf(90000, "ably: Attach error: %s", err))
	}
	return nil
}

var errDetach = newError(90000, errors.New("ably: Detach on invactive connection"))

// Detach initiates detach request, which is being processed on a separate
// goroutine.
//
// In order to receive its result register a channel with the On method; upon
// successful detachment the channel will receive StatChanDetached event.
//
// If channel is already detached, this method is a nop.
func (c *RealtimeChannel) Detach() error {
	c.state.Lock()
	defer c.state.Unlock()
	if !c.isActive() {
		return nil
	}
	c.state.set(StateChanDetaching, nil)
	if !c.client.Connection.isActive() {
		return c.state.set(StateChanFailed, errDetach)
	}
	msg := &proto.ProtocolMessage{Action: proto.ActionDetach, Channel: c.state.channel}
	if err := c.client.Connection.send(msg, nil); err != nil {
		c.state.set(StateChanFailed, newErrorf(90000, "ably: Detch error: %s", err))
	}
	return nil
}

func (c *RealtimeChannel) Close() error {
	return errors.New("TODO")
}

// Subscribe gives a channel, to which incoming messages that match given names
// are being dispatched.
//
// If no names are given, returned channel will receive all messages.
// If ch is nil, Subscribes creates new, unbuffered channel.
//
// The caller must ensure the messages are read from returned channel at
// sufficient pace, otherwise they may be dropped. In order to use buffered
// channel instead of the default unbuffered one, make new channel with requested
// capacity and pass it as ch; the Subscribe method will return it instead of
// creating new one.
//
// If ch is non-nil and it was already registered to receive messages with different
// names than the ones given, it will be added to receive also the new ones.
func (c *RealtimeChannel) Subscribe(ch chan *proto.Message, names ...string) (chan *proto.Message, error) {
	if err := c.Attach(); err != nil {
		return nil, err
	}
	if ch == nil {
		ch = make(chan *proto.Message)
	}
	if len(names) == 0 {
		names = []string{""}
	}
	c.listenMtx.Lock()
	for _, name := range names {
		l, ok := c.listen[name]
		if !ok {
			l = make(map[chan *proto.Message]struct{})
			c.listen[name] = l
		}
		l[ch] = struct{}{}
	}
	c.listenMtx.Unlock()
	return ch, nil
}

// Unsubscribe removes previously registered channel for the given message names.
func (c *RealtimeChannel) Unsubscribe(ch chan *proto.Message, names ...string) {
	if ch == nil {
		panic("ably: RealtimeChannel.Unsubscribe using nil channel")
	}
	if len(names) == 0 {
		names = []string{""}
	}
	c.listenMtx.Lock()
	for _, name := range names {
		delete(c.listen[name], ch)
		if len(c.listen[name]) == 0 {
			delete(c.listen, name)
		}
	}
	c.listenMtx.Unlock()
}

// On relays request channel states to c; on state transition
// connection will not block sending to c - the caller must ensure the incoming
// values are read at proper pace or the c is sufficiently buffered.
//
// If no states are given, c is registered for all of them.
// If c is nil, the method panics.
// If c is alreadt registered, its state set is expanded.
func (c *RealtimeChannel) On(ch chan<- State, states ...int) {
	c.state.on(ch, states...)
}

// Off removes c from listetning on the given channel state transitions.
//
// If no states are given, c is removed for all of the connection's states.
// If c is nil, the method panics.
func (c *RealtimeChannel) Off(ch chan<- State, states ...int) {
	c.state.off(ch, states...)
}

// Publish publishes a message on the channel, which is send on separate
// goroutine. Publish does not block.
//
// If listen channel is non-nil it will be used to notify whether the message
// was published succesfully with nil error value; when the operation failed,
// the error that caused the failure is going to be sent on the listen channel.
//
// This implicitly attaches the channel if it's not already attached.
func (c *RealtimeChannel) Publish(name string, data string, listen chan<- error) error {
	return c.PublishAll([]*proto.Message{{Name: name, Data: data}}, listen)
}

// PublishAll publishes all given messages on the channel at once.
// PublishAll does not block.
//
// If listen channel is non-nil it will be used to notify whether the messages
// were published succesfully with nil error value; when the operation failed,
// the error that caused the failure will be sent on the listen channel.
//
// This implicitly attaches the channel if it's not already attached.
func (c *RealtimeChannel) PublishAll(messages []*proto.Message, listen chan<- error) error {
	if err := c.Attach(); err != nil {
		return err
	}
	msg := &proto.ProtocolMessage{
		Action:   proto.ActionMessage,
		Channel:  c.Name,
		Messages: messages,
	}
	switch c.State() {
	case StateChanInitialized, StateChanAttaching:
		if c.client.opts.NoQueueing {
			return errQueueing
		}
		c.queue.Enqueue(msg, listen)
		return nil
	case StateChanAttached:
	default:
		return &Error{Code: 90001}
	}
	return c.client.Connection.send(msg, listen)
}

// State gives current state of the channel.
func (c *RealtimeChannel) State() int {
	c.state.Lock()
	state := c.state.current
	c.state.Unlock()
	return state
}

// Reason gives the last error that caused channel transition to failed state.
func (c *RealtimeChannel) Reason() error {
	c.state.Lock()
	err := c.state.err
	c.state.Unlock()
	return err
}

func (c *RealtimeChannel) notify(msg *proto.ProtocolMessage) {
	switch msg.Action {
	case proto.ActionAttached:
		c.state.syncSet(StateChanAttached, nil)
		c.queue.Flush()
	case proto.ActionDetached:
		c.state.syncSet(StateChanDetached, nil)
	case proto.ActionPresence:
		Log.Printf(LogError, "TODO: implement RealtimePresence: %v", msg)
	case proto.ActionError:
		c.state.syncSet(StateChanFailed, newErrorProto(msg.Error))
		c.queue.Fail(newErrorProto(msg.Error))
		// TODO recovery
	case proto.ActionMessage:
		c.listenMtx.Lock()
		for _, msg := range msg.Messages {
			if l, ok := c.listen[""]; ok {
				for ch := range l {
					select {
					case ch <- msg:
					default:
						Log.Printf(LogWarn, "dropped %q message due to slow receiver", msg.Name)
					}
				}
			}
			if l, ok := c.listen[msg.Name]; ok {
				for ch := range l {
					select {
					case ch <- msg:
					default:
						Log.Printf(LogWarn, "dropped %q message due to slow receiver", msg.Name)
					}
				}
			}
		}
		c.listenMtx.Unlock()
	default:
	}
}

func (c *RealtimeChannel) isActive() bool {
	return c.state.current == StateChanAttaching || c.state.current == StateChanAttached
}
