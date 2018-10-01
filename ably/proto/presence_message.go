package proto

import (
	"encoding/base64"
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
	ID           string                 `json:"id,omitempty" codec:"id,omitempty"`
	ClientID     string                 `json:"clientId,omitempty" codec:"clientId,omitempty"`
	ConnectionID string                 `json:"connectionId,omitempty" codec:"connectionID,omitempty"`
	Name         string                 `json:"name,omitempty" codec:"name,omitempty"`
	Data         interface{}            `json:"data,omitempty" codec:"data,omitempty"`
	Encoding     string                 `json:"encoding,omitempty" codec:"encoding,omitempty"`
	Timestamp    int64                  `json:"timestamp" codec:"timestamp"`
	Extras       map[string]interface{} `json:"extras" codec:"extras"`
	State        PresenceState          `json:"action" codec:"action"`
}

func (m PresenceMessage) MarshalJSON() ([]byte, error) {
	ctx := make(map[string]interface{})
	if m.ID != "" {
		ctx["id"] = m.ID
	}
	if m.ClientID != "" {
		ctx["clientId"] = m.ClientID
	}
	if m.ConnectionID != "" {
		ctx["connectionId"] = m.ConnectionID
	}
	if m.Name != "" {
		ctx["name"] = m.Name
	}
	encoding := m.Encoding
	switch e := m.Data.(type) {
	case []byte:
		// references (RSL4d1)
		v := base64.StdEncoding.EncodeToString(e)
		ctx["data"] = v
		encoding = MergeEncoding(encoding, Base64)
	default:
		// references (RSL4d2), (RSL4d3)
		ctx["data"] = e
	}
	if encoding != "" {
		ctx["encoding"] = encoding
	}
	if m.Timestamp != 0 {
		ctx["timestamp"] = m.Timestamp
	}
	if m.Extras != nil {
		ctx["extras"] = m.Extras
	}
	ctx["action"] = m.State
	return json.Marshal(ctx)
}

func (m PresenceMessage) CodecEncodeSelf(encoder *codec.Encoder) {
	ctx := make(map[string]interface{})
	if m.ID != "" {
		ctx["id"] = m.ID
	}
	if m.ClientID != "" {
		ctx["clientId"] = m.ClientID
	}
	if m.ConnectionID != "" {
		ctx["connectionId"] = m.ConnectionID
	}
	if m.Name != "" {
		ctx["name"] = m.Name
	}
	encoding := m.Encoding
	switch e := m.Data.(type) {
	case []byte:
		ctx["data"] = raw(e)
	case string:
		ctx["data"] = e
	default:
		b, err := json.Marshal(e)
		if err != nil {
			panic(err)
		}
		ctx["data"] = string(b)
		encoding = MergeEncoding(encoding, JSON)
	}
	if encoding != "" {
		ctx["encoding"] = encoding
	}
	if m.Timestamp != 0 {
		ctx["timestamp"] = m.Timestamp
	}
	if m.Extras != nil {
		ctx["extras"] = m.Extras
	}
	ctx["action"] = m.State
	encoder.MustEncode(ctx)
}

// CodecDecodeSelf implements codec.Selfer interface for msgpack decoding.
func (m *PresenceMessage) CodecDecodeSelf(decoder *codec.Decoder) {
	ctx := make(map[string]interface{})
	ctx["data"] = &raw{}
	decoder.MustDecode(&ctx)
	if v, ok := ctx["id"]; ok {
		m.ID = string(ToStringOrBytes(v))
	}
	if v, ok := ctx["clientId"]; ok {
		m.ClientID = string(ToStringOrBytes(v))
	}
	if v, ok := ctx["connectionId"]; ok {
		m.ConnectionID = string(ToStringOrBytes(v))
	}
	if v, ok := ctx["name"]; ok {
		m.Name = string(ToStringOrBytes(v))
	}
	if v, ok := ctx["encoding"]; ok {
		m.Encoding = string(ToStringOrBytes(v))
	}
	if v, ok := ctx["data"]; ok {
		r := v.(*raw)
		m.Data = []byte(*r)
	}
	if v, ok := ctx["timestamp"]; ok {
		switch e := v.(type) {
		case float64:
			m.Timestamp = int64(e)
		case uint64:
			m.Timestamp = int64(e)
		case int64:
			m.Timestamp = e
		}
	}
	if v, ok := ctx["extras"]; ok {
		m.Extras = v.(map[string]interface{})
	}
	if v, ok := ctx["action"]; ok {
		m.State = PresenceState(v.(int64))
	}
}
