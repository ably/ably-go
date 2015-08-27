package proto

const (
	FlagPresence Flag = iota + 1
	FlagBacklog
)

type Flag byte

func (f Flag) Has(flag Flag) bool {
	return f&flag == flag
}

type ProtocolMessage struct {
	Action           Action             `json:"action" msgpack:"action"`
	ID               string             `json:"id,omitempty" msgpack:"id,omitempty"`
	Flags            Flag               `json:"flags,omitempty" msgpack:"flags,omitempty"`
	Count            int                `json:"count,omitempty" msgpack:"count,omitempty"`
	Error            *Error             `json:"error,omitempty" msgpack:"error,omitempty"`
	ApplicationId    string             `json:"applicationId,omitempty" msgpack:"applicationId,omitempty"`
	ConnectionId     string             `json:"connectionId,omitempty" msgpack:"connectionId,omitempty"`
	ConnectionSerial int64              `json:"connectionSerial,omitempty" msgpack:"connectionSerial,omitempty"`
	Channel          string             `json:"channel,omitempty" msgpack:"channel,omitempty"`
	ChannelSerial    string             `json:"channelSerial,omitempty" msgpack:"channelSerial,omitempty"`
	MsgSerial        int64              `json:"msgSerial,omitempty" msgpack:"msgSerial,omitempty"`
	Timestamp        int64              `json:"timestamp,omitempty" msgpack:"timestamp,omitempty"`
	Messages         []*Message         `json:"messages,omitempty" msgpack:"messages,omitempty"`
	Presence         []*PresenceMessage `json:"presence,omitempty" msgpack:"presence,omitempty"`
}
