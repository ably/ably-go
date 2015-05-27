package proto

type ProtocolMessage struct {
	Action           Action             `json:"action"`
	ID               string             `json:"id,omitempty"`
	Flags            byte               `json:"flags,omitempty"`
	Count            int                `json:"count,omitempty"`
	Error            *Error             `json:"error,omitempty"`
	ApplicationId    string             `json:"applicationId,omitempty"`
	ConnectionId     string             `json:"connectionId,omitempty"`
	ConnectionSerial int64              `json:"connectionSerial,omitempty"`
	Channel          string             `json:"channel,omitempty"`
	ChannelSerial    string             `json:"channelSerial,omitempty"`
	MsgSerial        int64              `json:"msgSerial,omitempty"`
	Timestamp        int64              `json:"timestamp,omitempty"`
	Messages         []*Message         `json:"messages,omitempty"`
	Presence         []*PresenceMessage `json:"presence,omitempty"`
}
