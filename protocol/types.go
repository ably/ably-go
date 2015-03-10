package protocol

import "fmt"

type Action int64

const (
	ActionHeartbeat Action = iota
	ActionAck
	ActionNack
	ActionConnect
	ActionConnected
	ActionDisconnect
	ActionDisconnected
	ActionClose
	ActionClosed
	ActionError
	ActionAttach
	ActionAttached
	ActionDetach
	ActionDetached
	ActionPresence
	ActionMessage
)

var actions = map[Action]string{
	ActionHeartbeat:    "heartbeat",
	ActionAck:          "ack",
	ActionNack:         "nack",
	ActionConnect:      "connect",
	ActionConnected:    "connected",
	ActionDisconnect:   "disconnect",
	ActionDisconnected: "disconnected",
	ActionClose:        "close",
	ActionClosed:       "closed",
	ActionError:        "error",
	ActionAttach:       "attach",
	ActionAttached:     "attached",
	ActionDetach:       "detach",
	ActionDetached:     "detached",
	ActionPresence:     "presence",
	ActionMessage:      "message",
}

func (a Action) String() string {
	return actions[a]
}

type ProtocolMessage struct {
	Action           Action             `json:"action"`
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

type Error struct {
	StatusCode int    `json:"statusCode"`
	Code       int    `json:"code"`
	Message    string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d: %d %s", e.StatusCode, e.Code, e.Message)
}

type Type int64

const (
	TypeNONE Type = iota
	TypeTRUE
	TypeFALSE
	TypeINT32
	TypeINT64
	TypeDOUBLE
	TypeSTRING
	TypeBUFFER
	TypeJSONARRAY
	TypeJSONOBJECT
)

type Data struct {
	Type       Type    `json:"type"`
	I32Data    int32   `json:"i32Data"`
	I64Data    int64   `json:"i64Data"`
	DoubleData float64 `json:"doubleData"`
	StringData string  `json:"stringData"`
	BinaryData []byte  `json:"binaryData"`
	CipherData []byte  `json:"cipherData"`
}
