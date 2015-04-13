package proto

type PresenceState int64

const (
	PresenceStateENTER PresenceState = iota
	PresenceStateLEAVE
	PresenceStateUPDATE
)

type PresenceMessage struct {
	Message
	State        PresenceState `json:"action" msgpack:"action"`
	ClientId     string        `json:"clientId" msgpack:"clientId"`
	ConnectionId string        `json:"connectionId" msgpack:"connectionId"`
	Timestamp    int64         `json:"timestamp" msgpack:"timestamp"`
}
