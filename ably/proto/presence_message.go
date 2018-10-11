package proto

import (
	"encoding/json"

	"github.com/ugorji/go/codec"
)

type PresenceState int64

const (
	PresenceAbsent PresenceState = iota
	PresencePresent
	PresenceEnter
	PresenceLeave
	PresenceUpdate
)

type PresenceMessage struct {
	Message
	State PresenceState `json:"action" codec:"action"`
}

func (m PresenceMessage) MarshalJSON() ([]byte, error) {
	e, err := m.encodeJSON()
	if err != nil {
		return nil, err
	}
	ctx := e.toMap()
	ctx["action"] = m.State
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
	ctx := e.toMap()
	ctx["action"] = m.State
	encoder.MustEncode(ctx)
}

// CodecDecodeSelf implements codec.Selfer interface for msgpack decoding.
func (m *PresenceMessage) CodecDecodeSelf(decoder *codec.Decoder) {
	ctx := make(map[string]interface{})
	decoder.MustDecode(&ctx)
	if err := m.fromMap(ctx); err != nil {
		panic(err)
	}
}

func (m *PresenceMessage) fromMap(ctx map[string]interface{}) error {
	msg := &m.Message
	if err := msg.FromMap(ctx); err != nil {
		return err
	}
	if v, ok := ctx["action"]; ok {
		m.State = PresenceState(v.(int64))
	}
	return nil
}
