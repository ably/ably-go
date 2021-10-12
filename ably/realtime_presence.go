package ably

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type syncState uint8

const (
	syncInitial syncState = iota + 1
	syncInProgress
	syncComplete
)

// RealtimePresence represents a single presence map of a particular channel.
// It allows entering, leaving and updating presence state for the current
// client or on behalf of other client.
type RealtimePresence struct {
	mtx            sync.Mutex
	data           interface{}
	serial         string
	messageEmitter *eventEmitter
	channel        *RealtimeChannel
	members        map[string]*PresenceMessage
	stale          map[string]struct{}
	state          PresenceAction
	syncMtx        sync.Mutex
	syncState      syncState
}

func newRealtimePresence(channel *RealtimeChannel) *RealtimePresence {
	pres := &RealtimePresence{
		messageEmitter: newEventEmitter(channel.log()),
		channel:        channel,
		members:        make(map[string]*PresenceMessage),
		syncState:      syncInitial,
	}
	// Lock syncMtx to make all callers to Get(true) wait until the presence
	// is in initial sync state. This is to not make them early return
	// with an empty presence list before channel attaches.
	pres.syncMtx.Lock()
	return pres
}

func (pres *RealtimePresence) verifyChanState() error {
	switch state := pres.channel.State(); state {
	case ChannelStateDetached, ChannelStateDetaching, ChannelStateFailed:
		return newError(91001, fmt.Errorf("unable to enter presence channel (invalid channel state: %s)", state.String()))
	default:
		return nil
	}
}

func (pres *RealtimePresence) send(msg *PresenceMessage) (result, error) {
	attached, err := pres.channel.attach()
	if err != nil {
		return nil, err
	}
	if err := pres.verifyChanState(); err != nil {
		return nil, err
	}
	protomsg := &protocolMessage{
		Action:   actionPresence,
		Channel:  pres.channel.Name,
		Presence: []*PresenceMessage{msg},
	}
	return resultFunc(func(ctx context.Context) error {
		err := attached.Wait(ctx)
		if err != nil {
			return err
		}
		return wait(ctx)(pres.channel.send(protomsg))
	}), nil
}

func (pres *RealtimePresence) syncWait() {
	// If there's an undergoing sync operation or we wait till channel gets
	// attached, the following lock is going to block until the operations
	// complete.
	pres.syncMtx.Lock()
	pres.syncMtx.Unlock()
}

func syncSerial(msg *protocolMessage) string {
	if i := strings.IndexRune(msg.ChannelSerial, ':'); i != -1 {
		return msg.ChannelSerial[i+1:]
	}
	return ""
}

func (pres *RealtimePresence) onAttach(msg *protocolMessage) {
	serial := syncSerial(msg)
	pres.mtx.Lock()
	defer pres.mtx.Unlock()
	switch {
	case msg.Flags.Has(flagHasPresence) || serial != "":
		pres.syncStart(serial)
	case pres.syncState == syncInitial:
		pres.syncState = syncComplete
		pres.syncMtx.Unlock()
	}
}

// SyncComplete gives true if the initial SYNC operation has completed
// for the members present on the channel.
func (pres *RealtimePresence) SyncComplete() bool {
	pres.mtx.Lock()
	defer pres.mtx.Unlock()
	return pres.syncState == syncComplete
}

func (pres *RealtimePresence) syncStart(serial string) {
	if pres.syncState == syncInProgress {
		return
	} else if pres.syncState != syncInitial {
		// Sync has started, make all callers to Get(true) wait. If it's channel's
		// initial sync, the callers are already waiting.
		pres.syncMtx.Lock()
	}
	pres.serial = serial
	pres.syncState = syncInProgress
	pres.stale = make(map[string]struct{}, len(pres.members))
	for memberKey := range pres.members {
		pres.stale[memberKey] = struct{}{}
	}
}

func (pres *RealtimePresence) syncEnd() {
	if pres.syncState != syncInProgress {
		return
	}
	for memberKey := range pres.stale {
		delete(pres.members, memberKey)
	}
	for memberKey, presence := range pres.members {
		if presence.Action == PresenceActionAbsent {
			delete(pres.members, memberKey)
		}
	}
	pres.stale = nil
	pres.syncState = syncComplete
	// Sync has completed, unblock all callers to Get(true) waiting
	// for the sync.
	pres.syncMtx.Unlock()
}

func (pres *RealtimePresence) processIncomingMessage(msg *protocolMessage, syncSerial string) {
	for _, presmsg := range msg.Presence {
		if presmsg.Timestamp == 0 {
			presmsg.Timestamp = msg.Timestamp
		}
	}
	pres.mtx.Lock()
	if syncSerial != "" {
		pres.syncStart(syncSerial)
	}
	// Filter out old messages by their timestamp.
	messages := make([]*PresenceMessage, 0, len(msg.Presence))
	// Update presence map / channel's member state.
	for _, member := range msg.Presence {
		memberKey := member.ConnectionID + member.ClientID
		if oldMember, ok := pres.members[memberKey]; ok {
			if member.Timestamp <= oldMember.Timestamp {
				continue // do not process old message
			}
		}
		switch member.Action {
		case PresenceActionUpdate:
			member.Action = PresenceActionPresent
			fallthrough
		case PresenceActionPresent:
			delete(pres.stale, memberKey)
			pres.members[memberKey] = member
		case PresenceActionLeave:
			delete(pres.members, memberKey)
		}
		messages = append(messages, member)
	}
	if syncSerial == "" {
		pres.syncEnd()
	}
	pres.mtx.Unlock()
	msg.Count = len(messages)
	msg.Presence = messages
	for _, msg := range msg.Presence {
		var action PresenceAction
		switch msg.Action {
		case PresenceActionAbsent:
			action = PresenceActionAbsent
		case PresenceActionPresent:
			action = PresenceActionPresent
		case PresenceActionEnter:
			action = PresenceActionEnter
		case PresenceActionLeave:
			action = PresenceActionLeave
		case PresenceActionUpdate:
			action = PresenceActionUpdate
		}
		pres.messageEmitter.Emit(action, (*subscriptionPresenceMessage)(msg))
	}
}

// Get returns a list of current members on the channel, attaching the channel
// first is needed.
//
// If the context is canceled before the operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be attached anyway.
func (pres *RealtimePresence) Get(ctx context.Context) ([]*PresenceMessage, error) {
	return pres.GetWithOptions(ctx)
}

// A PresenceGetOption is an optional parameter for
// RealtimePresence.GetWithOptions.
type PresenceGetOption func(*presenceGetOptions)

// PresenceGetWithWaitForSync if true, makes GetWithOptions wait until the
// presence information is fully synchronized with the server before returning.
// It defaults to true.
func PresenceGetWithWaitForSync(wait bool) PresenceGetOption {
	return func(o *presenceGetOptions) {
		o.waitForSync = true
	}
}

type presenceGetOptions struct {
	waitForSync bool
}

func (o *presenceGetOptions) applyWithDefaults(options ...PresenceGetOption) {
	o.waitForSync = true
	for _, opt := range options {
		opt(o)
	}
}

// GetWithOptions is Get with optional parameters.
func (pres *RealtimePresence) GetWithOptions(ctx context.Context, options ...PresenceGetOption) ([]*PresenceMessage, error) {
	var opts presenceGetOptions
	opts.applyWithDefaults(options...)

	res, err := pres.channel.attach()
	if err != nil {
		return nil, err
	}
	// TODO: Don't ignore context.
	err = res.Wait(ctx)
	if err != nil {
		return nil, err
	}

	if opts.waitForSync {
		pres.syncWait()
	}

	pres.mtx.Lock()
	defer pres.mtx.Unlock()
	members := make([]*PresenceMessage, 0, len(pres.members))
	for _, member := range pres.members {
		members = append(members, member)
	}
	return members, nil
}

type subscriptionPresenceMessage PresenceMessage

func (*subscriptionPresenceMessage) isEmitterData() {}

// Subscribe registers a presence message handler to be called with each
// presence message with the given action received from the channel.
//
// This implicitly attaches the channel if it's not already attached. If the
// context is canceled before the attach operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be attached anyway.
//
// See package-level documentation on Event Emitter for details about
// messages dispatch.
func (pres *RealtimePresence) Subscribe(ctx context.Context, action PresenceAction, handle func(*PresenceMessage)) (unsubscribe func(), err error) {
	res, err := pres.channel.attach()
	if err != nil {
		return nil, err
	}
	err = res.Wait(ctx)
	if err != nil {
		return nil, err
	}
	return pres.messageEmitter.On(action, func(message emitterData) {
		handle((*PresenceMessage)(message.(*subscriptionPresenceMessage)))
	}), nil
}

// SubscribeAll registers a presence message handler to be called with each
// presence message received from the channel.
//
// This implicitly attaches the channel if it's not already attached. If the
// context is canceled before the attach operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be attached anyway.
//
// See package-level documentation on Event Emitter for details about
// messages dispatch.
func (pres *RealtimePresence) SubscribeAll(ctx context.Context, handle func(*PresenceMessage)) (unsubscribe func(), err error) {
	res, err := pres.channel.attach()
	if err != nil {
		return nil, err
	}
	err = res.Wait(ctx)
	if err != nil {
		return nil, err
	}
	return pres.messageEmitter.OnAll(func(message emitterData) {
		handle((*PresenceMessage)(message.(*subscriptionPresenceMessage)))
	}), nil
}

// Enter announces presence of the current client with an enter message
// for the associated channel.
//
// If the context is canceled before the operation finishes, the call
// returns with an error, but the operation carries on in the background and
// presence state may eventually be updated anyway.
func (pres *RealtimePresence) Enter(ctx context.Context, data interface{}) error {
	clientID := pres.auth().ClientID()
	if clientID == "" {
		return newError(91000, nil)
	}
	return pres.EnterClient(ctx, clientID, data)
}

// Update announces an updated presence message for the current client.
//
// If the current client is not present on the channel, Update will
// behave as Enter method.
//
// If the context is canceled before the operation finishes, the call
// returns with an error, but the operation carries on in the background and
// presence state may eventually be updated anyway.
func (pres *RealtimePresence) Update(ctx context.Context, data interface{}) error {
	clientID := pres.auth().ClientID()
	if clientID == "" {
		return newError(91000, nil)
	}
	return pres.UpdateClient(ctx, clientID, data)
}

// Leave announces current client leave the channel altogether with a leave
// message if data is non-empty.
//
// If the context is canceled before the operation finishes, the call
// returns with an error, but the operation carries on in the background and
// presence state may eventually be updated anyway.
func (pres *RealtimePresence) Leave(ctx context.Context, data interface{}) error {
	clientID := pres.auth().ClientID()
	if clientID == "" {
		return newError(91000, nil)
	}
	return pres.LeaveClient(ctx, clientID, data)
}

// EnterClient announces presence of the given clientID altogether with an enter
// message for the associated channel.
//
// If the context is canceled before the operation finishes, the call
// returns with an error, but the operation carries on in the background and
// presence state may eventually be updated anyway.
func (pres *RealtimePresence) EnterClient(ctx context.Context, clientID string, data interface{}) error {
	pres.mtx.Lock()
	pres.data = data
	pres.state = PresenceActionEnter
	pres.mtx.Unlock()
	msg := PresenceMessage{
		Action: PresenceActionEnter,
	}
	msg.Data = data
	msg.ClientID = clientID
	res, err := pres.send(&msg)
	if err != nil {
		return err
	}
	return res.Wait(ctx)
}

func nonnil(a, b interface{}) interface{} {
	if a != nil {
		return a
	}
	return b
}

// UpdateClient announces an updated presence message for the given clientID.
//
// If the given clientID is not present on the channel, Update will
// behave as Enter method.
//
// If the context is canceled before the operation finishes, the call
// returns with an error, but the operation carries on in the background and
// presence data may eventually be updated anyway.
func (pres *RealtimePresence) UpdateClient(ctx context.Context, clientID string, data interface{}) error {
	pres.mtx.Lock()
	if pres.state != PresenceActionEnter {
		oldData := pres.data
		pres.mtx.Unlock()
		return pres.EnterClient(ctx, clientID, nonnil(data, oldData))
	}
	pres.data = data
	pres.mtx.Unlock()
	msg := PresenceMessage{
		Action: PresenceActionUpdate,
	}
	msg.ClientID = clientID
	msg.Data = data
	res, err := pres.send(&msg)
	if err != nil {
		return err
	}
	return res.Wait(ctx)
}

// LeaveClient announces the given clientID leave the associated channel altogether
// with a leave message if data is non-empty.
//
// If the context is canceled before the operation finishes, the call
// returns with an error, but the operation carries on in the background and
// presence data may eventually be updated anyway.
func (pres *RealtimePresence) LeaveClient(ctx context.Context, clientID string, data interface{}) error {
	pres.mtx.Lock()
	if pres.data == nil {
		pres.data = data
	}
	pres.mtx.Unlock()

	msg := PresenceMessage{
		Action: PresenceActionLeave,
	}
	msg.ClientID = clientID
	msg.Data = data
	res, err := pres.send(&msg)
	if err != nil {
		return err
	}
	return res.Wait(ctx)
}

func (pres *RealtimePresence) auth() *Auth {
	return pres.channel.client.Auth
}

func (pres *RealtimePresence) log() logger {
	return pres.channel.log()
}
