package ably

import (
	"fmt"
)

// A PresenceAction is a kind of action involving presence in a channel.
type PresenceAction int64

const (
	PresenceActionAbsent PresenceAction = iota
	PresenceActionPresent
	PresenceActionEnter
	PresenceActionLeave
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
