package proto

import (
	"encoding/json"
	"fmt"

	"github.com/ugorji/go/codec"
)

type PresenceAction int64

const (
	PresenceAbsent PresenceAction = iota
	PresencePresent
	PresenceEnter
	PresenceLeave
	PresenceUpdate
)

type PresenceMessage struct {
	Message
	Action PresenceAction `json:"action" codec:"action"`
}

func (m PresenceMessage) String() string {
	return fmt.Sprintf("<PresenceMessage %v data=%v>", [...]string{
		"absent",
		"present",
		"enter",
		"leave",
		"update",
	}[m.Action], m.Data)
}

func (m PresenceMessage) MarshalJSON() ([]byte, error) {
	e, err := m.encodeJSON()
	if err != nil {
		return nil, err
	}
	ctx := e.ToMap()
	ctx["action"] = m.Action
	return json.Marshal(ctx)
}

func (m *PresenceMessage) UnmarshalJSON(data []byte) error {
	var ctx map[string]interface{}
	if err := json.Unmarshal(data, &ctx); err != nil {
		return err
	}
	return m.FromMap(ctx)
}

// CodecEncodeSelf encodes PresenceMessage into a msgpack format.
func (m PresenceMessage) CodecEncodeSelf(encoder *codec.Encoder) {
	e, err := m.encode()
	if err != nil {
		panic(err)
	}
	ctx := e.ToMap()
	ctx["action"] = m.Action
	encoder.MustEncode(ctx)
}

// CodecDecodeSelf implements codec.Selfer interface for msgpack decoding.
func (m *PresenceMessage) CodecDecodeSelf(decoder *codec.Decoder) {
	ctx := make(map[string]interface{})
	decoder.MustDecode(&ctx)
	if err := m.FromMap(ctx); err != nil {
		panic(err)
	}
}

func (m *PresenceMessage) FromMap(ctx map[string]interface{}) error {
	msg := &m.Message
	if err := msg.FromMap(ctx); err != nil {
		return err
	}
	if v, ok := ctx["action"]; ok {
		m.Action = PresenceAction(coerceInt64(v))
	}
	return nil
}

func (m PresenceMessage) ToMap() map[string]interface{} {
	ctx := m.Message.ToMap()
	ctx["action"] = m.Action
	return ctx
}
