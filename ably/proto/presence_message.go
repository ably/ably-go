package proto

type PresenceState int64

const (
	PresenceAbsent PresenceState = iota
	PresencePresent
	PresenceEnter
	PresenceLeave
	PresenceUpdate
)

type PresenceMessage struct {
	Message `codec:",inline"`
	State   PresenceState `json:"action" codec:"action"`
}
