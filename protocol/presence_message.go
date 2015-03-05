package protocol

type PresenceState int64

const (
	PresenceStateENTER PresenceState = iota
	PresenceStateLEAVE
	PresenceStateUPDATE
)

type PresenceMessage struct {
	State           PresenceState `json:"state"`
	ClientId        string        `json:"client_id"`
	ClientData      *Data         `json:"clientData"`
	MemberId        string        `json:"memberId"`
	InheritMemberId string        `json:"inheritMemberId"`
	ConnectionId    string        `json:"connectionId"`
	InstanceId      string        `json:"instanceId"`
}
