package proto

import "fmt"

const (
	FlagPresence Flag = iota + 1
	FlagBacklog
)

type Flag byte

func (f Flag) Has(flag Flag) bool {
	return f&flag == flag
}

type ConnectionDetails struct {
	ClientID           string `json:"clientId,omitempty" msgpack:"clientId,omitempty"`
	ConnectionKey      string `json:"connectionKey,omitempty" msgpack:"connectionKey,omitempty"`
	MaxMessageSize     int64  `json:"maxMessageSize,omitempty" msgpack:"maxMessageSize,omitempty"`
	MaxFrameSize       int64  `json:"maxFrameSize,omitempty" msgpack:"maxFrameSize,omitempty"`
	MaxInboundRate     int64  `json:"maxInboundRate,omitempty" msgpack:"maxInboundRate,omitempty"`
	ConnectionStateTTL int64  `json:"connectionStateTtl,omitempty" msgpack:"connectionStateTtl,omitempty"`
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

func (msg *ProtocolMessage) String() string {
	switch msg.Action {
	case ActionHeartbeat:
		return fmt.Sprintf("(action=%q)", msg.Action)
	case ActionAck, ActionNack:
		return fmt.Sprintf("(action=%q, serial=%d, count=%d)", msg.Action, msg.MsgSerial, msg.Count)
	case ActionConnect:
		return fmt.Sprintf("(action=%q)", msg.Action)
	case ActionConnected:
		return fmt.Sprintf("(action=%q, id=%q, details=%# v)", msg.Action, msg.ConnectionID, msg.ConnectionDetails)
	case ActionDisconnect:
		return fmt.Sprintf("(action=%q)", msg.Action)
	case ActionDisconnected:
		return fmt.Sprintf("(action=%q)", msg.Action)
	case ActionClose:
		return fmt.Sprintf("(action=%q)", msg.Action)
	case ActionClosed:
		return fmt.Sprintf("(action=%q)", msg.Action)
	case ActionError:
		return fmt.Sprintf("(action=%q, error=%# v)", msg.Action, msg.Error)
	case ActionAttach:
		return fmt.Sprintf("(action=%q, channel=%q)", msg.Action, msg.Channel)
	case ActionAttached:
		return fmt.Sprintf("(action=%q, channel=%q, channelSerial=%q, flags=%x)",
			msg.Action, msg.Channel, msg.ChannelSerial, msg.Flags)
	case ActionDetach:
		return fmt.Sprintf("(action=%q, channel=%q)", msg.Action, msg.Channel)
	case ActionDetached:
		return fmt.Sprintf("(action=%q, channel=%q)", msg.Action, msg.Channel)
	case ActionPresence, ActionSync:
		return fmt.Sprintf("(action=%q, id=%q, channel=%q, timestamp=%d, presenceMessages=%v)",
			msg.Action, msg.ConnectionID, msg.Timestamp, len(msg.Presence))
	case ActionMessage:
		return fmt.Sprintf("(action=%q, id=%q, messages=%v)", msg.Action,
			msg.ConnectionID, msg.Messages)
	default:
		return fmt.Sprintf("%v", msg)
	}
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
