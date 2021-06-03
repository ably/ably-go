package ably

import (
	"fmt"
)

type Proto_PresenceAction int64

const (
	PresenceAbsent Proto_PresenceAction = iota
	PresencePresent
	PresenceEnter
	PresenceLeave
	PresenceUpdate
)

type PresenceMessage struct {
	Message
	Action Proto_PresenceAction `json:"action" codec:"action"`
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
