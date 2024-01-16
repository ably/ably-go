package ably

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/ably/ably-go/ably/internal/ablyutil"
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

// RealtimeChannels is a goroutine-safe container for realtime channels that allows for creating,
// deleting and iterating over existing channels.
// Creates and destroys [ably.RESTChannel] and [ably.RealtimeChannel] objects.
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

// RTN16j, RTL15b
func (channels *RealtimeChannels) SetChannelSerialsFromRecoverOption(serials map[string]string) {
	for channelName, channelSerial := range serials {
		channel := channels.Get(channelName)
		channel.setChannelSerial(channelSerial)
	}
}

func (channels *RealtimeChannels) GetChannelSerials() map[string]string {
	channels.mtx.Lock()
	defer channels.mtx.Unlock()
	channelSerials := make(map[string]string)
	for channelName, realtimeChannel := range channels.chans {
		if realtimeChannel.State() == ChannelStateAttached {
			channelSerials[channelName] = realtimeChannel.getChannelSerial()
		}
	}
	return channelSerials
}

// ChannelOption configures a channel.
type ChannelOption func(*channelOptions)

// channelOptions wraps ChannelOptions. It exists so that users can't implement their own ChannelOption.
type channelOptions protoChannelOptions

// DeriveOptions allows options to be used in creating a derived channel
type DeriveOptions struct {
	Filter string
}

// ChannelWithCipherKey is a constructor that takes private key as a argument.
// It is used to encrypt and decrypt payloads (TB3)
func ChannelWithCipherKey(key []byte) ChannelOption {
	return func(o *channelOptions) {
		o.Cipher = Crypto.GetDefaultParams(CipherParams{
			Key: key,
		})
	}
}

// ChannelWithCipher requests encryption for this channel when not null, and specifies encryption-related parameters
// (such as algorithm, chaining mode, key length and key). See [an example] (RSL5a, TB2b).
//
// [an example]: https://ably.com/docs/realtime/encryption#getting-started
func ChannelWithCipher(params CipherParams) ChannelOption {
	return func(o *channelOptions) {
		o.Cipher = params
	}
}

// ChannelWithParams sets channel parameters that configure the behavior of the channel (TB2c).
func ChannelWithParams(key string, value string) ChannelOption {
	return func(o *channelOptions) {
		if o.Params == nil {
			o.Params = map[string]string{}
		}
		o.Params[key] = value
	}
}

// ChannelWithModes set an array of [ably.ChannelMode] to a channel (TB2d).
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

// Get creates a new [ably.RealtimeChannel] object for given channel name and provided [ably.ChannelOption] or
// returns the existing channel if already created with given channel name.
// Creating a channel only adds a new channel struct into the channel map. It does not
// perform any network communication until attached.
// It is safe to call Get from multiple goroutines - a single channel is
// guaranteed to be created only once for multiple calls to Get from different
// goroutines (RSN3a, RTS3a, RSN3c, RTS3c).
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

// GetDerived is a preview feature and may change in a future non-major release.
// It creates a new derived [ably.RealtimeChannel] object for given channel name, using the provided derive options and
// channel options if any. Returns error if any occurs.
func (ch *RealtimeChannels) GetDerived(name string, deriveOptions DeriveOptions, options ...ChannelOption) (*RealtimeChannel, error) {
	if deriveOptions.Filter != "" {
		match, err := ablyutil.MatchDerivedChannel(name)
		if err != nil {
			return nil, newError(40010, err)
		}
		filter := base64.StdEncoding.EncodeToString([]byte(deriveOptions.Filter))
		name = fmt.Sprintf("[filter=%s%s]%s", filter, match.QualifierParam, match.ChannelName)
	}
	return ch.Get(name, options...), nil
}

// Iterate returns a [ably.RealtimeChannel] for each iteration on existing channels.
// It is safe to call Iterate from multiple goroutines, however there's no guarantee
// the returned list would not list a channel that was already released from
// different goroutine (RSN2, RTS2).
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
//
// This function just checks the local channel map for existence. It can not check
// for the existence of channels created by other clients (RSN2, RTS2).
func (c *RealtimeChannels) Exists(name string) bool {
	c.mtx.Lock()
	_, ok := c.chans[name]
	c.mtx.Unlock()
	return ok
}

// Release releases a [ably.RealtimeChannel] object with given channel name (detaching it first), frees all
// resources associated, e.g. Removes any listeners associated with the channel.
// To release a channel, the [ably.ChannelState] must be INITIALIZED, DETACHED, or FAILED (RSN4, RTS4).
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

// RealtimeChannel represents a single named message/presence channel.
// It enables messages to be published and subscribed to. Also enables historic messages to be retrieved and
// provides access to the [ably.RealtimePresence] object of a channel.
type RealtimeChannel struct {
	mtx sync.Mutex

	// RealtimeChannel implements [ably.EventEmitter] and emits [ably.ChannelEvent] events, where a ChannelEvent is either
	// a [ably.ChannelState] or [ably.ChannelEventUpdate] (RTL2a, RTL2d, RTL2e).
	ChannelEventEmitter

	// Name is the channel name.
	Name string

	// Presence is a [ably.RealtimePresence] object, provides for entering and leaving client presence (RTL9).
	Presence *RealtimePresence

	// state is the current [ably.ChannelState] of the channel (RTL2b).
	state ChannelState

	//errorReason is a [ably.ErrorInfo] object describing the last error which occurred on the channel, if any (RTL4e).
	errorReason *ErrorInfo

	internalEmitter ChannelEventEmitter

	client         *Realtime
	messageEmitter *eventEmitter
	errorEmitter   *eventEmitter
	queue          *msgQueue
	options        *channelOptions

	// params are optional channel parameters that configure the behavior of the channel (RTL4k1).
	params channelParams

	// modes is an array of multiple [ably.ChannelMode] objects (RTL4m).
	modes []ChannelMode

	//attachResume is True when the channel moves to the ChannelStateAttached state, and False
	//when the channel moves to the ChannelStateDetaching or ChannelStateFailed states.
	attachResume bool

	properties ChannelProperties
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
		properties:     ChannelProperties{},
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

// Attach attaches the Realtime connection to the channel (ensuring the channel is created in the Ably system and all
// messages published on the channel are received by any channel listeners registered using RealtimeChannel#subscribe.)
// Any resulting channel state change will be emitted to any listeners registered using the EventEmitter#on or
// EventEmitter#once methods. A callback may optionally be passed in to this call to be notified of success or
// failure of the operation. As a convenience, attach() is called implicitly if RealtimeChannel#subscribe
// for the channel is called, or RealtimePresence#enter or RealtimePresence#subscribe are called on the
// [ably.RealtimePresence] object for this channel (RTL4d).
//
// If the passed context is cancelled before the attach operation finishes, the call returns with an error,
// but the operation carries on in the background and the channel may eventually be attached anyway.
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
		msg.ChannelSerial = c.properties.ChannelSerial // RTL4c1, accessing locked
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

// Detach detaches realtime connection to the channel, after which it stops receiving messages from it.
// Any resulting channel state change is emitted to any listeners registered using
// the EventEmitter#on or EventEmitter#once methods. A callback may optionally be passed in to this call to be
// notified of success or failure of the operation. Once all clients globally have
// detached from the channel, the channel will be released in the Ably service within two minutes (RTL5e).
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

// Subscribe registers an event listener for messages with a given event name on this channel.
// The caller supplies a listener function, which is called each time one or more matching messages
// arrives on the channel. A callback may optionally be passed in to this call to be notified of success
// or failure of the channel realtimeChannel#attach operation (RTL7a).
//
// This implicitly attaches the channel if it's not already attached. If the
// context is canceled before the attach operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be attached anyway.
//
// See package-level documentation => [ably] Event Emitters for details about messages dispatch.
func (c *RealtimeChannel) Subscribe(ctx context.Context, name string, handle func(*Message)) (func(), error) {

	// unsubscribe deregisters the given listener for the specified event name.
	// This removes an earlier event-specific subscription (RTL8a)
	unsubscribe := c.messageEmitter.On(subscriptionName(name), func(message emitterData) {
		handle((*Message)(message.(*subscriptionMessage)))
	})
	res, err := c.attach()
	if err != nil {
		unsubscribe()
		return nil, err
	}
	err = res.Wait(ctx)
	if err != nil {
		unsubscribe()
		return nil, err
	}
	return unsubscribe, nil
}

// SubscribeAll registers an event listener for messages on this channel. The caller supplies a listener function,
// which is called each time one or more messages arrives on the channel. A callback may optionally be passed
// in to this call to be notified of success or failure of the channel RealtimeChannel#attach operation (RTL7a).
//
// This implicitly attaches the channel if it's not already attached. If the
// context is canceled before the attach operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be attached anyway.
//
// See package-level documentation => [ably] Event Emitters for details about messages dispatch.
func (c *RealtimeChannel) SubscribeAll(ctx context.Context, handle func(*Message)) (func(), error) {
	// unsubscribe deregisters all listeners to messages on this channel.
	// This removes all earlier subscriptions (RTL8a, RTE5).
	unsubscribe := c.messageEmitter.OnAll(func(message emitterData) {
		handle((*Message)(message.(*subscriptionMessage)))
	})
	res, err := c.attach()
	if err != nil {
		unsubscribe()
		return nil, err
	}
	err = res.Wait(ctx)
	if err != nil {
		unsubscribe()
		return nil, err
	}
	return unsubscribe, nil
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
// See package-level documentation => [ably] Event Emitters for details about messages dispatch.
func (em ChannelEventEmitter) On(e ChannelEvent, handle func(ChannelStateChange)) (off func()) {
	return em.emitter.On(e, func(change emitterData) {
		handle(change.(ChannelStateChange))
	})
}

// OnAll registers an event handler for all connection events.
//
// See package-level documentation => [ably] Event Emitters for details about messages dispatch.
func (em ChannelEventEmitter) OnAll(handle func(ChannelStateChange)) (off func()) {
	return em.emitter.OnAll(func(change emitterData) {
		handle(change.(ChannelStateChange))
	})
}

// Once registers an one-off event handler for connection events of a specific kind.
//
// See package-level documentation => [ably] Event Emitters for details about messages dispatch.
func (em ChannelEventEmitter) Once(e ChannelEvent, handle func(ChannelStateChange)) (off func()) {
	return em.emitter.Once(e, func(change emitterData) {
		handle(change.(ChannelStateChange))
	})
}

// OnceAll registers an one-off event handler for all connection events.
//
// See package-level documentation => [ably] Event Emitters for details about messages dispatch.
func (em ChannelEventEmitter) OnceAll(handle func(ChannelStateChange)) (off func()) {
	return em.emitter.OnceAll(func(change emitterData) {
		handle(change.(ChannelStateChange))
	})
}

// Off deregisters event handlers for connection events of a specific kind.
//
// See package-level documentation => [ably] Event Emitters for details about messages dispatch.
func (em ChannelEventEmitter) Off(e ChannelEvent) {
	em.emitter.Off(e)
}

// OffAll de-registers all event handlers.
//
// See package-level documentation => [ably] Event Emitters for details about messages dispatch.
func (em ChannelEventEmitter) OffAll() {
	em.emitter.OffAll()
}

// Publish publishes a single message to the channel with the given event name and message payload.
//
// This will block until either the publish is acknowledged or fails to deliver.
//
// If the context is cancelled before the attach operation finishes, the call
// returns an error but the publish will carry on in the background and may
// eventually be published anyway.
func (c *RealtimeChannel) Publish(ctx context.Context, name string, data interface{}) error {
	return c.PublishMultiple(ctx, []*Message{{Name: name, Data: data}})
}

// PublishAsync is the same as Publish except instead of blocking it calls onAck
// with nil if the publish was successful or the appropriate error.
//
// Note onAck must not block as it would block the internal client.
func (c *RealtimeChannel) PublishAsync(name string, data interface{}, onAck func(err error)) error {
	return c.PublishMultipleAsync([]*Message{{Name: name, Data: data}}, onAck)
}

// PublishMultiple publishes all given messages on the channel at once.
//
// If the context is cancelled before the attach operation finishes, the call
// returns an error but the publish will carry on in the background and may
// eventually be published anyway.
func (c *RealtimeChannel) PublishMultiple(ctx context.Context, messages []*Message) error {
	listen := make(chan error, 1)
	onAck := func(err error) {
		listen <- err
	}
	if err := c.PublishMultipleAsync(messages, onAck); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-listen:
		return err
	}
}

// PublishMultipleAsync is the same as PublishMultiple except it calls onAck instead of blocking
// (see PublishAsync).
func (c *RealtimeChannel) PublishMultipleAsync(messages []*Message, onAck func(err error)) error {
	id := c.client.Auth.clientIDForCheck()
	for _, v := range messages {
		if v.ClientID != "" && id != wildcardClientID && v.ClientID != id {
			// Spec RTL6g3,RTL6g4
			return fmt.Errorf("Unable to publish message containing a clientId (%s) that is incompatible with the library clientId (%s)", v.ClientID, id)
		}
	}
	msg := &protocolMessage{
		Action:   actionMessage,
		Channel:  c.Name,
		Messages: messages,
	}
	return c.send(msg, onAck)
}

// History retrieves a [ably.HistoryRequest] object, containing an array of historical
// [ably.Message] objects for the channel. If the channel is configured to persist messages,
// then messages can be retrieved from history for up to 72 hours in the past. If not, messages can only be
// retrieved from history for up to two minutes in the past.
//
// See package-level documentation => [ably] Pagination for details about history pagination.
func (c *RealtimeChannel) History(o ...HistoryOption) HistoryRequest {
	return c.client.rest.Channels.Get(c.Name).History(o...)
}

// HistoryUntilAttach retrieves a [ably.HistoryRequest] object, containing an array of historical
// [ably.Message] objects for the channel. If the channel is configured to persist messages,
// then messages can be retrieved from history for up to 72 hours in the past. If not, messages can only be
// retrieved from history for up to two minutes in the past.
//
// This function will only retrieve messages prior to the moment that the channel was attached or emitted an UPDATE
// indicating loss of continuity. This bound is specified by passing the querystring param fromSerial with the RealtimeChannel#properties.attachSerial
// assigned to the channel in the ATTACHED ProtocolMessage (see RTL15a).
// If the untilAttach param is specified when the channel is not attached, it results in an error.
//
// See package-level documentation => [ably] Pagination for details about history pagination.
func (c *RealtimeChannel) HistoryUntilAttach(o ...HistoryOption) (*HistoryRequest, error) {
	if c.state != ChannelStateAttached {
		return nil, errors.New("channel is not attached, cannot use attachSerial value in fromSerial param")
	}

	untilAttachParam := func(o *historyOptions) {
		c.mtx.Lock()
		o.params.Set("fromSerial", c.properties.AttachSerial)
		c.mtx.Unlock()
	}
	o = append(o, untilAttachParam)

	historyRequest := c.client.rest.Channels.Get(c.Name).History(o...)
	return &historyRequest, nil
}

func (c *RealtimeChannel) send(msg *protocolMessage, onAck func(err error)) error {
	if enqueued := c.maybeEnqueue(msg, onAck); enqueued {
		return nil
	}

	if !c.canSend() {
		return newError(ErrChannelOperationFailedInvalidChannelState, nil)
	}

	c.client.Connection.send(msg, onAck)
	return nil
}

func (c *RealtimeChannel) maybeEnqueue(msg *protocolMessage, onAck func(err error)) bool {
	// RTL6c2
	if c.opts().NoQueueing {
		return false
	}
	switch c.client.Connection.State() {
	default:
		return false
	case ConnectionStateInitialized,
		ConnectionStateConnecting,
		ConnectionStateDisconnected:
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

	c.queue.Enqueue(msg, onAck)
	return true
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
	// RTL15b
	if !empty(msg.ChannelSerial) && (msg.Action == actionMessage ||
		msg.Action == actionPresence || msg.Action == actionAttached) {
		c.log().Debugf("Setting channel serial for channelName - %v, previous - %v, current - %v",
			c.Name, c.getChannelSerial(), msg.ChannelSerial)
		c.setChannelSerial(msg.ChannelSerial)
	}

	switch msg.Action {
	case actionAttached:
		c.mtx.Lock()
		c.properties.AttachSerial = msg.ChannelSerial // RTL15a
		c.mtx.Unlock()
		if c.State() == ChannelStateDetaching || c.State() == ChannelStateDetached { // RTL5K
			c.sendDetachMsg()
			return
		}
		if len(msg.Params) > 0 {
			c.setParams(msg.Params)
		}
		if msg.Flags != 0 {
			c.setModes(channelModeFromFlag(msg.Flags))
		}

		if c.State() == ChannelStateAttached {
			if !msg.Flags.Has(flagResumed) { // RTL12
				c.Presence.onAttach(msg)
				c.emitErrorUpdate(newErrorFromProto(msg.Error), false)
			}
		} else {
			c.Presence.onAttach(msg)
			c.setState(ChannelStateAttached, newErrorFromProto(msg.Error), msg.Flags.Has(flagResumed))
		}
		c.queue.Flush()
	case actionDetached:
		c.mtx.Lock()
		err := error(newErrorFromProto(msg.Error))
		switch c.state {
		case ChannelStateDetaching:
			c.lockSetState(ChannelStateDetached, err, false)
			c.mtx.Unlock()
			return
		case ChannelStateAttached, ChannelStateSuspended: // RTL13a
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
		c.Presence.processProtoSyncMessage(msg) // RTP18
	case actionPresence:
		c.Presence.processProtoPresenceMessage(msg)
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

func (c *RealtimeChannel) setChannelSerial(serial string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.properties.ChannelSerial = serial
}

func (c *RealtimeChannel) getChannelSerial() string {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.properties.ChannelSerial
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

	// RTP5a
	if state == ChannelStateDetached || state == ChannelStateFailed {
		c.Presence.onChannelDetachedOrFailed(channelStateError(state, err))
	}
	// RTP5a1
	if state == ChannelStateDetached || state == ChannelStateSuspended || state == ChannelStateFailed {
		c.properties.ChannelSerial = "" // setting on already locked method
	}
	// RTP5f
	if state == ChannelStateSuspended {
		c.Presence.onChannelSuspended(channelStateError(state, err))
	}

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

func (c *RealtimeChannel) emitErrorUpdate(err *ErrorInfo, resumed bool) {
	change := ChannelStateChange{
		Current:  c.state,
		Previous: c.state,
		Reason:   err,
		Resumed:  resumed,
		Event:    ChannelEventUpdate,
	}
	c.emitter.Emit(change.Event, change)
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
	// RTL2g, RTL12
	if !changed {
		change.Event = ChannelEventUpdate
	} else {
		change.Event = ChannelEvent(change.Current)
	}
	c.internalEmitter.emitter.Emit(change.Event, change)
	c.emitter.Emit(change.Event, change)
	return c.errorReason.unwrapNil()
}
