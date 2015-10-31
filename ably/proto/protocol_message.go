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
	Messages          []*Message         `json:"messages,omitempty" msgpack:"messages,omitempty"`
	Presence          []*PresenceMessage `json:"presence,omitempty" msgpack:"presence,omitempty"`
	ID                string             `json:"id,omitempty" msgpack:"id,omitempty"`
	ApplicationID     string             `json:"applicationId,omitempty" msgpack:"applicationId,omitempty"`
	ConnectionID      string             `json:"connectionId,omitempty" msgpack:"connectionId,omitempty"`
	ConnectionKey     string             `json:"connectionKey,omitempty" msgpack:"connectionKey,omitempty"`
	Channel           string             `json:"channel,omitempty" msgpack:"channel,omitempty"`
	ChannelSerial     string             `json:"channelSerial,omitempty" msgpack:"channelSerial,omitempty"`
	ConnectionDetails *ConnectionDetails `json:"connectionDetails,omitempty" msgpack:"connectionDetails,omitempty"`
	Error             *Error             `json:"error,omitempty" msgpack:"error,omitempty"`
	MsgSerial         int64              `json:"msgSerial,omitempty" msgpack:"msgSerial,omitempty"`
	ConnectionSerial  int64              `json:"connectionSerial,omitempty" msgpack:"connectionSerial,omitempty"`
	Timestamp         int64              `json:"timestamp,omitempty" msgpack:"timestamp,omitempty"`
	Count             int                `json:"count,omitempty" msgpack:"count,omitempty"`
	Action            Action             `json:"action,omitempty" msgpack:"action,omitempty"`
	Flags             Flag               `json:"flags,omitempty" msgpack:"flags,omitempty"`
}

type Conn interface {
	// Send write the given ProtocolMessage to the connection.
	// It is expected to block until whole message is written.
	Send(*ProtocolMessage) error

	// Receive reads ProtocolMessage from the connection.
	// It is expected to block until whole message is read.
	Receive() (*ProtocolMessage, error)

	// Close closes the connection.
	Close() error
}
