package ably

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

var (
	errConnAttach = func(connState ConnectionState) error {
		return fmt.Errorf("cannot Attach channel because connection is in %v state", connState)
	}
	errConnDetach = func(connState ConnectionState) error {
		return fmt.Errorf("cannot Detach channel because connection is in %v state", connState)
	}
	errChannelDetach = func(channelState ChannelState) error {
		return fmt.Errorf("cannot Detach channel because it is in %v state", channelState)
	}
	errDetach = errors.New("attempted to detach channel from inactive connection")
)

type chanSlice []*RealtimeChannel

func (ch chanSlice) Len() int           { return len(ch) }
func (ch chanSlice) Less(i, j int) bool { return ch[i].Name < ch[j].Name }
func (ch chanSlice) Swap(i, j int)      { ch[i], ch[j] = ch[j], ch[i] }
func (ch chanSlice) Sort()              { sort.Sort(ch) }

// RealtimeChannels is a goroutine-safe container for realtime channels that allows
// for creating, deleting and iterating over existing channels.
type RealtimeChannels struct {
	mtx    sync.Mutex
	client *Realtime
	chans  map[string]*RealtimeChannel
}

func newChannels(client *Realtime) *RealtimeChannels {
	return &RealtimeChannels{
		client: client,
		chans:  make(map[string]*RealtimeChannel),
	}
}

// A ChannelOption configures a channel.
type ChannelOption func(*channelOptions)

// channelOptions wraps ChannelOptions. It exists so that users can't
// implement their own ChannelOption.
type channelOptions protoChannelOptions

// CipherKey is like Cipher with an AES algorithm and CBC mode.
func ChannelWithCipherKey(key []byte) ChannelOption {
	return func(o *channelOptions) {
		o.Cipher = Crypto.GetDefaultParams(CipherParams{
			Key: key,
		})
	}
}

// Cipher sets cipher parameters for encrypting messages on a channel.
func ChannelWithCipher(params CipherParams) ChannelOption {
	return func(o *channelOptions) {
		o.Cipher = params
	}
}

func ChannelWithParams(key string, value string) ChannelOption {
	return func(o *channelOptions) {
		if o.Params == nil {
			o.Params = map[string]string{}
		}
		o.Params[key] = value
	}
}

func ChannelWithModes(modes ...ChannelMode) ChannelOption {
	return func(o *channelOptions) {
		o.Modes = append(o.Modes, modes...)
	}
}

func applyChannelOptions(os ...ChannelOption) *channelOptions {
	to := channelOptions{}
	for _, set := range os {
		set(&to)
	}
	return &to
}

// Get looks up a channel given by the name and creates it if it does not exist
// already.
//
// It is safe to call Get from multiple goroutines - a single channel is
// guaranteed to be created only once for multiple calls to Get from different
// goroutines.
func (ch *RealtimeChannels) Get(name string, options ...ChannelOption) *RealtimeChannel {
	// TODO: options
	ch.mtx.Lock()
	c, ok := ch.chans[name]
	if !ok {
		c = newRealtimeChannel(name, ch.client, applyChannelOptions(options...))
		ch.chans[name] = c
	}
	ch.mtx.Unlock()
	return c
}

// Iterate returns a list of created channels.
//
// It is safe to call Iterate from multiple goroutines, however there's no guarantee
// the returned list would not list a channel that was already released from
// different goroutine.
func (ch *RealtimeChannels) Iterate() []*RealtimeChannel { // RSN2, RTS2
	ch.mtx.Lock()
	chans := make([]*RealtimeChannel, 0, len(ch.chans))
	for _, c := range ch.chans {
		chans = append(chans, c)
	}
	ch.mtx.Unlock()
	return chans
}

// Exists returns true if the channel by the given name exists.
func (c *RealtimeChannels) Exists(name string) bool { // RSN2, RTS2
	c.mtx.Lock()
	_, ok := c.chans[name]
	c.mtx.Unlock()
	return ok
}

// Release releases all resources associated with a channel, detaching it first
// if necessary. See RealtimeChannel.Detach for details.
func (ch *RealtimeChannels) Release(ctx context.Context, name string) error {
	ch.mtx.Lock()
	defer ch.mtx.Unlock()
	c, ok := ch.chans[name]
	if !ok {
		return nil
	}
	err := c.Detach(ctx)
	if err != nil {
		return err
	}
	delete(ch.chans, name)
	return nil
}

func (ch *RealtimeChannels) broadcastConnStateChange(change ConnectionStateChange) {
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

	client         *Realtime
	messageEmitter *eventEmitter
	queue          *msgQueue
	options        *channelOptions
	params         channelParams
	modes          []ChannelMode

	//attachResume is True when the channel moves to the ChannelStateAttached state, and False
	//when the channel moves to the ChannelStateDetaching or ChannelStateFailed states.
	attachResume bool
}

func newRealtimeChannel(name string, client *Realtime, chOptions *channelOptions) *RealtimeChannel {
	c := &RealtimeChannel{
		ChannelEventEmitter: ChannelEventEmitter{newEventEmitter(client.log())},
		Name:                name,

		state:           ChannelStateInitialized,
		internalEmitter: ChannelEventEmitter{newEventEmitter(client.log())},

		client:         client,
		messageEmitter: newEventEmitter(client.log()),
		options:        chOptions,
	}
	c.Presence = newRealtimePresence(c)
	c.queue = newMsgQueue(client.Connection)
	return c
}

func (c *RealtimeChannel) onConnStateChange(change ConnectionStateChange) {
	switch change.Current {
	case ConnectionStateConnected:
		c.queue.Flush()
	case ConnectionStateFailed:
		c.setState(ChannelStateFailed, change.Reason, false)
		c.queue.Fail(change.Reason)
	}
}

// Attach attaches the Realtime connection to the channel, after which it starts
// receiving messages from it.
//
// If the context is canceled before the attach operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be attached anyway.
func (c *RealtimeChannel) Attach(ctx context.Context) error {
	res, err := c.attach()
	if err != nil {
		return err
	}

	attachTimeout := c.client.Connection.opts.realtimeRequestTimeout()
	timeoutCtx, cancel := c.opts().contextWithTimeout(context.Background(), attachTimeout)
	defer cancel()
	internalOpErr := make(chan error, 1)
	go func() {
		err := res.Wait(timeoutCtx)
		if errors.Is(err, context.DeadlineExceeded) {
			err = newError(ErrTimeoutError, errors.New("timed out before attaching channel"))
			c.setState(ChannelStateSuspended, err, false)
		}
		internalOpErr <- err
	}()
	select {
	case <-ctx.Done(): //Unblock channel wait, when external context is cancelled
		return ctx.Err()
	case err = <-internalOpErr:
		return err
	}
}

func (c *RealtimeChannel) attach() (result, error) {
	return c.mayAttach(true)
}

func (c *RealtimeChannel) mayAttach(checkActive bool) (result, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if checkActive {
		switch c.state {
		case ChannelStateAttaching:
			return c.internalEmitter.listenResult(ChannelStateAttached, ChannelStateDetached, ChannelStateSuspended, ChannelStateFailed), nil // RTL4h
		case ChannelStateAttached: // RTL4a
			return nopResult, nil
		}
	}
	return c.lockAttach(nil)
}

func (c *RealtimeChannel) lockAttach(err error) (result, error) {
	if c.state == ChannelStateFailed { // RTL4g
		err = nil
	}

	switch c.client.Connection.State() {
	// RTL4b
	case ConnectionStateInitialized,
		ConnectionStateClosed,
		ConnectionStateClosing,
		ConnectionStateSuspended,
		ConnectionStateFailed:
		return nil, newError(ErrConnectionFailed, errConnAttach(c.client.Connection.State()))
	}

	sendAttachMsg := func() (result, error) {
		res := c.internalEmitter.listenResult(ChannelStateAttached, ChannelStateDetached, ChannelStateSuspended, ChannelStateFailed) //RTL4d
		msg := &protocolMessage{
			Action:  actionAttach,
			Channel: c.Name,
		}
		if len(c.channelOpts().Params) > 0 {
			msg.Params = c.channelOpts().Params
		}
		if len(c.channelOpts().Modes) > 0 {
			msg.SetModesAsFlag(c.channelOpts().Modes)
		}
		if c.attachResume {
			msg.Flags.Set(flagAttachResume)
		}
		c.client.Connection.send(msg, nil)
		return res, nil
	}

	if c.state == ChannelStateDetaching { //// RTL4h
		attachRes := c.internalEmitter.listenResult(ChannelStateDetached, ChannelStateFailed)
		return resultFunc(func(ctx context.Context) error { // runs inside goroutine, need to check for locks again before accessing state
			detachErr := attachRes.Wait(ctx)
			if detachErr != nil && c.State() == ChannelStateFailed { // RTL4g - channel state is failed
				c.mtx.Lock()
				res, err := c.lockAttach(nil) // RTL4g - set error to nil
				c.mtx.Unlock()
				if err != nil {
					return err
				}
				return res.Wait(ctx)
			}
			c.setState(ChannelStateAttaching, err, false)
			res, _ := sendAttachMsg()
			return res.Wait(ctx)
		}), nil
	}
	c.lockSetState(ChannelStateAttaching, err, false)
	return sendAttachMsg()
}

// Detach detaches the Realtime connection to the channel, after which it stops
// receiving messages from it.
//
// If the context is canceled before the detach operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be detached anyway.
func (c *RealtimeChannel) Detach(ctx context.Context) error {
	prevChannelState := c.State()
	res, err := c.detach()
	if err != nil {
		return err
	}

	detachTimeout := c.client.Connection.opts.realtimeRequestTimeout()
	timeoutCtx, cancel := c.opts().contextWithTimeout(context.Background(), detachTimeout)
	defer cancel()
	internalOpErr := make(chan error, 1)
	go func() {
		err := res.Wait(timeoutCtx)
		if errors.Is(err, context.DeadlineExceeded) {
			err = newError(ErrTimeoutError, errors.New("timed out before detaching channel"))
			c.setState(prevChannelState, err, false)
		}
		internalOpErr <- err
	}()
	select {
	case <-ctx.Done(): //Unblock channel wait, when external context is cancelled
		return ctx.Err()
	case err = <-internalOpErr:
		return err
	}
}

func (c *RealtimeChannel) detach() (result, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.client.Connection.State() == ConnectionStateClosing || c.client.Connection.State() == ConnectionStateFailed { // RTL5g
		return nil, connStateError(c.client.Connection.State(), errConnDetach(c.client.Connection.State()))
	}
	if c.state == ChannelStateInitialized || c.state == ChannelStateDetached { //RTL5a
		return nopResult, nil
	}
	if c.state == ChannelStateFailed { // RTL5b
		return nil, channelStateError(ChannelStateFailed, errChannelDetach(c.state))
	}
	if c.state == ChannelStateSuspended { // RTL5j
		c.lockSetState(ChannelStateDetached, nil, false)
		return nopResult, nil
	}
	if c.state == ChannelStateDetaching { // RTL5i
		return c.internalEmitter.listenResult(ChannelStateDetached, ChannelStateFailed), nil
	}
	if c.state == ChannelStateAttaching { //RTL5i
		attachRes := c.internalEmitter.listenResult(ChannelStateAttached, ChannelStateDetached, ChannelStateSuspended, ChannelStateFailed) //RTL4d
		return resultFunc(func(ctx context.Context) error {                                                                                // runs inside goroutine, need to check for locks again before accessing state
			err := attachRes.Wait(ctx) // error can be suspended or failed, so send back error right away
			if err != nil {
				return err
			}
			c.setState(ChannelStateDetaching, nil, false)
			res, _ := c.sendDetachMsg()
			return res.Wait(ctx)
		}), nil
	}
	return c.detachUnsafe()
}

func (c *RealtimeChannel) detachSkipVerifyActive() (result, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.detachUnsafe()
}

func (c *RealtimeChannel) sendDetachMsg() (result, error) {
	res := c.internalEmitter.listenResult(ChannelStateDetached, ChannelStateFailed)
	msg := &protocolMessage{
		Action:  actionDetach,
		Channel: c.Name,
	}
	c.client.Connection.send(msg, nil)
	return res, nil
}

func (c *RealtimeChannel) detachUnsafe() (result, error) {
	c.lockSetState(ChannelStateDetaching, nil, false) // no need to check for locks, method is already under lock context
	return c.sendDetachMsg()
}

type subscriptionName string

func (subscriptionName) isEmitterEvent() {}

type subscriptionMessage Message

func (*subscriptionMessage) isEmitterData() {}

// Subscribe registers a message handler to be called with each message with the
// given name received from the channel.
//
// This implicitly attaches the channel if it's not already attached. If the
// context is canceled before the attach operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be attached anyway.
//
// See package-level documentation on Event Emitter for details about
// messages dispatch.
func (c *RealtimeChannel) Subscribe(ctx context.Context, name string, handle func(*Message)) (unsubscribe func(), err error) {
	res, err := c.attach()
	if err != nil {
		return nil, err
	}
	err = res.Wait(ctx)
	if err != nil {
		return nil, err
	}
	return c.messageEmitter.On(subscriptionName(name), func(message emitterData) {
		handle((*Message)(message.(*subscriptionMessage)))
	}), nil
}

// SubscribeAll register a message handler to be called with each message
// received from the channel.
//
// This implicitly attaches the channel if it's not already attached. If the
// context is canceled before the attach operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be attached anyway.
//
// See package-level documentation on Event Emitter for details about
// messages dispatch.
func (c *RealtimeChannel) SubscribeAll(ctx context.Context, handle func(*Message)) (unsubscribe func(), err error) {
	res, err := c.attach()
	if err != nil {
		return nil, err
	}
	err = res.Wait(ctx)
	if err != nil {
		return nil, err
	}
	return c.messageEmitter.OnAll(func(message emitterData) {
		handle((*Message)(message.(*subscriptionMessage)))
	}), nil
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
	return em.emitter.Once(e, func(change emitterData) {
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

// OffAll de-registers all event handlers.
//
// See package-level documentation on Event Emitter for details.
func (em ChannelEventEmitter) OffAll() {
	em.emitter.OffAll()
}

// Publish publishes a message on the channel.
//
// This implicitly attaches the channel if it's not already attached. If the
// context is canceled before the attach operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be attached and the message published anyway.
func (c *RealtimeChannel) Publish(ctx context.Context, name string, data interface{}) error {
	return c.PublishMultiple(ctx, []*Message{{Name: name, Data: data}})
}

// PublishMultiple publishes all given messages on the channel at once.
//
// This implicitly attaches the channel if it's not already attached. If the
// context is canceled before the attach operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be attached and the message published anyway.
func (c *RealtimeChannel) PublishMultiple(ctx context.Context, messages []*Message) error {
	id := c.client.Auth.clientIDForCheck()
	for _, v := range messages {
		if v.ClientID != "" && id != wildcardClientID && v.ClientID != id {
			// Spec RSL1g3,RSL1g4
			return fmt.Errorf("Unable to publish message containing a clientId (%s) that is incompatible with the library clientId (%s)", v.ClientID, id)
		}
	}
	msg := &protocolMessage{
		Action:   actionMessage,
		Channel:  c.Name,
		Messages: messages,
	}
	res, err := c.send(msg)
	if err != nil {
		return err
	}
	return res.Wait(ctx)
}

// History is equivalent to RESTChannel.History.
func (c *RealtimeChannel) History(o ...HistoryOption) HistoryRequest {
	return c.client.rest.Channels.Get(c.Name).History(o...)
}

func (c *RealtimeChannel) send(msg *protocolMessage) (result, error) {
	if res, enqueued := c.maybeEnqueue(msg); enqueued {
		return res, nil
	}

	if !c.canSend() {
		return nil, newError(ErrChannelOperationFailedInvalidChannelState, nil)
	}

	res, listen := newErrResult()
	c.client.Connection.send(msg, listen)
	return res, nil
}

func (c *RealtimeChannel) maybeEnqueue(msg *protocolMessage) (_ result, enqueued bool) {
	// RTL6c2
	if c.opts().NoQueueing {
		return nil, false
	}
	switch c.client.Connection.State() {
	default:
		return nil, false
	case ConnectionStateInitialized,
		ConnectionStateConnecting,
		ConnectionStateDisconnected:
	}
	switch c.State() {
	default:
		return nil, false
	case ChannelStateInitialized,
		ChannelStateAttached,
		ChannelStateDetached,
		ChannelStateAttaching,
		ChannelStateDetaching:
	}

	res, listen := newErrResult()
	c.queue.Enqueue(msg, listen)
	return res, true
}

func (c *RealtimeChannel) canSend() bool {
	// RTL6c1
	if c.client.Connection.State() != ConnectionStateConnected {
		return false
	}
	switch c.State() {
	default:
		return false
	case ChannelStateInitialized,
		ChannelStateAttached,
		ChannelStateDetached,
		ChannelStateAttaching,
		ChannelStateDetaching:
	}
	return true
}

// State gives the current state of the channel.
func (c *RealtimeChannel) State() ChannelState {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.state
}

// ErrorReason gives the last error that caused channel transition to failed state.
func (c *RealtimeChannel) ErrorReason() *ErrorInfo {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.errorReason
}

func (c *RealtimeChannel) notify(msg *protocolMessage) {
	switch msg.Action {
	case actionAttached:
		if c.State() == ChannelStateDetaching { // RTL5K
			c.sendDetachMsg()
			return
		}
		if len(msg.Params) > 0 {
			c.setParams(msg.Params)
		}
		if msg.Flags != 0 {
			c.setModes(channelModeFromFlag(msg.Flags))
		}
		c.Presence.onAttach(msg)
		// RTL12
		c.setState(ChannelStateAttached, newErrorFromProto(msg.Error), msg.Flags.Has(flagResumed))
		c.queue.Flush()
	case actionDetached:
		c.mtx.Lock()
		err := error(newErrorFromProto(msg.Error))
		switch c.state {
		case ChannelStateDetaching:
			c.lockSetState(ChannelStateDetached, err, false)
			c.mtx.Unlock()
			return
		case ChannelStateAttached: // TODO: Also SUSPENDED; RTL13a
			var res result
			res, err = c.lockAttach(err)
			if err != nil {
				break
			}

			c.mtx.Unlock()
			go func() {
				// We need to wait in another goroutine to allow more messages
				// to reach the connection.

				err = res.Wait(context.Background())
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
	case actionSync:
		c.Presence.processIncomingMessage(msg, syncSerial(msg))
	case actionPresence:
		c.Presence.processIncomingMessage(msg, "")
	case actionError:
		c.setState(ChannelStateFailed, newErrorFromProto(msg.Error), false)
		c.queue.Fail(newErrorFromProto(msg.Error))
	case actionMessage:
		if c.State() == ChannelStateAttached {
			for _, msg := range msg.Messages {
				c.messageEmitter.Emit(subscriptionName(msg.Name), (*subscriptionMessage)(msg))
			}
		}
	default:
	}
}

func (c *RealtimeChannel) lockStartRetryAttachLoop(err error) {
	// TODO: Move to SUSPENDED; move it to DETACHED for now.
	c.lockSetState(ChannelStateDetached, err, false)

	stateChange := make(channelStateChanges, 1)
	off := c.internalEmitter.OnceAll(stateChange.Receive)

	c.mtx.Unlock()

	go func() {
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

	err := wait(ctx)(c.mayAttach(false))
	if err == nil {
		return true
	}
	// TODO: Move to SUSPENDED; move it to DETACHED for now.
	c.setState(ChannelStateDetached, err, false)
	return false
}

func (c *RealtimeChannel) isActive() bool {
	return c.state == ChannelStateAttaching || c.state == ChannelStateAttached
}

func (c *RealtimeChannel) channelOpts() *channelOptions {
	return c.options
}

func (c *RealtimeChannel) setParams(params channelParams) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.params = params
}

func (c *RealtimeChannel) setModes(modes []ChannelMode) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.modes = modes
}

func (c *RealtimeChannel) Modes() []ChannelMode {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	var modes []ChannelMode
	modes = append(modes, c.modes...)
	return modes
}

func (c *RealtimeChannel) Params() map[string]string {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	params := make(channelParams)
	for key, value := range c.params {
		params[key] = value
	}
	return params
}

func (c *RealtimeChannel) opts() *clientOptions {
	return c.client.opts()
}

func (c *RealtimeChannel) log() logger {
	return c.client.log()
}

func (c *RealtimeChannel) setState(state ChannelState, err error, resumed bool) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.lockSetState(state, err, resumed)
}

func (c *RealtimeChannel) lockSetAttachResume(state ChannelState) {
	if state == ChannelStateDetaching {
		c.attachResume = false
	}
	if state == ChannelStateAttached {
		c.attachResume = true
	}
	if state == ChannelStateFailed {
		c.attachResume = false
	}
}

func (c *RealtimeChannel) lockSetState(state ChannelState, err error, resumed bool) error {
	c.lockSetAttachResume(state)
	previous := c.state
	changed := c.state != state
	c.state = state
	c.errorReason = channelStateError(state, err)
	change := ChannelStateChange{
		Current:  c.state,
		Previous: previous,
		Reason:   c.errorReason,
		Resumed:  resumed,
	}
	// RTL2g
	if !changed {
		change.Event = ChannelEventUpdate
	} else {
		change.Event = ChannelEvent(change.Current)
	}
	c.internalEmitter.emitter.Emit(change.Event, change)
	c.emitter.Emit(change.Event, change)
	return c.errorReason.unwrapNil()
}
