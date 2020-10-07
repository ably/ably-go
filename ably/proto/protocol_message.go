package proto

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	FlagPresence Flag = iota + 1
	FlagBacklog
)

type Flag int64

func (f Flag) Has(flag Flag) bool {
	return f&flag == flag
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
	Messages          []Message          `json:"messages,omitempty" codec:"messages,omitempty"`
	Presence          []*PresenceMessage `json:"presence,omitempty" codec:"presence,omitempty"`
	ID                string             `json:"id,omitempty" codec:"id,omitempty"`
	ApplicationID     string             `json:"applicationId,omitempty" codec:"applicationId,omitempty"`
	ConnectionID      string             `json:"connectionId,omitempty" codec:"connectionId,omitempty"`
	ConnectionKey     string             `json:"connectionKey,omitempty" codec:"connectionKey,omitempty"`
	Channel           string             `json:"channel,omitempty" codec:"channel,omitempty"`
	ChannelSerial     string             `json:"channelSerial,omitempty" codec:"channelSerial,omitempty"`
	ConnectionDetails *ConnectionDetails `json:"connectionDetails,omitempty" codec:"connectionDetails,omitempty"`
	Error             *ErrorInfo         `json:"error,omitempty" codec:"error,omitempty"`
	MsgSerial         int64              `json:"msgSerial" codec:"msgSerial"`
	ConnectionSerial  int64              `json:"connectionSerial" codec:"connectionSerial"`
	Timestamp         int64              `json:"timestamp,omitempty" codec:"timestamp,omitempty"`
	Count             int                `json:"count,omitempty" codec:"count,omitempty"`
	Action            Action             `json:"action,omitempty" codec:"action,omitempty"`
	Flags             Flag               `json:"flags,omitempty" codec:"flags,omitempty"`
}

func (p *ProtocolMessage) UnmarshalJSON(b []byte) error {
	ctx := make(map[string]interface{})
	err := json.Unmarshal(b, &ctx)
	if err != nil {
		return err
	}
	p.FromMap(ctx)
	return nil
}

func (p *ProtocolMessage) FromMap(ctx map[string]interface{}) {
	if v, ok := ctx["messages"]; ok {
		i := v.([]interface{})
		for _, v := range i {
			var msg Message
			msg.FromMap(v.(map[string]interface{}))
			p.Messages = append(p.Messages, msg)
		}
	}
	if v, ok := ctx["presence"]; ok {
		i := v.([]interface{})
		for _, v := range i {
			msg := &PresenceMessage{}
			msg.FromMap(v.(map[string]interface{}))
			p.Presence = append(p.Presence, msg)
		}
	}
	if v, ok := ctx["id"]; ok {
		p.ID = v.(string)
	}
	if v, ok := ctx["applicationId"]; ok {
		p.ApplicationID = v.(string)
	}
	if v, ok := ctx["connectionId"]; ok {
		p.ConnectionID = v.(string)
	}
	if v, ok := ctx["connectionKey"]; ok {
		p.ConnectionKey = v.(string)
	}
	if v, ok := ctx["channel"]; ok {
		p.Channel = v.(string)
	}
	if v, ok := ctx["channelSerial"]; ok {
		p.ChannelSerial = v.(string)
	}
	if v, ok := ctx["connectionDetails"]; ok {
		c := &ConnectionDetails{}
		c.FromMap(v.(map[string]interface{}))
		p.ConnectionDetails = c
	}
	if v, ok := ctx["error"]; ok {
		c := &ErrorInfo{}
		c.FromMap(v.(map[string]interface{}))
		p.Error = c
	}
	if v, ok := ctx["msgSerial"]; ok {
		p.MsgSerial = coerceInt64(v)
	}
	if v, ok := ctx["connectionSerial"]; ok {
		p.ConnectionSerial = coerceInt64(v)
	}
	if v, ok := ctx["timestamp"]; ok {
		p.Timestamp = coerceInt64(v)
	}
	if v, ok := ctx["count"]; ok {
		p.Count = int(coerceInt64(v))
	}
	if v, ok := ctx["action"]; ok {
		p.Action = Action(coerceInt8(v))
	}
	if v, ok := ctx["flags"]; ok {
		p.Flags = Flag(coerceInt64(v))
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
