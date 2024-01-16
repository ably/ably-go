package ably

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type syncState uint8

const (
	syncInitial syncState = iota + 1
	syncInProgress
	syncComplete
)

// RealtimePresence represents a single presence map of a particular channel.
// It allows entering, leaving and updating presence state for the current client or on behalf of other client.
// It enables the presence set to be entered and subscribed to, and the historic presence set to be retrieved for a channel.
type RealtimePresence struct {
	mtx               sync.Mutex
	data              interface{}
	messageEmitter    *eventEmitter
	channel           *RealtimeChannel
	members           map[string]*PresenceMessage // RTP2
	internalMembers   map[string]*PresenceMessage // RTP17
	beforeSyncMembers map[string]*PresenceMessage
	state             PresenceAction
	syncState         syncState
	queue             *msgQueue
	syncDone          chan struct{}
}

func newRealtimePresence(channel *RealtimeChannel) *RealtimePresence {
	pres := &RealtimePresence{
		messageEmitter:  newEventEmitter(channel.log()),
		channel:         channel,
		members:         make(map[string]*PresenceMessage),
		internalMembers: make(map[string]*PresenceMessage),
		syncState:       syncInitial,
		syncDone:        make(chan struct{}),
	}
	pres.queue = newMsgQueue(pres.channel.client.Connection)
	return pres
}

// RTP16c
func (pres *RealtimePresence) isValidChannelState() error {
	switch state := pres.channel.State(); state {
	case ChannelStateDetaching, ChannelStateDetached, ChannelStateFailed, ChannelStateSuspended:
		return newError(91001, fmt.Errorf("unable to enter presence channel (invalid channel state: %s)", state.String()))
	default:
		return nil
	}
}

// RTP5a
func (pres *RealtimePresence) onChannelDetachedOrFailed(err error) {
	for k := range pres.members {
		delete(pres.members, k)
	}
	for k := range pres.internalMembers {
		delete(pres.internalMembers, k)
	}
	pres.queue.Fail(err)
}

// RTP5f, RTP16b
func (pres *RealtimePresence) onChannelSuspended(err error) {
	pres.queue.Fail(err)
}

func (pres *RealtimePresence) maybeEnqueue(msg *protocolMessage, onAck func(err error)) bool {
	if pres.channel.opts().NoQueueing {
		if onAck != nil {
			onAck(errors.New("unable enqueue message because Options.QueueMessages is set to false"))
		}
		return false
	}
	pres.queue.Enqueue(msg, onAck)
	return true
}

func (pres *RealtimePresence) send(msg *PresenceMessage) (result, error) {
	// RTP16c
	if err := pres.isValidChannelState(); err != nil {
		return nil, err
	}
	protomsg := &protocolMessage{
		Action:   actionPresence,
		Channel:  pres.channel.Name,
		Presence: []*PresenceMessage{msg},
	}
	listen := make(chan error, 1)
	onAck := func(err error) {
		listen <- err
	}
	switch pres.channel.State() {
	case ChannelStateInitialized: // RTP16b
		if pres.maybeEnqueue(protomsg, onAck) {
			pres.channel.attach()
		}
	case ChannelStateAttaching: // RTP16b
		pres.maybeEnqueue(protomsg, onAck)
	case ChannelStateAttached: // RTP16a
		pres.channel.client.Connection.send(protomsg, onAck) // RTP16a, RTL6c
	}

	return resultFunc(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-listen:
			return err
		}
	}), nil
}

func (pres *RealtimePresence) syncWait(ctx context.Context) error {
	// If there's an undergoing sync operation or we wait till channel gets
	// attached, the following lock is going to block until the operations
	// complete.
	select {
	case <-pres.syncDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RTP18
func syncSerial(msg *protocolMessage) (noChannelSerial bool, syncSequenceId string, syncCursor string) {
	if empty(msg.ChannelSerial) { // RTP18c
		noChannelSerial = true
		return
	}
	// RTP18a
	serials := strings.Split(msg.ChannelSerial, ":")
	syncSequenceId = serials[0]
	if len(serials) > 1 {
		syncCursor = serials[1]
	}
	return false, syncSequenceId, syncCursor
}

// for every attach local members will be entered
func (pres *RealtimePresence) enterMembers(internalMembers []*PresenceMessage) {
	for _, member := range internalMembers {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		// RTP17g
		err := pres.enterClientWithId(ctx, member.ID, member.ClientID, member.Data)
		// RTP17e
		if err != nil {
			pres.channel.log().Errorf("Error for internal member presence enter with id %v, clientId %v, err %v", member.ID, member.ClientID, err)
			pres.channel.emitErrorUpdate(newError(91004, err), true)
		}
		cancel()
	}
}

func (pres *RealtimePresence) onAttach(msg *protocolMessage) {
	pres.mtx.Lock()
	defer pres.mtx.Unlock()
	// RTP1
	if msg.Flags.Has(flagHasPresence) {
		pres.syncStart()
	} else {
		pres.leaveMembers(pres.members) // RTP19a
		if pres.syncState == syncInitial {
			pres.syncState = syncComplete
			close(pres.syncDone)
		}
	}
	// RTP5b
	pres.queue.Flush()
	// RTP17f
	if len(pres.internalMembers) > 0 {
		internalMembers := make([]*PresenceMessage, len(pres.internalMembers))
		indexCounter := 0
		for _, member := range pres.internalMembers {
			internalMembers[indexCounter] = member
			indexCounter = indexCounter + 1
		}
		go pres.enterMembers(internalMembers)
	}
}

// SyncComplete gives true if the initial SYNC operation has completed for the members present on the channel.
func (pres *RealtimePresence) SyncComplete() bool {
	pres.mtx.Lock()
	defer pres.mtx.Unlock()
	return pres.syncState == syncComplete
}

func (pres *RealtimePresence) syncStart() {
	if pres.syncState == syncInProgress {
		return
	} else if pres.syncState != syncInitial {
		// Start new sync after previous one was finished
		pres.syncDone = make(chan struct{})
	}
	pres.syncState = syncInProgress
	pres.beforeSyncMembers = make(map[string]*PresenceMessage, len(pres.members)) // RTP19
	for memberKey, member := range pres.members {
		pres.beforeSyncMembers[memberKey] = member
	}
}

// RTP19, RTP19a
func (pres *RealtimePresence) leaveMembers(members map[string]*PresenceMessage) {
	for memberKey := range members {
		delete(pres.members, memberKey)
	}
	for _, msg := range members {
		msg.Action = PresenceActionLeave
		msg.ID = ""
		msg.Timestamp = time.Now().UnixMilli()
		pres.messageEmitter.Emit(msg.Action, (*subscriptionPresenceMessage)(msg))
	}
}

func (pres *RealtimePresence) syncEnd() {
	if pres.syncState != syncInProgress {
		return
	}
	pres.leaveMembers(pres.beforeSyncMembers) // RTP19

	for memberKey, presence := range pres.members { // RTP2f
		if presence.Action == PresenceActionAbsent {
			delete(pres.members, memberKey)
		}
	}
	pres.beforeSyncMembers = nil
	pres.syncState = syncComplete
	// Sync has completed, unblock all callers to Get(true) waiting
	// for the sync.
	close(pres.syncDone)
}

// RTP2a, RTP2b, RTP2c
func (pres *RealtimePresence) addPresenceMember(memberMap map[string]*PresenceMessage, memberKey string, presenceMember *PresenceMessage) bool {
	if existingMember, ok := memberMap[memberKey]; ok { // RTP2a
		isMemberNew, err := presenceMember.IsNewerThan(existingMember) // RTP2b
		if err != nil {
			pres.log().Error(err)
		}
		if isMemberNew {
			memberMap[memberKey] = presenceMember
			return true
		}
		return false
	}
	memberMap[memberKey] = presenceMember
	return true
}

// RTP2a, RTP2b, RTP2c
func (pres *RealtimePresence) removePresenceMember(memberMap map[string]*PresenceMessage, memberKey string, presenceMember *PresenceMessage) bool {
	if existingMember, ok := memberMap[memberKey]; ok { // RTP2a
		isMemberNew, err := presenceMember.IsNewerThan(existingMember) // RTP2b
		if err != nil {
			pres.log().Error(err)
		}
		if isMemberNew {
			delete(memberMap, memberKey)
			return existingMember.Action != PresenceActionAbsent
		}
	}
	return false
}

// RTP18
func (pres *RealtimePresence) processProtoSyncMessage(msg *protocolMessage) {
	// TODO - Part of RTP18a where new sequence id is received in middle of sync will not call synStart
	// because sync is in progress. Though it will wait till all proto messages are processed.
	// This is not implemented because of additional complexity of managing locks and reverting to prev. memberstate
	noChannelSerial, _, syncCursor := syncSerial(msg)

	pres.syncStart() // RTP18a, RTP18c

	pres.processProtoPresenceMessage(msg)

	if noChannelSerial || empty(syncCursor) { // RTP18c, RTP18b
		pres.syncEnd()
	}
}

func (pres *RealtimePresence) processProtoPresenceMessage(msg *protocolMessage) {
	pres.mtx.Lock()
	// RTP17 - Update internal presence map
	for _, presenceMember := range msg.Presence {
		memberKey := presenceMember.ClientID                                    // RTP17h
		if pres.channel.client.Connection.ID() != presenceMember.ConnectionID { // RTP17
			continue
		}
		switch presenceMember.Action {
		case PresenceActionEnter, PresenceActionUpdate, PresenceActionPresent: // RTP2d, RTP17b
			presenceMemberShallowCopy := *presenceMember
			presenceMemberShallowCopy.Action = PresenceActionPresent
			pres.addPresenceMember(pres.internalMembers, memberKey, &presenceMemberShallowCopy)
		case PresenceActionLeave: // RTP17b, RTP2e
			if !presenceMember.isServerSynthesized() {
				pres.removePresenceMember(pres.internalMembers, memberKey, presenceMember)
			}
		}
	}

	// Update presence map / channel's member state.
	updatedPresenceMessages := make([]*PresenceMessage, 0, len(msg.Presence))
	for _, presenceMember := range msg.Presence {
		memberKey := presenceMember.ConnectionID + presenceMember.ClientID // TP3h
		memberUpdated := false
		switch presenceMember.Action {
		case PresenceActionEnter, PresenceActionUpdate, PresenceActionPresent: // RTP2d
			delete(pres.beforeSyncMembers, memberKey)
			presenceMemberShallowCopy := *presenceMember
			presenceMemberShallowCopy.Action = PresenceActionPresent
			memberUpdated = pres.addPresenceMember(pres.members, memberKey, &presenceMemberShallowCopy)
		case PresenceActionLeave: // RTP2e
			memberUpdated = pres.removePresenceMember(pres.members, memberKey, presenceMember)
		}
		// RTP2g
		if memberUpdated {
			updatedPresenceMessages = append(updatedPresenceMessages, presenceMember)
		}
	}
	pres.mtx.Unlock()

	// RTP2g
	for _, msg := range updatedPresenceMessages {
		pres.messageEmitter.Emit(msg.Action, (*subscriptionPresenceMessage)(msg))
	}
}

// Get retrieves the current members (array of [ably.PresenceMessage] objects) present on the channel
// and the metadata for each member, such as their [ably.PresenceAction] and ID (RTP11).
// If the channel state is initialised or non-attached, it will be updated to [ably.ChannelStateAttached].
//
// If the context is cancelled before the operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be attached anyway (RTP11).
func (pres *RealtimePresence) Get(ctx context.Context) ([]*PresenceMessage, error) {
	return pres.GetWithOptions(ctx)
}

// A PresenceGetOption is an optional parameter for RealtimePresence.GetWithOptions.
type PresenceGetOption func(*presenceGetOptions)

// PresenceGetWithWaitForSync sets whether to wait for a full presence set synchronization between Ably
// and the clients on the channel to complete before returning the results.
// Synchronization begins as soon as the channel is [ably.ChannelStateAttached].
// When set to true the results will be returned as soon as the sync is complete.
// When set to false the current list of members will be returned without the sync completing.
// The default is true (RTP11c1).
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
// Retrieves the current members (array of [ably.PresenceMessage] objects) present on the channel
// and the metadata for each member, such as their [ably.PresenceAction] and ID (RTP11).
// If the channel state is initialised or non-attached, it will be updated to [ably.ChannelStateAttached].
//
// If the context is cancelled before the operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be attached anyway (RTP11).
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
		err = pres.syncWait(ctx)
		if err != nil {
			return nil, err
		}
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

// Subscribe registers a event listener that is called each time a received [ably.PresenceMessage] matches given
// [ably.PresenceAction] or an action within an array of [ably.PresenceAction], such as a new member entering
// the presence set. A callback may optionally be passed in to this call to be notified of success or failure of
// the channel RealtimeChannel.Attach operation (RTP6b).
//
// This implicitly attaches the channel if it's not already attached. If the
// context is cancelled before the attach operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be attached anyway.
//
// See package-level documentation => [ably] Event Emitters for details about messages dispatch.
func (pres *RealtimePresence) Subscribe(ctx context.Context, action PresenceAction, handle func(*PresenceMessage)) (func(), error) {
	// unsubscribe deregisters a specific listener that is registered to receive [ably.PresenceMessage]
	// on the channel for a given [ably.PresenceAction] (RTP7b).
	unsubscribe := pres.messageEmitter.On(action, func(message emitterData) {
		handle((*PresenceMessage)(message.(*subscriptionPresenceMessage)))
	})
	res, err := pres.channel.attach()
	if err != nil {
		unsubscribe()
		return nil, err
	}
	err = res.Wait(ctx)
	if err != nil {
		unsubscribe()
		return nil, err
	}
	return unsubscribe, err
}

// SubscribeAll registers a event listener that is called each time a received [ably.PresenceMessage] such as a
// new member entering the presence set. A callback may optionally be passed in to this call to be notified of
// success or failure of the channel RealtimeChannel.Attach operation (RTP6a).
//
// This implicitly attaches the channel if it's not already attached. If the
// context is cancelled before the attach operation finishes, the call
// returns with an error, but the operation carries on in the background and
// the channel may eventually be attached anyway.
//
// See package-level documentation => [ably] Event Emitters for details about messages dispatch.
func (pres *RealtimePresence) SubscribeAll(ctx context.Context, handle func(*PresenceMessage)) (func(), error) {
	// unsubscribe Deregisters a specific listener that is registered
	// to receive [ably.PresenceMessage] on the channel (RTP7a).
	unsubscribe := pres.messageEmitter.OnAll(func(message emitterData) {
		handle((*PresenceMessage)(message.(*subscriptionPresenceMessage)))
	})
	res, err := pres.channel.attach()
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

// Enter announces the presence of the current client with optional data payload (enter message) on the channel.
// It enters client presence into the channel presence set.
// A clientId is required to be present on a channel (RTP8).
// If this connection has no clientID then this function will fail.
//
// If the context is cancelled before the operation finishes, the call
// returns with an error, but the operation carries on in the background and
// presence state may eventually be updated anyway.
func (pres *RealtimePresence) Enter(ctx context.Context, data interface{}) error {
	clientID := pres.auth().ClientID()
	if clientID == "" {
		return newError(91000, nil)
	}
	return pres.EnterClient(ctx, clientID, data)
}

// Update announces an updated presence message for the current client. Updates the data payload for a presence member.
// If the current client is not present on the channel, Update will behave as Enter method,
// i.e. if called before entering the presence set, this is treated as an [ably.PresenceActionEnter] event (RTP9).
// If this connection has no clientID then this function will fail.
//
// If the context is cancelled before the operation finishes, the call
// returns with an error, but the operation carries on in the background and
// presence state may eventually be updated anyway.
func (pres *RealtimePresence) Update(ctx context.Context, data interface{}) error {
	clientID := pres.auth().ClientID()
	if clientID == "" {
		return newError(91000, nil)
	}
	return pres.UpdateClient(ctx, clientID, data)
}

// Leave announces current client leave on the channel altogether with an optional data payload (leave message).
// It is removed from the channel presence members set. A client must have previously entered the presence set
// before they can leave it (RTP10).
//
// If the context is cancelled before the operation finishes, the call returns with an error,
// but the operation carries on in the background and presence state may eventually be updated anyway.
func (pres *RealtimePresence) Leave(ctx context.Context, data interface{}) error {
	clientID := pres.auth().ClientID()
	if clientID == "" {
		return newError(91000, nil)
	}
	return pres.LeaveClient(ctx, clientID, data)
}

// EnterClient announces presence of the given clientID altogether with a enter message for the associated channel.
// It enters given clientId in the channel presence set. It enables a single client to update presence on
// behalf of any number of clients using a single connection. The library must have been instantiated with an API key
// or a token bound to a wildcard (*) clientId (RTP4, RTP14, RTP15).
//
// If the context is cancelled before the operation finishes, the call returns with an error,
// but the operation carries on in the background and presence state may eventually be updated anyway.
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

func (pres *RealtimePresence) enterClientWithId(ctx context.Context, id string, clientID string, data interface{}) error {
	pres.mtx.Lock()
	pres.data = data
	pres.state = PresenceActionEnter
	pres.mtx.Unlock()
	msg := PresenceMessage{
		Action: PresenceActionEnter,
	}
	msg.ID = id
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
// If the given clientID is not present on the channel, Update will behave as Enter method.
// Updates the data payload for a presence member using a given clientId.
// Enables a single client to update presence on behalf of any number of clients using a single connection.
// The library must have been instantiated with an API key or a token bound to a wildcard (*) clientId (RTP15).
//
// If the context is cancelled before the operation finishes, the call returns with an error,
// but the operation carries on in the background and presence data may eventually be updated anyway.
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

// LeaveClient announces the given clientID leave from the associated channel altogether with a optional leave message.
// Leaves the given clientId from channel presence set. Enables a single client to update presence
// on behalf of any number of clients using a single connection. The library must have been instantiated with
// an API key or a token bound to a wildcard (*) clientId (RTP15).
//
// If the context is cancelled before the operation finishes, the call returns with an error,
// but the operation carries on in the background and presence data may eventually be updated anyway.
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
