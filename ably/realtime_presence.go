package ably

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ably/ably-go/ably/proto"
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
	members        map[string]*proto.PresenceMessage
	stale          map[string]struct{}
	state          proto.PresenceAction
	syncMtx        sync.Mutex
	syncState      syncState
}

func newRealtimePresence(channel *RealtimeChannel) *RealtimePresence {
	pres := &RealtimePresence{
		messageEmitter: newEventEmitter(channel.logger()),
		channel:        channel,
		members:        make(map[string]*proto.PresenceMessage),
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

func (pres *RealtimePresence) send(msg *proto.PresenceMessage) (Result, error) {
	if _, err := pres.channel.attach(false); err != nil {
		return nil, err
	}
	if err := pres.verifyChanState(); err != nil {
		return nil, err
	}
	protomsg := &proto.ProtocolMessage{
		Action:   proto.ActionPresence,
		Channel:  pres.channel.Name,
		Presence: []*proto.PresenceMessage{msg},
	}
	return pres.channel.send(protomsg)
}

func (pres *RealtimePresence) syncWait() {
	// If there's an undergoing sync operation or we wait till channel gets
	// attached, the following lock is going to block until the operations
	// complete.
	pres.syncMtx.Lock()
	pres.syncMtx.Unlock()
}

func syncSerial(msg *proto.ProtocolMessage) string {
	if i := strings.IndexRune(msg.ChannelSerial, ':'); i != -1 {
		return msg.ChannelSerial[i+1:]
	}
	return ""
}

func (pres *RealtimePresence) onAttach(msg *proto.ProtocolMessage) {
	serial := syncSerial(msg)
	pres.mtx.Lock()
	defer pres.mtx.Unlock()
	switch {
	case msg.Flags.Has(proto.FlagPresence) || serial != "":
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
		if presence.Action == proto.PresenceAbsent {
			delete(pres.members, memberKey)
		}
	}
	pres.stale = nil
	pres.syncState = syncComplete
	// Sync has completed, unblock all callers to Get(true) waiting
	// for the sync.
	pres.syncMtx.Unlock()
}

func (pres *RealtimePresence) processIncomingMessage(msg *proto.ProtocolMessage, syncSerial string) {
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
	messages := make([]*proto.PresenceMessage, 0, len(msg.Presence))
	// Update presence map / channel's member state.
	for _, member := range msg.Presence {
		memberKey := member.ConnectionID + member.ClientID
		if oldMember, ok := pres.members[memberKey]; ok {
			if member.Timestamp <= oldMember.Timestamp {
				continue // do not process old message
			}
		}
		switch member.Action {
		case proto.PresenceUpdate:
			member.Action = proto.PresencePresent
			fallthrough
		case proto.PresencePresent:
			delete(pres.stale, memberKey)
			pres.members[memberKey] = member
		case proto.PresenceLeave:
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
		case proto.PresenceAbsent:
			action = PresenceActionAbsent
		case proto.PresencePresent:
			action = PresenceActionPresent
		case proto.PresenceEnter:
			action = PresenceActionEnter
		case proto.PresenceLeave:
			action = PresenceActionLeave
		case proto.PresenceUpdate:
			action = PresenceActionUpdate
		}
		pres.messageEmitter.Emit(action, (*subscriptionPresenceMessage)(msg))
	}
}

// Get returns a list of current members on the channel.
//
// If wait is true it blocks until undergoing sync operation completes.
// If wait is false or sync already completed, the function returns immediately.
func (pres *RealtimePresence) Get(wait bool) ([]*proto.PresenceMessage, error) {
	if _, err := pres.channel.attach(false); err != nil {
		return nil, err
	}
	if wait {
		pres.syncWait()
	}
	pres.mtx.Lock()
	defer pres.mtx.Unlock()
	members := make([]*proto.PresenceMessage, 0, len(pres.members))
	for _, member := range pres.members {
		members = append(members, member)
	}
	return members, nil
}

// A PresenceAction is a kind of action involving presence in a channel.
type PresenceAction struct {
	name string
}

var (
	PresenceActionAbsent  PresenceAction = PresenceAction{name: "ABSENT"}
	PresenceActionPresent PresenceAction = PresenceAction{name: "PRESENT"}
	PresenceActionEnter   PresenceAction = PresenceAction{name: "ENTER"}
	PresenceActionLeave   PresenceAction = PresenceAction{name: "LEAVE"}
	PresenceActionUpdate  PresenceAction = PresenceAction{name: "UPDATE"}
)

func (e PresenceAction) String() string {
	return e.name
}

func (PresenceAction) isEmitterEvent() {}

type PresenceMessage = proto.PresenceMessage

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
	res, err := pres.channel.attach(true)
	if err != nil {
		return nil, err
	}
	// TODO: Don't ignore context.
	err = res.Wait()
	if err != nil {
		return nil, err
	}
	return pres.messageEmitter.On(action, func(message emitterData) {
		handle((*PresenceMessage)(message.(*subscriptionPresenceMessage)))
	}), nil
}

// Subscribe registers a presence message handler to be called with each
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
	res, err := pres.channel.attach(true)
	if err != nil {
		return nil, err
	}
	// TODO: Don't ignore context.
	err = res.Wait()
	if err != nil {
		return nil, err
	}
	return pres.messageEmitter.OnAll(func(message emitterData) {
		handle((*PresenceMessage)(message.(*subscriptionPresenceMessage)))
	}), nil
}

// Enter announces presence of the current client with an enter message
// for the associated channel.
func (pres *RealtimePresence) Enter(data string) (Result, error) {
	clientID := pres.auth().ClientID()
	if clientID == "" {
		return nil, newError(91000, nil)
	}
	return pres.EnterClient(clientID, data)
}

// Update announces an updated presence message for the current client.
//
// If the current client is not present on the channel, Update will
// behave as Enter method.
func (pres *RealtimePresence) Update(data string) (Result, error) {
	clientID := pres.auth().ClientID()
	if clientID == "" {
		return nil, newError(91000, nil)
	}
	return pres.UpdateClient(clientID, data)
}

// Leave announces current client leave the channel altogether with a leave
// message if data is non-empty.
func (pres *RealtimePresence) Leave(data string) (Result, error) {
	clientID := pres.auth().ClientID()
	if clientID == "" {
		return nil, newError(91000, nil)
	}
	return pres.LeaveClient(clientID, data)
}

// EnterClient announces presence of the given clientID altogether with an enter
// message for the associated channel.
func (pres *RealtimePresence) EnterClient(clientID string, data interface{}) (Result, error) {
	pres.mtx.Lock()
	pres.data = data
	pres.state = proto.PresenceEnter
	pres.mtx.Unlock()
	msg := proto.PresenceMessage{
		Action: proto.PresenceEnter,
	}
	msg.Data = data
	msg.ClientID = clientID
	return pres.send(&msg)
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
func (pres *RealtimePresence) UpdateClient(clientID string, data interface{}) (Result, error) {
	pres.mtx.Lock()
	if pres.state != proto.PresenceEnter {
		oldData := pres.data
		pres.mtx.Unlock()
		return pres.EnterClient(clientID, nonnil(data, oldData))
	}
	pres.data = data
	pres.mtx.Unlock()
	msg := proto.PresenceMessage{
		Action: proto.PresenceUpdate,
	}
	msg.ClientID = clientID
	msg.Data = data
	return pres.send(&msg)
}

// LeaveClient announces the given clientID leave the associated channel altogether
// with a leave message if data is non-empty.
func (pres *RealtimePresence) LeaveClient(clientID string, data interface{}) (Result, error) {
	pres.mtx.Lock()
	if pres.state != proto.PresenceEnter {
		pres.mtx.Unlock()
		return nil, newError(91001, nil)
	}
	if pres.data == nil {
		pres.data = data
	}
	pres.mtx.Unlock()

	msg := proto.PresenceMessage{
		Action: proto.PresenceLeave,
	}
	msg.ClientID = clientID
	msg.Data = data
	return pres.send(&msg)
}

func (pres *RealtimePresence) auth() *Auth {
	return pres.channel.client.Auth
}

func (pres *RealtimePresence) logger() *LoggerOptions {
	return pres.channel.logger()
}
