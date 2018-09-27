package proto

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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
	ID           string                 `json:"id,omitempty" codec:"id,omitempty"`
	ClientID     string                 `json:"clientId,omitempty" codec:"clientId,omitempty"`
	ConnectionID string                 `json:"connectionId,omitempty" codec:"connectionID,omitempty"`
	Name         string                 `json:"name,omitempty" codec:"name,omitempty"`
	Data         interface{}            `json:"data,omitempty" codec:"data,omitempty"`
	Encoding     string                 `json:"encoding,omitempty" codec:"encoding,omitempty"`
	Timestamp    int64                  `json:"timestamp" codec:"timestamp"`
	Extras       map[string]interface{} `json:"extras" codec:"extras"`
}

func (m Message) MarshalJSON() ([]byte, error) {
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
		ctx["connectionId"] = m.Name
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
	return json.Marshal(ctx)
}

func (m Message) CodecEncodeSelf(encoder *codec.Encoder) {
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
	encoder.MustEncode(ctx)
}

// CodecDecodeSelf implements codec.Selfer interface for msgpack decoding.
func (m *Message) CodecDecodeSelf(decoder *codec.Decoder) {
	ctx := make(map[string]interface{})
	ctx["data"] = &raw{}
	decoder.MustDecode(&ctx)
	if v, ok := ctx["id"]; ok {
		m.ID = string(v.([]byte))
	}
	if v, ok := ctx["clientId"]; ok {
		m.ClientID = string(v.([]byte))
	}
	if v, ok := ctx["connectionId"]; ok {
		m.ConnectionID = string(v.([]byte))
	}
	if v, ok := ctx["name"]; ok {
		m.Name = string(v.([]byte))
	}
	if v, ok := ctx["encoding"]; ok {
		m.Encoding = string(v.([]byte))
	}
	if v, ok := ctx["data"]; ok {
		r := v.(*raw)
		m.Data = []byte(*r)
	}
	if v, ok := ctx["timestamp"]; ok {
		m.Timestamp = int64(v.(uint64))
	}
	if v, ok := ctx["extras"]; ok {
		m.Extras = v.(map[string]interface{})
	}
}

// ValueEncoding returns encoding type forvalue based on the given protocol.
func ValueEncoding(protocol string, value interface{}) string {
	switch protocol {
	case "application/json":
		switch value.(type) {
		case []byte:
			// references (RSL4d1)
			return Base64
		case string:
			// references (RSL4d2)
			return UTF8
		default:
			// references (RSL4d3)
			return JSON
		}
	case "application/x-msgpack":
		switch value.(type) {
		case []byte, string:
			// references (RSL4c1) and (RSL4c2)
			return ""
		default:
			// references (RSL4c3)
			return JSON
		}
	default:
		return ""
	}
}

// ToStringOrBytes returns []byte, assuming the Value is a string which will be
// casted to []byte or it is []byte which is returned as is.
func ToStringOrBytes(v interface{}) []byte {
	switch e := v.(type) {
	case []byte:
		return e
	default:
		return []byte(v.(string))
	}
}

type raw []byte

func (r raw) MarshalBinary() ([]byte, error) {
	return []byte(r), nil
}

func (r *raw) UnmarshalBinary(data []byte) error {
	if r == nil {
		return errors.New("raw: UnmarshalBinary on nil pointer")
	}
	*r = append((*r)[0:0], data...)
	return nil
}

func wrapPanic(fn func()) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%v", e)
		}
	}()
	fn()
	return nil
}

// MemberKey returns string that allows to uniquely identify connected clients.
func (m *Message) MemberKey() string {
	return m.ConnectionID + ":" + m.ClientID
}

// DecodeData reads the current Encoding field and decode Data following it.
// The Encoding field contains slash (/) separated values and will be read from right to left
// to decode data.
// For example, if Encoding is currently set to "json/base64" it will first try to decode data
// using base64 decoding and then json. In this example JSON is not a real type used in the Go
// library so the string is left untouched.
//
// If opts is not nil, it will be used to decrypt the msessage if the message
// was encrypted.
//
// NOTE: This is not supposed to be called directly by the user , it is intended
// for internal use of the library, unless you know what you are doing stay safe
// and don't try this.
func (m *Message) DecodeData(opts *ChannelOptions) error {
	// strings.Split on empty string returns []string{""}
	if m.Data == nil || m.Encoding == "" {
		return nil
	}
	encodings := strings.Split(m.Encoding, "/")
	for i := len(encodings) - 1; i >= 0; i-- {
		switch encodings[i] {
		case Base64:
			data, err := base64.StdEncoding.DecodeString(string(ToStringOrBytes(m.Data)))
			if err != nil {
				return err
			}
			m.Data = data
		case JSON, UTF8:
		default:
			switch {
			case strings.HasPrefix(encodings[i], Cipher):
				if opts != nil && opts.Cipher.Key != nil {
					if err := m.decrypt(encodings[i], opts); err != nil {
						return err
					}
				} else {
					return fmt.Errorf("decrypting %s without decryption options", encodings[i])
				}
			default:
				return fmt.Errorf("unknown encoding %s", encodings[i])
			}

		}
	}
	return nil
}

// EncodeData resets the current Encoding field to an empty string and starts
// encoding data following the given encoding parameter.
// encoding contains slash (/) separated values that EncodeData will read
// from left to right to encode the current Data string.
//
// You can pass ChannelOptions to configure encryption of the message.
//
// For example for encoding is json/utf-8/cipher+aes-128-cbc/base64 Will be
// handled as follows.
//
//	1- The message will be encoded as json, then
//	2- The result of step 1 will be encoded as utf-8
// 	3- If opts is not nil, we will check if we can get a valid ChannelCipher that
// 	will be used to encrypt the result of step 2 in case we have it then we use
// 	it to encrypt the result of step 2
//
// Any errors encountered in any step will be returned immediately.
//
// NOTE: This is not supposed to be called directly by the user , it is intended
// for internal use of the library, unless you know what you are doing stay safe
// and don't try this.
func (m *Message) EncodeData(encoding string, opts *ChannelOptions) error {
	if encoding == "" {
		return nil
	}
	m.Encoding = ""
	for _, encoding := range strings.Split(encoding, "/") {
		switch encoding {
		case Base64:
			data := base64.StdEncoding.EncodeToString(ToStringOrBytes(m.Data))
			m.Data = data
			m.mergeEncoding(encoding)
			continue
		case UTF8:
			m.mergeEncoding(encoding)
			continue
		case JSON:
			v, err := json.Marshal(m.Data)
			if err != nil {
				return err
			}
			m.Data = string(v)
			m.mergeEncoding(encoding)
			continue
		default:
			if strings.HasPrefix(encoding, Cipher) {
				if opts != nil && opts.Cipher.Key != nil {
					if err := m.encrypt("", opts); err != nil {
						return err
					}
				} else {
					return errors.New("encrypted message received by encryption was not set")
				}
			}
		}
	}
	return nil
}

func (m *Message) getKeyLen(cipherStr string) int64 {
	cipherConf := strings.Split(cipherStr, "+")
	if len(cipherConf) != 2 || cipherConf[0] != "cipher" {
		return 0
	}
	cipherParts := strings.Split(cipherConf[1], "-")
	switch {
	case cipherParts[0] != "aes":
		// TODO log unknown encryption algorithm
		return 0
	case cipherParts[2] != "cbc":
		// TODO log unknown mode
		return 0
	}
	keylen, err := strconv.ParseInt(cipherParts[1], 10, 0)
	if err != nil {
		// TODO parsing error
		return 0
	}
	return keylen
}

func (m *Message) decrypt(cipherStr string, opts *ChannelOptions) error {
	cipher, err := opts.GetCipher()
	if err != nil {
		return err
	}
	out, err := cipher.Decrypt(m.Data.([]byte))
	if err != nil {
		return err
	}
	m.Data = out
	return nil
}

func (m *Message) encrypt(encoding string, opts *ChannelOptions) error {
	cipher, err := opts.GetCipher()
	if err != nil {
		return err
	}
	data, err := cipher.Encrypt(ToStringOrBytes(m.Data))
	if err != nil {
		return err
	}
	m.Data = data
	if encoding != "" {
		encoding += "/"
	}
	m.mergeEncoding(encoding + cipher.GetAlgorithm())
	return nil
}

// addPadding expands the message Data string to a suitable CBC valid length.
// CBC encryption requires specific block size to work.
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

func (m *Message) mergeEncoding(encoding string) {
	if m.Encoding == "" {
		m.Encoding = encoding
	} else {
		m.Encoding = m.Encoding + "/" + encoding
	}
}

func MergeEncoding(base string, e ...string) string {
	if base != "" {
		return strings.Join(append([]string{base}, e...), "/")
	}
	return strings.Join(e, "/")
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
