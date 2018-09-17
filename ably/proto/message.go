package proto

import (
	"crypto/aes"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	Data         string                 `json:"data,omitempty" codec:"data,omitempty"`
	Encoding     string                 `json:"encoding,omitempty" codec:"encoding,omitempty"`
	Timestamp    int64                  `json:"timestamp" codec:"timestamp"`
	Extras       map[string]interface{} `json:"extras" codec:"extras"`
}

// MemberKey returns string that allows to uniquely identify connected clients.
func (msg *Message) MemberKey() string {
	return msg.ConnectionID + ":" + msg.ClientID
}

// DecodeData reads the current Encoding field and decode Data following it.
// The Encoding field contains slash (/) separated values and will be read from right to left
// to decode data.
// For example, if Encodind is currently set to "json/base64" it will first try to decode data
// using base64 decoding and then json. In this example JSON is not a real type used in the Go
// library so the string is left untouched.
// To be able to decode aes encoded string, the keys parameter must be present. Otherwise, DecodeData
// will return an error.
func (m *Message) DecodeData(opts *ChannelOptions) error {
	// strings.Split on empty string returns []string{""}
	if m.Encoding == "" || m.Data == "" {
		return nil
	}
	encodings := strings.Split(m.Encoding, "/")
	for i := len(encodings) - 1; i >= 0; i-- {
		switch encodings[i] {
		case Base64:
			data, err := base64.StdEncoding.DecodeString(m.Data)
			if err != nil {
				return err
			}
			m.Data = string(data)
		case JSON, UTF8:
		default:
			switch {
			case strings.HasPrefix(encodings[i], Cipher):
				if opts != nil && opts.Cipher.Key != nil {
					if err := m.Decrypt(encodings[i], opts); err != nil {
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
// encoding data following the given `encoding` parameter.
// `encoding` contains slash (/) separated values that EncodeData will read
// from left to right to encode the current Data string.
// To encode data using aes, the keys parameter must be present. Otherwise,
// EncodeData will return an error.
func (m *Message) EncodeData(encoding string, opts *ChannelOptions) error {
	m.Encoding = ""
	for _, encoding := range strings.Split(encoding, "/") {
		switch encoding {
		case Base64:
			m.Data = base64.StdEncoding.EncodeToString([]byte(m.Data))
			m.mergeEncoding(encoding)
			continue
		case JSON, UTF8:
			m.mergeEncoding(encoding)
			continue
		default:
			if strings.HasPrefix(encoding, Cipher) {
				if opts != nil && opts.Cipher.Key != nil {
					if err := m.Encrypt("", opts); err != nil {
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

func (m *Message) Decrypt(cipherStr string, opts *ChannelOptions) error {
	cipher, err := opts.GetCipher()
	if err != nil {
		return err
	}
	out, err := cipher.Decrypt([]byte(m.Data))
	if err != nil {
		return err
	}
	m.Data = string(out)
	return nil
}

func (m *Message) Encrypt(encoding string, opts *ChannelOptions) error {
	cipher, err := opts.GetCipher()
	if err != nil {
		return err
	}
	data, err := cipher.Encrypt([]byte(m.Data))
	if err != nil {
		return err
	}
	m.Data = string(data)
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
