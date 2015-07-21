package ably

import (
	"fmt"
	"sync"

	"github.com/ably/ably-go/ably/proto"
)

type syncState uint8

const (
	syncInitial syncState = iota + 1
	syncInProgress
	syncComplete
)

// RealtimePresence represents a single presence map of a prarticular channel.
// It allows entering, leaving and updating presence state for the current
// client or on behalf of other client.
type RealtimePresence struct {
	mtx       sync.Mutex
	data      string
	serial    string
	subs      *subscriptions
	channel   *RealtimeChannel
	members   map[string]*proto.PresenceMessage
	stale     map[string]struct{}
	state     proto.PresenceState
	syncMtx   sync.Mutex
	syncState syncState
}

func newRealtimePresence(channel *RealtimeChannel) *RealtimePresence {
	pres := &RealtimePresence{
		subs:      newSubscriptions(subscriptionPresenceMessages),
		channel:   channel,
		members:   make(map[string]*proto.PresenceMessage),
		syncState: syncInitial,
	}
	// Lock syncMtx to make all callers to Get(true) wait until the presence
	// is in initial sync state. This is to not make them early return
	// with an empty presence list before channel attaches.
	pres.syncMtx.Lock()
	return pres
}

func (pres *RealtimePresence) verifyChanState() error {
	switch state := pres.channel.State(); state {
	case StateChanDetached, StateChanDetaching, StateChanClosing, StateChanClosed, StateChanFailed:
		return newError(91001, fmt.Errorf("unable to enter presence channel (invalid channel state: %s)", state.String()))
	default:
		return nil
	}
}

func (pres *RealtimePresence) send(msg *proto.PresenceMessage, result bool) (Result, error) {
	if _, err := pres.channel.attach(false); err != nil {
		return nil, err
	}
	if err := pres.verifyChanState(); err != nil {
		return nil, err
	}
	protomsg := &proto.ProtocolMessage{
		Action:   proto.ActionPresence,
		Channel:  pres.channel.state.channel,
		Presence: []*proto.PresenceMessage{msg},
	}
	return pres.channel.send(protomsg, result)
}

func (pres *RealtimePresence) syncWait() {
	// If there's an undergoing sync operation or we wait till channel gets
	// attached, the following lock is going to block until the operations
	// complete.
	pres.syncMtx.Lock()
	pres.syncMtx.Unlock()
}

func (pres *RealtimePresence) onAttach(hasSync bool) {
	pres.mtx.Lock()
	defer pres.mtx.Unlock()
	switch {
	case hasSync:
		pres.syncStart()
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

func (pres *RealtimePresence) syncStart() {
	if pres.syncState == syncInProgress {
		return
	} else if pres.syncState != syncInitial {
		// Sync has started, make all callers to Get(true) wait. If it's channel's
		// initial sync, the callers are already waiting.
		pres.syncMtx.Lock()
	}
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
		if presence.State == proto.PresenceAbsent {
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
		if presmsg.ConnectionId == "" {
			presmsg.ConnectionId = msg.ConnectionId
		}
		if presmsg.Timestamp == 0 {
			presmsg.Timestamp = msg.Timestamp
		}
	}
	pres.mtx.Lock()
	if syncSerial != "" {
		pres.serial = syncSerial
		pres.syncStart()
	}
	// Filter out old messages by their timestamp.
	messages := make([]*proto.PresenceMessage, 0, len(msg.Presence))
	// Update presence map / channel's member state.
	for _, member := range msg.Presence {
		memberKey := member.ConnectionId + member.ClientID
		if oldMember, ok := pres.members[memberKey]; ok {
			if member.Timestamp <= oldMember.Timestamp {
				continue // do not process old message
			}
		}
		switch member.State {
		case proto.PresenceUpdate:
			memberCopy := *member
			member = &memberCopy
			member.State = proto.PresencePresent
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
	pres.subs.presenceEnqueue(msg)
}

// Get returns a list of current members on the channel.
//
// If wait is true it blocks until undergoing sync operation completes.
// If wait is false or sync already completed, the function returns immediately.
func (pres *RealtimePresence) Get(wait bool) ([]*proto.PresenceMessage, error) {
	if err := pres.verifyChanState(); err != nil {
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

// Subscribe subscribes to presence events on the associated channel.
//
// If the channel is not attached, Subscribe implicitly attaches it.
// If no presence states are given, Subscribe subscribes to all of them.
func (pres *RealtimePresence) Subscribe(states ...proto.PresenceState) (*Subscription, error) {
	if _, err := pres.channel.attach(false); err != nil {
		return nil, err
	}
	return pres.subs.subscribe(statesToKeys(states)...)
}

// Unsubscribe removes previous Subscription for the given presence states.
//
// If sub was already unsubscribed, the method is a nop.
func (pres *RealtimePresence) Unsubscribe(sub *Subscription, states ...proto.PresenceState) {
	pres.subs.unsubscribe(true, sub, statesToKeys(states)...)
}

// Enter announces presence of the current client with an enter message
// for the associated channel.
func (pres *RealtimePresence) Enter(data string) (Result, error) {
	clientID := pres.channel.client.opts.ClientID
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
	clientID := pres.channel.client.opts.ClientID
	if clientID == "" {
		return nil, newError(91000, nil)
	}
	return pres.UpdateClient(clientID, data)
}

// Leave announces current client leave the channel altogether with a leave
// message if data is non-empty.
func (pres *RealtimePresence) Leave(data string) (Result, error) {
	clientID := pres.channel.client.opts.ClientID
	if clientID == "" {
		return nil, newError(91000, nil)
	}
	return pres.LeaveClient(clientID, data)
}

// EnterClient announces presence of the given clientID altogether with an enter
// message for the associated channel.
func (pres *RealtimePresence) EnterClient(clientID, data string) (Result, error) {
	pres.mtx.Lock()
	pres.data = data
	pres.state = proto.PresenceEnter
	pres.mtx.Unlock()
	msg := &proto.PresenceMessage{
		State:    proto.PresenceEnter,
		ClientID: clientID,
		Message:  proto.Message{Data: data},
	}
	return pres.send(msg, true)
}

// UpdateClient announces an updated presence message for the given clientID.
//
// If the given clientID is not present on the channel, Update will
// behave as Enter method.
func (pres *RealtimePresence) UpdateClient(clientID, data string) (Result, error) {
	pres.mtx.Lock()
	if pres.state != proto.PresenceEnter {
		oldData := pres.data
		pres.mtx.Unlock()
		return pres.EnterClient(clientID, nonempty(data, oldData))
	}
	pres.data = data
	pres.mtx.Unlock()
	msg := &proto.PresenceMessage{
		State:    proto.PresenceUpdate,
		ClientID: clientID,
		Message:  proto.Message{Data: data},
	}
	return pres.send(msg, true)
}

// LeaveClient announces the given clientID leave the associated channel altogether
// with a leave message if data is non-empty.
func (pres *RealtimePresence) LeaveClient(clientID, data string) (Result, error) {
	pres.mtx.Lock()
	if pres.state != proto.PresenceEnter {
		pres.mtx.Unlock()
		return nil, newError(91001, nil)
	}
	data = nonempty(data, pres.data)
	pres.data = data
	pres.mtx.Unlock()
	msg := &proto.PresenceMessage{
		State:    proto.PresenceLeave,
		ClientID: clientID,
		Message:  proto.Message{Data: data},
	}
	return pres.send(msg, true)
}
