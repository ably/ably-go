package ably

import (
	"fmt"
	"time"
)

// TR3
const (
	flagHasPresence       protoFlag = 1 << 0
	flagHasBacklog        protoFlag = 1 << 1
	flagResumed           protoFlag = 1 << 2
	flagTransient         protoFlag = 1 << 4
	flagAttachResume      protoFlag = 1 << 5
	flagPresence          protoFlag = 1 << 16
	flagPublish           protoFlag = 1 << 17
	flagSubscribe         protoFlag = 1 << 18
	flagPresenceSubscribe protoFlag = 1 << 19
)

type protoFlag int64

func (flags protoFlag) Has(flag protoFlag) bool {
	return flags&flag == flag
}

func (flags *protoFlag) Set(flag protoFlag) {
	*flags |= flag
}

// connectionDetails contains any constraints a client should adhere to and provides additional metadata about a
// [ably.Connection], such as if a request to RealtimeClient#publish a message that exceeds the maximum message size
// should be rejected immediately without communicating with Ably.
type connectionDetails struct {
	// ClientID contains the client ID assigned to the token. If clientId is null or omitted, then the client is
	// prohibited from assuming a clientId in any operations, however if clientId is a wildcard string *,
	// then the client is permitted to assume any clientId. Any other string value for clientId implies that the
	// clientId is both enforced and assumed for all operations from this client (RSA12a, CD2a).
	ClientID string `json:"clientId,omitempty" codec:"clientId,omitempty"`
	// ConnectionKey is the connection secret key string that is used to resume a connection and its state.
	// (RTN15e, CD2b).
	ConnectionKey string `json:"connectionKey,omitempty" codec:"connectionKey,omitempty"`
	// MaxMessageSize is the maximum message size is an attribute of an Ably account which represents the largest
	// permitted payload size of a single message or set of messages published in a single operation. Publish requests
	// whose payload exceeds this limit are rejected by the server. maxMessageSize enables the client to enforce,
	// or further restrict, the maximum size of a single message or set of messages published via REST.
	// The default value is 65536 (64 KiB). In the case of a realtime connection, the server may indicate
	// the associated maximum message size on connection establishment via ConnectionDetails.maxMessageSize].
	// This value takes precedence over the client's default maxMessageSize (TO3l8).
	MaxMessageSize int64 `json:"maxMessageSize,omitempty" codec:"maxMessageSize,omitempty"`
	// MaxFrameSize is the maximum size of a single POST body or WebSocket frame. This is mostly only relevant
	// for a REST client request (for batch publishes), since publishes will hit the maxMessageSize limit before this.
	// The default value is 524288 (512 KiB) (TO3l8).
	MaxFrameSize int64 `json:"maxFrameSize,omitempty" codec:"maxFrameSize,omitempty"`
	// MaxInboundRate is the maximum allowable number of requests per second from a client or Ably.
	// In the case of a realtime connection, this restriction applies to the number of messages sent,
	// whereas in the case of REST, it is the total number of REST requests per second (CD2e).
	MaxInboundRate int64 `json:"maxInboundRate,omitempty" codec:"maxInboundRate,omitempty"`
	// ConnectionStateTTL is the duration that Ably will persist the connection state for when a Realtime client is
	// abruptly disconnected (CD2f, RTN14e, DF1a).
	ConnectionStateTTL durationFromMsecs `json:"connectionStateTtl,omitempty" codec:"connectionStateTtl,omitempty"`
	// MaxIdleInterval is the maximum length of time in milliseconds that the server will allow no activity
	// to occur in the server to client direction. After such a period of inactivity, the server will send
	// a HEARTBEAT or transport-level ping to the client. If the value is 0, the server will allow arbitrarily-long
	// levels of inactivity (CD2h).
	MaxIdleInterval durationFromMsecs `json:"maxIdleInterval,omitempty" codec:"maxIdleInterval,omitempty"`
}

func (c *connectionDetails) FromMap(ctx map[string]interface{}) {
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
		c.ConnectionStateTTL = durationFromMsecs(coerceInt64(v) * int64(time.Millisecond))
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

type protocolMessage struct {
	Messages          []*Message         `json:"messages,omitempty" codec:"messages,omitempty"`
	Presence          []*PresenceMessage `json:"presence,omitempty" codec:"presence,omitempty"`
	ID                string             `json:"id,omitempty" codec:"id,omitempty"`
	ApplicationID     string             `json:"applicationId,omitempty" codec:"applicationId,omitempty"`
	ConnectionID      string             `json:"connectionId,omitempty" codec:"connectionId,omitempty"`
	ConnectionKey     string             `json:"connectionKey,omitempty" codec:"connectionKey,omitempty"`
	Channel           string             `json:"channel,omitempty" codec:"channel,omitempty"`
	ChannelSerial     string             `json:"channelSerial,omitempty" codec:"channelSerial,omitempty"`
	ConnectionDetails *connectionDetails `json:"connectionDetails,omitempty" codec:"connectionDetails,omitempty"`
	Error             *errorInfo         `json:"error,omitempty" codec:"error,omitempty"`
	MsgSerial         int64              `json:"msgSerial" codec:"msgSerial"`
	Timestamp         int64              `json:"timestamp,omitempty" codec:"timestamp,omitempty"`
	Count             int                `json:"count,omitempty" codec:"count,omitempty"`
	Action            protoAction        `json:"action,omitempty" codec:"action,omitempty"`
	Flags             protoFlag          `json:"flags,omitempty" codec:"flags,omitempty"`
	Params            channelParams      `json:"params,omitempty" codec:"params,omitempty"`
	Auth              *authDetails       `json:"auth,omitempty" codec:"auth,omitempty"`
}

// authDetails contains the token string used to authenticate a client with Ably.
type authDetails struct {
	// AccessToken is the authentication token string (AD2).
	AccessToken string `json:"accessToken,omitempty" codec:"accessToken,omitempty"`
}

func (p *protocolMessage) SetModesAsFlag(modes []ChannelMode) {
	for _, mode := range modes {
		flag := mode.toFlag()
		if flag != 0 {
			p.Flags.Set(flag)
		}
	}
}

func (msg *protocolMessage) String() string {
	switch msg.Action {
	case actionHeartbeat:
		return fmt.Sprintf("(action=%q)", msg.Action)
	case actionAck, actionNack:
		return fmt.Sprintf("(action=%q, serial=%d, count=%d)", msg.Action, msg.MsgSerial, msg.Count)
	case actionConnect:
		return fmt.Sprintf("(action=%q)", msg.Action)
	case actionConnected:
		return fmt.Sprintf("(action=%q, id=%q, details=%#v)", msg.Action, msg.ConnectionID, msg.ConnectionDetails)
	case actionDisconnect:
		return fmt.Sprintf("(action=%q)", msg.Action)
	case actionDisconnected:
		return fmt.Sprintf("(action=%q)", msg.Action)
	case actionClose:
		return fmt.Sprintf("(action=%q)", msg.Action)
	case actionClosed:
		return fmt.Sprintf("(action=%q)", msg.Action)
	case actionError:
		return fmt.Sprintf("(action=%q, error=%#v)", msg.Action, msg.Error)
	case actionAttach:
		return fmt.Sprintf("(action=%q, channel=%q)", msg.Action, msg.Channel)
	case actionAttached:
		return fmt.Sprintf("(action=%q, channel=%q, channelSerial=%q, flags=%x)",
			msg.Action, msg.Channel, msg.ChannelSerial, msg.Flags)
	case actionDetach:
		return fmt.Sprintf("(action=%q, channel=%q)", msg.Action, msg.Channel)
	case actionDetached:
		return fmt.Sprintf("(action=%q, channel=%q)", msg.Action, msg.Channel)
	case actionPresence, actionSync:
		return fmt.Sprintf("(action=%q, id=%v, channel=%q, timestamp=%d, connectionId=%v, presenceMessages=%v)",
			msg.Action, msg.ID, msg.Channel, msg.Timestamp, msg.ConnectionID, msg.Presence)
	case actionMessage:
		return fmt.Sprintf("(action=%q, id=%q, messages=%v)", msg.Action,
			msg.ConnectionID, msg.Messages)
	case actionAuth:
		return fmt.Sprintf("(action=%q, id=%q, auth=%v)", msg.Action,
			msg.ConnectionID, msg.Auth)
	default:
		return fmt.Sprintf("%#v", msg)
	}
}

type conn interface {

	// Send write the given ProtocolMessage to the connection.
	// It is expected to block until whole message is written.
	Send(*protocolMessage) error

	// Receive reads ProtocolMessage from the connection.
	// It is expected to block until whole message is read.
	//
	// If the deadline is greater than zero and no message is received before
	// then, a net.Error with Timeout() == true is returned.
	Receive(deadline time.Time) (*protocolMessage, error)

	// Close closes the connection.
	Close() error
}
