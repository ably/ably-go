package proto

import (
	"fmt"
)

type PresenceAction int64

const (
	PresenceAbsent PresenceAction = iota
	PresencePresent
	PresenceEnter
	PresenceLeave
	PresenceUpdate
)

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
