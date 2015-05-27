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
	Message
	State        PresenceState `json:"action" msgpack:"action"`
	ClientID     string        `json:"clientId" msgpack:"clientId"`
	ID           string        `json:"id,omitempty" msgpack:"id,omitempty"`
	ConnectionId string        `json:"connectionId,omitempty" msgpack:"connectionId,omitempty"`
	Timestamp    int64         `json:"timestamp,omitempty" msgpack:"timestamp,omityempty"`
}
