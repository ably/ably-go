package protocol

type PresenceState int64

const (
	PresenceStateENTER PresenceState = iota
	PresenceStateLEAVE
	PresenceStateUPDATE
)

type PresenceMessage struct {
	State        PresenceState `json:"action"`
	ClientId     string        `json:"clientId"`
	Data         string        `json:"data"`
	ConnectionId string        `json:"connectionId"`
	Timestamp    int64         `json:"timestamp"`
}
