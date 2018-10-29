package proto

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/ugorji/go/codec"
)

// encodings
const (
	UTF8   = "utf-8"
	JSON   = "json"
	Base64 = "base64"
	Cipher = "cipher"
)

type Message struct {
	ID              string                 `json:"id,omitempty" codec:"id,omitempty"`
	ClientID        string                 `json:"clientId,omitempty" codec:"clientId,omitempty"`
	ConnectionID    string                 `json:"connectionId,omitempty" codec:"connectionID,omitempty"`
	Name            string                 `json:"name,omitempty" codec:"name,omitempty"`
	Data            interface{}            `json:"data,omitempty" codec:"data,omitempty"`
	Encoding        string                 `json:"encoding,omitempty" codec:"encoding,omitempty"`
	Timestamp       int64                  `json:"timestamp" codec:"timestamp"`
	Extras          map[string]interface{} `json:"extras" codec:"extras"`
	*ChannelOptions `json:"-" codec:"-"`
}

func (m *Message) maybeJSONEncode() error {
	if m.Data == nil {
		return nil
	}
	switch m.Data.(type) {
	case json.Marshaler:
		bs, err := m.Data.(json.Marshaler).MarshalJSON()
		if err != nil {
			return err
		}
		m.Data = string(bs)
		m.Encoding = mergeEncoding(m.Encoding, JSON)
		return nil
	case map[string]interface{}:
		bs, err := json.Marshal(m.Data)
		if err != nil {
			return err
		}
		m.Data = string(bs)
		m.Encoding = mergeEncoding(m.Encoding, JSON)
		return nil
	}
	dataType := reflect.TypeOf(m.Data)

	// marshal any sort of slice except for []byte (i.e. []uint8)
	if dataType.Kind() == reflect.Slice && dataType.Elem().Kind() != reflect.Uint8 {
		bs, err := json.Marshal(m.Data)
		if err != nil {
			return err
		}
		m.Data = string(bs)
		m.Encoding = mergeEncoding(m.Encoding, JSON)
	}
	return nil
}

func (m Message) HasCipher() bool {
	if m.ChannelOptions != nil {
		c, _ := m.ChannelOptions.GetCipher()
		return c != nil
	}
	return false
}

func (m Message) encode() (Message, error) {
	if m.Data == nil {
		return m, nil
	}
	err := m.maybeJSONEncode()
	if err != nil {
		return m, err
	}
	switch m.Data.(type) {
	case string:
		if m.HasCipher() {
			m.Encoding = mergeEncoding(m.Encoding, UTF8)
		}
	case []byte:
		// ok
	default:
		return Message{}, errors.New("unsupported payload type")
	}
	if m.ChannelOptions != nil {
		if cipher, err := m.GetCipher(); err == nil {
			// since we know that m.Data is either []byte or string at this point, coerceBytes is always
			// safe here
			bs, err := coerceBytes(m.Data)
			if err != nil {
				return Message{}, err
			}
			e, err := cipher.Encrypt(bs)
			if err != nil {
				return Message{}, err
			}
			m.Data = e
			m.Encoding = mergeEncoding(m.Encoding, cipher.GetAlgorithm())
		}
	}

	return m, nil
}

func (m Message) encodeJSON() (Message, error) {
	e, err := m.encode()
	if err != nil {
		return m, err
	}
	if d, ok := e.Data.([]byte); ok {
		b := base64.StdEncoding.EncodeToString(d)
		e.Data = b
		e.Encoding = mergeEncoding(e.Encoding, Base64)
	}
	return e, nil
}

func coerceString(i interface{}) (string, error) {
	switch v := i.(type) {
	case []byte:
		return string(v), nil
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("UTF8 encoding can only handle types string or []byte, but got %T", i)
	}
}

func coerceBytes(i interface{}) ([]byte, error) {
	switch v := i.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("coerceBytes should never get anything but strings or []byte, got %T", i)
	}
}

// ToMap returns a map of all message field names to their respective value if
// the field are set.
//
// Spec RSL1j
func (m Message) ToMap() map[string]interface{} {
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
	if m.Data != nil {
		ctx["data"] = m.Data
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
	return ctx
}

func (m Message) MarshalJSON() ([]byte, error) {
	e, err := m.encodeJSON()
	if err != nil {
		return nil, err
	}
	return json.Marshal(e.ToMap())
}

func (m *Message) UnmarshalJSON(data []byte) error {
	var ctx map[string]interface{}
	if err := json.Unmarshal(data, &ctx); err != nil {
		return err
	}
	return m.FromMap(ctx)
}

func (m Message) CodecEncodeSelf(encoder *codec.Encoder) {
	e, err := m.encode()
	if err != nil {
		panic(err)
	}
	encoder.MustEncode(e.ToMap())
}

// CodecDecodeSelf implements codec.Selfer interface for msgpack decoding.
func (m *Message) CodecDecodeSelf(decoder *codec.Decoder) {
	ctx := make(map[string]interface{})
	decoder.MustDecode(&ctx)
	m.FromMap(ctx)
}

func (m *Message) FromMap(ctx map[string]interface{}) error {
	if v, ok := ctx["id"]; ok {
		x, err := coerceString(v)
		if err != nil {
			return err
		}
		m.ID = string(x)
	}
	if v, ok := ctx["clientId"]; ok {
		x, err := coerceString(v)
		if err != nil {
			return err
		}
		m.ClientID = string(x)
	}
	if v, ok := ctx["connectionId"]; ok {
		x, err := coerceString(v)
		if err != nil {
			return err
		}
		m.ConnectionID = string(x)
	}
	if v, ok := ctx["name"]; ok {
		x, err := coerceString(v)
		if err != nil {
			return err
		}
		m.Name = string(x)
	}
	if v, ok := ctx["encoding"]; ok {
		x, err := coerceString(v)
		if err != nil {
			return err
		}
		m.Encoding = string(x)
	}
	if v, ok := ctx["data"]; ok {
		m.Data = v
		dec, err := m.decode()
		if err != nil {
			return err
		}
		*m = dec
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
	return nil
}

// MemberKey returns string that allows to uniquely identify connected clients.
func (m *Message) MemberKey() string {
	return m.ConnectionID + ":" + m.ClientID
}

func (m Message) Decrypt() (interface{}, error) {
	cipher, err := m.GetCipher()
	if err != nil {
		return nil, err
	}

	d, err := coerceBytes(m.Data)
	if err != nil {
		return nil, err
	}
	v, err := cipher.Decrypt(d)
	if err != nil {
		fmt.Println("decrypting ", m.Encoding, len(d), len(string(d)))
		return nil, err
	}
	return v, nil
}

func (m Message) decode() (Message, error) {
	// strings.Split on empty string returns []string{""}
	if m.Data == nil || m.Encoding == "" {
		return m, nil
	}
	encodings := strings.Split(m.Encoding, "/")
	for i := len(encodings) - 1; i >= 0; i-- {
		switch encodings[i] {
		case Base64:
			d, err := coerceString(m.Data)
			if err != nil {
				return Message{}, err
			}
			data, err := base64.StdEncoding.DecodeString(d)
			if err != nil {
				return Message{}, err
			}
			m.Data = data
		case UTF8:
			d, err := coerceString(m.Data)
			if err != nil {
				return Message{}, err
			}
			m.Data = d
		case JSON:
			d, err := coerceBytes(m.Data)
			if err != nil {
				return Message{}, err
			}
			var result interface{}
			if err := json.Unmarshal(d, &result); err != nil {
				return m, fmt.Errorf("error unmarshaling JSON payload of type %T: %s", m.Data, err.Error())
			}
			m.Data = result
		default:
			switch {
			case strings.HasPrefix(encodings[i], Cipher):
				d, err := m.Decrypt()
				if err != nil {
					return m, err
				}
				m.Data = d
			default:
				return m, fmt.Errorf("unknown encoding %s", encodings[i])
			}

		}
	}
	return m, nil
}

func addPadding(src []byte) []byte {
	padlen := byte(aes.BlockSize - (len(src) % aes.BlockSize))
	data := make([]byte, len(src)+int(padlen))
	padding := data[len(src)-1:]
	copy(data, src)
	for i := range padding {
		padding[i] = padlen
	}
	return data
}

func mergeEncoding(a string, b string) string {
	if a == "" {
		return b
	}
	return a + "/" + b
}

// Appends padding.
func pkcs7Pad(data []byte, blocklen int) ([]byte, error) {
	if blocklen <= 0 {
		return nil, fmt.Errorf("invalid blocklen %d", blocklen)
	}
	padlen := 1
	for ((len(data) + padlen) % blocklen) != 0 {
		padlen = padlen + 1
	}
	p := make([]byte, len(data)+padlen)
	copy(p, data)
	for i := len(data); i < len(p); i++ {
		p[i] = byte(padlen)
	}
	return p, nil
}

// Returns slice of the original data without padding.
func pkcs7Unpad(data []byte, blocklen int) ([]byte, error) {
	if blocklen <= 0 {
		return nil, fmt.Errorf("invalid blocklen %d", blocklen)
	}
	if len(data)%blocklen != 0 || len(data) == 0 {
		return nil, fmt.Errorf("invalid data len %d", len(data))
	}
	padlen := int(data[len(data)-1])
	if padlen > blocklen || padlen == 0 {
		// no padding found.
		return data, nil
	}
	// check padding
	for _, p := range data[len(data)-padlen:] {
		if p != byte(padlen) {
			return nil, fmt.Errorf("invalid padding character")
		}
	}
	return data[:len(data)-padlen], nil
}
