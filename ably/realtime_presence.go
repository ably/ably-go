package ably

import (
	"errors"
	"sync"

	"github.com/ably/ably-go/ably/proto"
)

// RealtimePresence represents a single presence map of a prarticular channel.
// It allows entering, leaving and updating presence state for the current
// client or on behalf of other client.
type RealtimePresence struct {
	mu             sync.Mutex
	sync           sync.WaitGroup
	data           string
	serial         string
	subs           *subscriptions
	channel        *RealtimeChannel
	members        map[string]*proto.PresenceMessage
	stale          map[string]struct{}
	state          proto.PresenceState
	syncInProgress bool
	synced         bool
}

func newRealtimePresence(channel *RealtimeChannel) *RealtimePresence {
	return &RealtimePresence{
		subs:    newSubscriptions(subscriptionPresenceMessages),
		channel: channel,
		members: make(map[string]*proto.PresenceMessage),
	}
}

func (pres *RealtimePresence) send(msg *proto.PresenceMessage, result bool) (Result, error) {
	if _, err := pres.channel.attach(false); err != nil {
		return nil, err
	}
	switch state := pres.channel.State(); state {
	case StateChanDetached, StateChanDetaching, StateChanClosing, StateChanClosed,
		StateChanFailed:
		return nil, newError(91001, errors.New(StateText(state)))
	}
	protomsg := &proto.ProtocolMessage{
		Action:   proto.ActionPresence,
		Channel:  pres.channel.state.channel,
		Presence: []*proto.PresenceMessage{msg},
	}
	return pres.channel.send(protomsg, result)
}

func (pres *RealtimePresence) syncStartLock() {
	pres.mu.Lock()
	pres.syncStart()
	pres.mu.Unlock()
}

func (pres *RealtimePresence) syncStart() {
	if pres.syncInProgress {
		return
	}
	pres.sync.Add(1)
	pres.syncInProgress = true
	pres.stale = make(map[string]struct{}, len(pres.members))
	for memberKey := range pres.members {
		pres.stale[memberKey] = struct{}{}
	}
}

func (pres *RealtimePresence) syncEnd() {
	if !pres.syncInProgress {
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
	pres.syncInProgress = false
	pres.sync.Done()
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
	pres.mu.Lock()
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
	pres.mu.Unlock()
	msg.Count = len(messages)
	msg.Presence = messages
	pres.subs.presenceEnqueue(msg)
}

func (pres *RealtimePresence) Get(wait bool) []*proto.PresenceMessage {
	if wait {
		pres.sync.Wait()
	}
	pres.mu.Lock()
	defer pres.mu.Unlock()
	members := make([]*proto.PresenceMessage, 0, len(pres.members))
	for _, member := range pres.members {
		members = append(members, member)
	}
	return members
}

func (pres *RealtimePresence) Subscribe(states ...proto.PresenceState) (*Subscription, error) {
	if _, err := pres.channel.attach(false); err != nil {
		return nil, err
	}
	return pres.subs.subscribe(statesToKeys(states)...)
}

func (pres *RealtimePresence) Unsubscribe(sub *Subscription, states ...proto.PresenceState) {
	pres.subs.unsubscribe(true, sub, statesToKeys(states)...)
}

func (pres *RealtimePresence) Enter(data string) (Result, error) {
	clientID := pres.channel.client.opts.ClientID
	if clientID == "" {
		return nil, newError(91000, nil)
	}
	return pres.EnterClient(clientID, data)
}

func (pres *RealtimePresence) Update(data string) (Result, error) {
	clientID := pres.channel.client.opts.ClientID
	if clientID == "" {
		return nil, newError(91000, nil)
	}
	return pres.UpdateClient(clientID, data)
}

func (pres *RealtimePresence) Leave(data string) (Result, error) {
	clientID := pres.channel.client.opts.ClientID
	if clientID == "" {
		return nil, newError(91000, nil)
	}
	return pres.LeaveClient(clientID, data)
}

func (pres *RealtimePresence) EnterClient(clientID, data string) (Result, error) {
	pres.mu.Lock()
	pres.data = data
	pres.state = proto.PresenceEnter
	pres.mu.Unlock()
	msg := &proto.PresenceMessage{
		State:    proto.PresenceEnter,
		ClientID: clientID,
		Message:  proto.Message{Data: data},
	}
	return pres.send(msg, true)
}

func (pres *RealtimePresence) UpdateClient(clientID, data string) (Result, error) {
	pres.mu.Lock()
	if pres.state != proto.PresenceEnter {
		oldData := pres.data
		pres.mu.Unlock()
		return pres.EnterClient(clientID, nonempty(data, oldData))
	}
	pres.data = data
	pres.mu.Unlock()
	msg := &proto.PresenceMessage{
		State:    proto.PresenceUpdate,
		ClientID: clientID,
		Message:  proto.Message{Data: data},
	}
	return pres.send(msg, true)
}

func (pres *RealtimePresence) LeaveClient(clientID, data string) (Result, error) {
	pres.mu.Lock()
	if pres.state != proto.PresenceEnter {
		pres.mu.Unlock()
		return nil, newError(91001, nil)
	}
	data = nonempty(data, pres.data)
	pres.data = data
	pres.mu.Unlock()
	msg := &proto.PresenceMessage{
		State:    proto.PresenceLeave,
		ClientID: clientID,
		Message:  proto.Message{Data: data},
	}
	return pres.send(msg, true)
}
