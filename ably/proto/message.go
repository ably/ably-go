package proto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Message struct {
	ID           string                 `json:"id,omitempty" msgpack:"id,omitempty"`
	ClientID     string                 `json:"clientId,omitempty" msgpack:"clientId,omitempty"`
	ConnectionID string                 `json:"connectionId,omitempty" msgpack:"connectionID,omitempty"`
	Name         string                 `json:"name,omitempty" msgpack:"name,omitempty"`
	Data         string                 `json:"data,omitempty" msgpack:"data,omitempty"`
	Encoding     string                 `json:"encoding,omitempty" msgpack:"encoding,omitempty"`
	Timestamp    int64                  `json:"timestamp" msgpack:"timestamp"`
	Extras       map[string]interface{} `json:"extras" msgpack:"extras"`
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
func (m *Message) DecodeData(key []byte) error {
	// strings.Split on empty string returns []string{""}
	if m.Encoding == "" || m.Data == "" {
		return nil
	}
	encodings := strings.Split(m.Encoding, "/")
	for i := len(encodings) - 1; i >= 0; i-- {
		switch encodings[i] {
		case "base64":
			data, err := base64.StdEncoding.DecodeString(m.Data)
			if err != nil {
				return err
			}
			m.Data = string(data)
		case "json", "utf-8":
		default:
			if err := m.Decrypt(encodings[i], key); err != nil {
				return err
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
func (m *Message) EncodeData(encoding string, key, iv []byte) error {
	m.Encoding = ""
	for _, encoding := range strings.Split(encoding, "/") {
		switch encoding {
		case "base64":
			m.Data = base64.StdEncoding.EncodeToString([]byte(m.Data))
			m.mergeEncoding(encoding)
			continue
		case "json", "utf-8":
			m.mergeEncoding(encoding)
			continue
		default:
			if err := m.Encrypt(encoding, key, iv); err != nil {
				return err
			}
			continue
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

func (m *Message) Decrypt(cipherStr string, key []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	if len(m.Data)%aes.BlockSize != 0 {
		return errors.New("ciphertext is not a multiple of the block size")
	}
	iv := []byte(m.Data[:aes.BlockSize])
	m.Data = m.Data[aes.BlockSize:]
	out := make([]byte, len(m.Data))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, []byte(m.Data))
	out, err = pkcs7Unpad(out, aes.BlockSize)
	if err != nil {
		return err
	}
	m.Data = string(out)
	return nil
}

func (m *Message) Encrypt(cipherStr string, key, iv []byte) error {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return err
	}
	// Apply padding to the payload if it's not already padded. Event if
	// len(m.Data)%aes.Block == 0 we have no guarantee it's already padded.
	// Try to unpad it and pad on failure.
	if _, err = pkcs7Unpad([]byte(m.Data), aes.BlockSize); err != nil {
		data, err := pkcs7Pad([]byte(m.Data), aes.BlockSize)
		if err != nil {
			return err
		}
		m.Data = string(data)
	}
	out := make([]byte, aes.BlockSize+len(m.Data))
	if len(iv) == 0 {
		if _, err = io.ReadFull(rand.Reader, iv); err != nil {
			return err
		}
	}
	copy(out[:aes.BlockSize], iv)
	cipher.NewCBCEncrypter(block, out[:aes.BlockSize]).CryptBlocks(out[aes.BlockSize:], []byte(m.Data))
	m.Data = string(out)
	m.mergeEncoding(cipherStr)
	return nil
}

// addPadding expands the message Data string to a suitable CBC valid length.
// CBC encryption requires specific block size to work.
func (m *Message) addPadding() {
	padlen := byte(aes.BlockSize - (len(m.Data) % aes.BlockSize))
	data := make([]byte, len(m.Data)+int(padlen))
	padding := data[len(m.Data)-1:]
	copy(data, m.Data)
	for i := range padding {
		padding[i] = padlen
	}
	m.Data = string(data)
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
