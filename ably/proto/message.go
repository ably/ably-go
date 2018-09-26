package proto

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
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
	Data         *DataValue             `json:"data,omitempty" codec:"data,omitempty"`
	Encoding     string                 `json:"encoding,omitempty" codec:"encoding,omitempty"`
	Timestamp    int64                  `json:"timestamp" codec:"timestamp"`
	Extras       map[string]interface{} `json:"extras" codec:"extras"`
}

// DataValue implements json.Marshaler and json.Unmarshaler and codec.Selfer
// interface that wraps the Value field.
//
type DataValue struct {

	// This field can either be []byte, string or any other value that can be
	// marshalled to json string.
	Value interface{}
}

// NewDataValue returns a new *DataValue instance. This will return an error if
// v is not of type string,struct,map or slice.
func NewDataValue(v interface{}) (*DataValue, error) {
	e := reflect.ValueOf(v)
	if e.Kind() == reflect.Ptr {
		e = e.Elem()
	}
	switch e.Kind() {
	case reflect.String, reflect.Struct, reflect.Map, reflect.Slice:
		return &DataValue{Value: v}, nil
	default:
		return nil, fmt.Errorf("ably-go: %s is not supported for data field", e.Kind())
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

// ToBytes casts the Value field to []byte, use this when you are absolutely
// sure the Value is of []byte, because if it isn't this will panic.
//
// For safety, you should to a safe casting like
//	v,ok:=d.Value.([]byte)
func (d DataValue) ToBytes() []byte {
	return d.Value.([]byte)
}

// ToString casts Value to string. use this when you are absolutely
// sure the Value is of string, because if it isn't this will panic.
//
// For safety, you should to a safe casting like
//	v,ok:=d.Value.(string)
func (d DataValue) ToString() string {
	return d.Value.(string)
}

// ToStringOrBytes returns []byte, assuming the Value is a string which will be
// casted to []byte or it is []byte which is returned as is.
func (d DataValue) ToStringOrBytes() []byte {
	switch e := d.Value.(type) {
	case []byte:
		return e
	default:
		return []byte(d.Value.(string))
	}
}

// MarshalJSON implements json.Marshaler interface.
func (d DataValue) MarshalJSON() ([]byte, error) {
	switch e := d.Value.(type) {
	case []byte:
		// references (RSL4d1)
		v := base64.StdEncoding.EncodeToString(e)
		return json.Marshal(v)
	default:
		// references (RSL4d2), (RSL4d3)
		return json.Marshal(e)
	}
}

// UnmarshalJSON implements json.Unmarshaler interface. This will try to
// unmarshal the the json data to the vallue contained in the Value field.
//
// Note than it is not mandatory for the Value field to contain any value, this
// is just a work around so a user can provide a hint on which value is to be
// expected.
func (d *DataValue) UnmarshalJSON(data []byte) error {
	switch d.Value.(type) {
	case []byte:
		var s string
		err := json.Unmarshal(data, &s)
		if err != nil {
			return err
		}
		v, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return err
		}
		d.Value = v
		return nil
	default:
		return d.unmarshalValue(data)
	}
}

func (d *DataValue) tryUnmarshal(data []byte) error {
	opts := []interface{}{
		"", true, int64(1), 0.1, []interface{}{}, map[string]interface{}{},
	}
	for _, v := range opts {
		d.Value = v
		err := d.unmarshalValue(data)
		if err == nil {
			return nil
		}
	}
	return nil
}

func (d *DataValue) unmarshalValue(data []byte) error {
	if d.Value == nil {
		return d.tryUnmarshal(data)
	}
	e := reflect.ValueOf(d.Value)
	if e.Kind() != reflect.Ptr {
		n := reflect.New(e.Type())
		err := json.Unmarshal(data, n.Interface())
		if err != nil {
			return err
		}
		d.Value = n.Elem().Interface()
		return nil
	}
	return json.Unmarshal(data, e.Interface())
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

// CodecEncodeSelf implements codec.Selfer interface for msgpack encoding.
func (d DataValue) CodecEncodeSelf(e *codec.Encoder) {
	switch v := d.Value.(type) {
	case []byte:
		e.MustEncode(raw(v))
	case string:
		e.MustEncode(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		e.MustEncode(string(b))
	}
}

func (d *DataValue) tryDecode(e *codec.Decoder) error {
	opts := []interface{}{
		raw{},
	}
	for _, v := range opts {
		d.Value = v
		err := wrapPanic(func() {
			d.CodecDecodeSelf(e)
		})
		if err == nil {
			return nil
		}
	}
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

// CodecDecodeSelf implements codec.Selfer interface for msgpack decoding.
func (d *DataValue) CodecDecodeSelf(e *codec.Decoder) {
	if d.Value == nil {
		err := d.tryDecode(e)
		if err != nil {
			log.Println(err)
		}
		return
	}
	ev := reflect.ValueOf(d.Value)
	switch d.Value.(type) {
	case []byte, raw:
		var r raw
		e.MustDecode(&r)
		d.Value = []byte(r)
	case string:
		var s string
		e.MustDecode(&s)
		d.Value = s
	default:
		if ev.Kind() != reflect.Ptr {
			n := reflect.New(ev.Type())
			switch ev.Kind() {
			case reflect.Map, reflect.Slice, reflect.Struct:
				var s string
				e.MustDecode(&s)
				err := json.Unmarshal([]byte(s), n.Interface())
				if err != nil {
					panic(err)
				}
				d.Value = n.Elem().Interface()
			default:
				e.MustDecode(n.Interface())
				d.Value = n.Elem().Interface()
			}
		} else {
			switch ev.Kind() {
			case reflect.Map, reflect.Slice, reflect.Struct:
				var s string
				e.MustDecode(&s)
				err := json.Unmarshal([]byte(s), ev.Interface())
				if err != nil {
					panic(err)
				}
				d.Value = ev.Elem().Interface()
			default:
				e.MustDecode(ev.Interface())
				d.Value = ev.Elem().Interface()
			}
		}
	}
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
			data, err := base64.StdEncoding.DecodeString(m.Data.ToString())
			if err != nil {
				return err
			}
			value, err := NewDataValue(data)
			if err != nil {
				return err
			}
			m.Data = value
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
			data := base64.StdEncoding.EncodeToString(m.Data.ToStringOrBytes())
			value, err := NewDataValue(data)
			if err != nil {
				return err
			}
			m.Data = value
			m.mergeEncoding(encoding)
			continue
		case JSON, UTF8:
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
	out, err := cipher.Decrypt(m.Data.ToBytes())
	if err != nil {
		return err
	}
	value, err := NewDataValue(out)
	if err != nil {
		return err
	}
	m.Data = value
	return nil
}

func (m *Message) encrypt(encoding string, opts *ChannelOptions) error {
	cipher, err := opts.GetCipher()
	if err != nil {
		return err
	}
	data, err := cipher.Encrypt(m.Data.ToStringOrBytes())
	if err != nil {
		return err
	}
	value, err := NewDataValue(data)
	if err != nil {
		return err
	}
	m.Data = value
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
