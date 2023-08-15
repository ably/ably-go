package ably

import (
	"fmt"
	"strings"
)

// PresenceAction describes the possible actions members in the presence set can emit (TP2).
type PresenceAction int64

const (
	// PresenceActionAbsent specifies a member is not present in the channel.
	PresenceActionAbsent PresenceAction = iota
	// PresenceActionPresent denotes when subscribing to presence events on a channel that already has members present,
	// this event is emitted for every member already present on the channel before the subscribe listener was registered.
	PresenceActionPresent
	// PresenceActionEnter denotes that new member has entered the channel.
	PresenceActionEnter
	// PresenceActionLeave is a member who was present has now left the channel. This may be a result of an explicit
	// request to leave or implicitly when detaching from the channel. Alternatively, if a member's connection is
	// abruptly disconnected and they do not resume their connection within a minute, Ably treats this as a
	// leave event as the client is no longer present.
	PresenceActionLeave
	// PresenceActionUpdate is a already present member has updated their member data. Being notified of member data
	// updates can be very useful, for example, it can be used to update the status of a user when they are
	// typing a message.
	PresenceActionUpdate
)

func (e PresenceAction) String() string {
	switch e {
	case PresenceActionAbsent:
		return "ABSENT"
	case PresenceActionPresent:
		return "PRESENT"
	case PresenceActionEnter:
		return "ENTER"
	case PresenceActionLeave:
		return "LEAVE"
	case PresenceActionUpdate:
		return "UPDATE"
	}
	return ""
}

func (PresenceAction) isEmitterEvent() {}

type PresenceMessage struct {
	Message
	Action PresenceAction `json:"action" codec:"action"`
}

func (m PresenceMessage) String() string {
	return fmt.Sprintf("<PresenceMessage %v clientID=%v data=%v>", [...]string{
		"absent",
		"present",
		"enter",
		"leave",
		"update",
	}[m.Action], m.ClientID, m.Data)
}

// RTP2b1
func (msg *PresenceMessage) isServerSynthesizedPresenceMessage() bool {
	return strings.HasPrefix(msg.ID, msg.ConnectionID)
}

func (oldMessage *PresenceMessage) IsNewerThan(incomingMessage *PresenceMessage) bool {
	if oldMessage.isServerSynthesizedPresenceMessage() ||
		incomingMessage.isServerSynthesizedPresenceMessage() {
		return oldMessage.Timestamp > incomingMessage.Timestamp
	}
	return false
}
