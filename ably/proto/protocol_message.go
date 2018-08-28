package proto

import "fmt"

const (
	FlagPresence Flag = iota + 1
	FlagBacklog
)

type Flag int64

func (f Flag) Has(flag Flag) bool {
	return f&flag == flag
}

type ConnectionDetails struct {
	ClientID           string `json:"clientId,omitempty" codec:"clientId,omitempty"`
	ConnectionKey      string `json:"connectionKey,omitempty" codec:"connectionKey,omitempty"`
	MaxMessageSize     int64  `json:"maxMessageSize,omitempty" codec:"maxMessageSize,omitempty"`
	MaxFrameSize       int64  `json:"maxFrameSize,omitempty" codec:"maxFrameSize,omitempty"`
	MaxInboundRate     int64  `json:"maxInboundRate,omitempty" codec:"maxInboundRate,omitempty"`
	ConnectionStateTTL int64  `json:"connectionStateTtl,omitempty" codec:"connectionStateTtl,omitempty"`
}

type ProtocolMessage struct {
	Messages          []*Message         `json:"messages,omitempty" codec:"messages,omitempty"`
	Presence          []*PresenceMessage `json:"presence,omitempty" codec:"presence,omitempty"`
	ID                string             `json:"id,omitempty" codec:"id,omitempty"`
	ApplicationID     string             `json:"applicationId,omitempty" codec:"applicationId,omitempty"`
	ConnectionID      string             `json:"connectionId,omitempty" codec:"connectionId,omitempty"`
	ConnectionKey     string             `json:"connectionKey,omitempty" codec:"connectionKey,omitempty"`
	Channel           string             `json:"channel,omitempty" codec:"channel,omitempty"`
	ChannelSerial     string             `json:"channelSerial,omitempty" codec:"channelSerial,omitempty"`
	ConnectionDetails *ConnectionDetails `json:"connectionDetails,omitempty" codec:"connectionDetails,omitempty"`
	Error             *Error             `json:"error,omitempty" codec:"error,omitempty"`
	MsgSerial         int64              `json:"msgSerial,omitempty" codec:"msgSerial,omitempty"`
	ConnectionSerial  int64              `json:"connectionSerial,omitempty" codec:"connectionSerial,omitempty"`
	Timestamp         int64              `json:"timestamp,omitempty" codec:"timestamp,omitempty"`
	Count             int                `json:"count,omitempty" codec:"count,omitempty"`
	Action            Action             `json:"action,omitempty" codec:"action,omitempty"`
	Flags             Flag               `json:"flags,omitempty" codec:"flags,omitempty"`
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
			msg.Action, msg.ConnectionID, msg.Channel, msg.Timestamp, msg.Presence)
	case ActionMessage:
		return fmt.Sprintf("(action=%q, id=%q, messages=%v)", msg.Action,
			msg.ConnectionID, msg.Messages)
	default:
		return fmt.Sprintf("%# v", msg)
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
