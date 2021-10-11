package ably

import (
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// encodings
const (
	encUTF8   = "utf-8"
	encJSON   = "json"
	encBase64 = "base64"
	encCipher = "cipher"
)

// Message is what Ably channels send and receive.
type Message struct {
	ID           string                 `json:"id,omitempty" codec:"id,omitempty"`
	ClientID     string                 `json:"clientId,omitempty" codec:"clientId,omitempty"`
	ConnectionID string                 `json:"connectionId,omitempty" codec:"connectionID,omitempty"`
	Name         string                 `json:"name,omitempty" codec:"name,omitempty"`
	Data         interface{}            `json:"data,omitempty" codec:"data,omitempty"`
	Encoding     string                 `json:"encoding,omitempty" codec:"encoding,omitempty"`
	Timestamp    int64                  `json:"timestamp,omitempty" codec:"timestamp,omitempty"`
	Extras       map[string]interface{} `json:"extras,omitempty" codec:"extras,omitempty"`
}

func (m Message) String() string {
	return fmt.Sprintf("<Message %q data=%v>", m.Name, m.Data)
}

func unencodableDataErr(data interface{}) error {
	return fmt.Errorf("message data type %T must be string, []byte, or a value that can be encoded as a JSON object or array", data)
}

func (m Message) withEncodedData(cipher channelCipher) (Message, error) {
	if m.Data == nil {
		return m, nil
	}

	// If string isn't UTF-8, convert to []byte to encode it as base64
	// below.
	if d, ok := m.Data.(string); ok && !utf8.ValidString(d) {
		m.Data = []byte(d)
	}

	switch d := m.Data.(type) {
	case string:
	case []byte:
		m.Data = base64.StdEncoding.EncodeToString(d)
		m.Encoding = mergeEncoding(m.Encoding, encBase64)
	default:
		// RSL4c3, RSL4d3: JSON is only for objects and arrays. So marshal data
		// into JSON, then check if it's one of those.
		b, err := json.Marshal(d)
		if err != nil {
			return Message{}, fmt.Errorf("%s; encoding as JSON: %w", unencodableDataErr(d), err)
		}
		token, _ := json.NewDecoder(bytes.NewReader(b)).Token()
		if token != json.Delim('[') && token != json.Delim('{') {
			return Message{}, fmt.Errorf("%s; encoded as JSON %T", unencodableDataErr(d), token)
		}
		m.Data = string(b)
		m.Encoding = mergeEncoding(m.Encoding, encJSON)
	}

	if cipher == nil {
		return m, nil
	}

	// since we know that m.Data is either []byte or string at this point, coerceBytes is always
	// safe here
	bs, err := coerceBytes(m.Data)
	if err != nil {
		panic(err)
	}
	e, err := cipher.Encrypt(bs)
	if err != nil {
		return Message{}, fmt.Errorf("encrypting message data: %w", err)
	}
	// if the plain text is a valid utf-8 string, then add utf-8 to the list
	// of encodings to indicate to the client-side decoder that the plain text
	// should be further decoded into a utf-8 string after being decrypted.
	if s, ok := m.Data.(string); ok && utf8.ValidString(s) {
		m.Encoding = mergeEncoding(m.Encoding, encUTF8)
	}
	m.Data = base64.StdEncoding.EncodeToString(e)
	m.Encoding = mergeEncoding(m.Encoding, cipher.GetAlgorithm())
	m.Encoding = mergeEncoding(m.Encoding, encBase64)
	return m, nil
}

func (m Message) withDecodedData(cipher channelCipher) (Message, error) {
	// TODO: Unexport once proto gets merged into package ably.

	// strings.Split on empty string returns []string{""}
	if m.Data == nil || m.Encoding == "" {
		return m, nil
	}
	encodings := strings.Split(m.Encoding, "/")
	for len(encodings) > 0 {
		encoding := encodings[len(encodings)-1]
		encodings = encodings[:len(encodings)-1]
		switch encoding {
		case encBase64:
			d, err := coerceString(m.Data)
			if err != nil {
				return m, err
			}
			data, err := base64.StdEncoding.DecodeString(d)
			if err != nil {
				return m, err
			}
			m.Data = data
		case encUTF8:
			d, err := coerceString(m.Data)
			if err != nil {
				return m, err
			}
			m.Data = d
		case encJSON:
			d, err := coerceBytes(m.Data)
			if err != nil {
				return m, err
			}
			var result interface{}
			if err := json.Unmarshal(d, &result); err != nil {
				return m, fmt.Errorf("error unmarshaling JSON payload of type %T: %s", m.Data, err.Error())
			}
			m.Data = result
		default:
			if strings.HasPrefix(encoding, encCipher) {
				if cipher == nil {
					return m, fmt.Errorf("message data is encrypted as %s, but cipher wasn't provided", encoding)
				}
				d, err := coerceBytes(m.Data)
				if err != nil {
					return m, err
				}
				d, err = cipher.Decrypt(d)
				if err != nil {
					return m, fmt.Errorf("decrypting message data: %w", err)
				}
				m.Data = d
			} else {
				return m, fmt.Errorf("unknown encoding %s", encoding)
			}
		}
		m.Encoding = strings.Join(encodings, "/")
	}
	return m, nil
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

// memberKey returns string that allows to uniquely identify connected clients.
func (m *Message) memberKey() string {
	return m.ConnectionID + ":" + m.ClientID
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
