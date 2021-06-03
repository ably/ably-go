package ably

import (
	"fmt"
	"time"
)

// TR3
const (
	FlagHasPresence       Flag = 1 << 0
	FlagHasBacklog        Flag = 1 << 1
	FlagResumed           Flag = 1 << 2
	FlagTransient         Flag = 1 << 4
	FlagAttachResume      Flag = 1 << 5
	FlagPresence          Flag = 1 << 16
	FlagPublish           Flag = 1 << 17
	FlagSubscribe         Flag = 1 << 18
	FlagPresenceSubscribe Flag = 1 << 19
)

type Flag int64

func (flags Flag) Has(flag Flag) bool {
	return flags&flag == flag
}

func (flags *Flag) Set(flag Flag) {
	*flags |= flag
}

type ConnectionDetails struct {
	ClientID           string            `json:"clientId,omitempty" codec:"clientId,omitempty"`
	ConnectionKey      string            `json:"connectionKey,omitempty" codec:"connectionKey,omitempty"`
	MaxMessageSize     int64             `json:"maxMessageSize,omitempty" codec:"maxMessageSize,omitempty"`
	MaxFrameSize       int64             `json:"maxFrameSize,omitempty" codec:"maxFrameSize,omitempty"`
	MaxInboundRate     int64             `json:"maxInboundRate,omitempty" codec:"maxInboundRate,omitempty"`
	ConnectionStateTTL DurationFromMsecs `json:"connectionStateTtl,omitempty" codec:"connectionStateTtl,omitempty"`
	MaxIdleInterval    DurationFromMsecs `json:"maxIdleInterval,omitempty" codec:"maxIdleInterval,omitempty"`
}

func (c *ConnectionDetails) FromMap(ctx map[string]interface{}) {
	if v, ok := ctx["clientId"]; ok {
		c.ClientID = v.(string)
	}
	if v, ok := ctx["connectionKey"]; ok {
		c.ConnectionKey = v.(string)
	}
	if v, ok := ctx["maxMessageSize"]; ok {
		c.MaxMessageSize = coerceInt64(v)
	}
	if v, ok := ctx["maxFrameSize"]; ok {
		c.MaxFrameSize = coerceInt64(v)
	}
	if v, ok := ctx["maxInboundRate"]; ok {
		c.MaxInboundRate = coerceInt64(v)
	}
	if v, ok := ctx["connectionStateTtl"]; ok {
		c.ConnectionStateTTL = DurationFromMsecs(coerceInt64(v) * int64(time.Millisecond))
	}
}

func coerceInt8(v interface{}) int8 {
	switch e := v.(type) {
	case float64:
		return int8(e)
	default:
		return v.(int8)
	}
}

func coerceInt(v interface{}) int {
	switch e := v.(type) {
	case float64:
		return int(e)
	default:
		return v.(int)
	}
}
func coerceInt64(v interface{}) int64 {
	switch e := v.(type) {
	case float64:
		return int64(e)
	default:
		return v.(int64)
	}
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
	Error             *Proto_ErrorInfo   `json:"error,omitempty" codec:"error,omitempty"`
	MsgSerial         int64              `json:"msgSerial" codec:"msgSerial"`
	ConnectionSerial  int64              `json:"connectionSerial" codec:"connectionSerial"`
	Timestamp         int64              `json:"timestamp,omitempty" codec:"timestamp,omitempty"`
	Count             int                `json:"count,omitempty" codec:"count,omitempty"`
	Action            Action             `json:"action,omitempty" codec:"action,omitempty"`
	Flags             Flag               `json:"flags,omitempty" codec:"flags,omitempty"`
	Params            channelParams      `json:"params,omitempty" codec:"params,omitempty"`
}

func (p *ProtocolMessage) SetModesAsFlag(modes []ChannelMode) {
	for _, mode := range modes {
		flag := mode.ToFlag()
		if flag != 0 {
			p.Flags.Set(flag)
		}
	}
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
	//
	// If the deadline is greater than zero and no message is received before
	// then, a net.Error with Timeout() == true is returned.
	Receive(deadline time.Time) (*ProtocolMessage, error)

	// Close closes the connection.
	Close() error
}
