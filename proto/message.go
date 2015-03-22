package proto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Message struct {
	Name     string `json:"name" msgpack:"name"`
	Data     string `json:"data" msgpack:"data"`
	Encoding string `json:"encoding,omitempty" msgpack:"encoding,omitempty"`
}

func (m *Message) DecodeData(keys map[string]string) error {
	encodings := strings.Split(m.Encoding, "/")
	for i := len(encodings) - 1; i >= 0; i-- {
		switch encodings[i] {
		case "base64":
			data, err := base64.StdEncoding.DecodeString(m.Data)
			if err != nil {
				return err
			}

			m.Data = string(data)
			continue

		case "json", "utf-8":
			continue
		default:
			if err := m.Decrypt(encodings[i], keys); err != nil {
				return err
			}
			continue
		}
	}

	return nil
}

func (m *Message) EncodeData(encoding string, keys map[string]string) error {
	m.Encoding = ""
	encodings := strings.Split(encoding, "/")
	for i := 0; i < len(encodings); i++ {
		switch encodings[i] {
		case "base64":
			m.Data = base64.StdEncoding.EncodeToString([]byte(m.Data))
			m.mergeEncoding(encodings[i])
			continue
		case "json", "utf-8":
			m.mergeEncoding(encodings[i])
			continue
		default:
			if err := m.Encrypt(encodings[i], keys); err != nil {
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

	if cipherParts[0] != "aes" {
		// TODO log unknown encryption algorithm
		return 0
	}

	if cipherParts[2] != "cbc" {
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

func (m *Message) Decrypt(cipherStr string, keys map[string]string) error {
	keylen := m.getKeyLen(cipherStr)
	if keylen == 0 {
		return fmt.Errorf("unrecognized key length")
	}

	switch keylen {
	case 128, 192, 256:
		block, err := aes.NewCipher([]byte(keys["key"]))
		if err != nil {
			return err
		}

		if len(m.Data)%aes.BlockSize != 0 {
			return fmt.Errorf("ciphertext is not a multiple of the block size")
		}

		iv := m.Data[:aes.BlockSize]
		m.Data = m.Data[aes.BlockSize:]

		out := make([]byte, len(m.Data))

		blockMode := cipher.NewCBCDecrypter(block, []byte(iv))
		blockMode.CryptBlocks(out, []byte(m.Data))

		m.Data = strings.TrimRight(string(out), "\x0b")
	default:
		// TODO log wrong keylen
		// Golang supports only these previous specified keys
	}

	return nil
}

func (m *Message) Encrypt(cipherStr string, keys map[string]string) error {
	keylen := m.getKeyLen(cipherStr)
	if keylen == 0 {
		return fmt.Errorf("unrecognized key length")
	}

	switch keylen {
	case 128, 192, 256:
		block, err := aes.NewCipher([]byte(keys["key"]))
		if err != nil {
			return err
		}

		if len(m.Data)%aes.BlockSize != 0 {
			m.addPadding()
		}

		out := make([]byte, aes.BlockSize+len(m.Data))
		iv := out[:aes.BlockSize]
		if _, err := io.ReadFull(rand.Reader, iv); err != nil {
			return err
		}

		blockMode := cipher.NewCBCEncrypter(block, iv)
		blockMode.CryptBlocks(out[aes.BlockSize:], []byte(m.Data))

		m.Data = string(out)
		m.mergeEncoding(cipherStr)
	default:
		// TODO log wrong keylen
		// Golang supports only these previous specified keys
	}

	return nil
}

func (m *Message) addPadding() {
	paddingLength := aes.BlockSize - (len(m.Data) % aes.BlockSize)
	paddingChar := "\x0b"
	for i := 0; i < paddingLength; i++ {
		m.Data = m.Data + paddingChar
	}
}

func (m *Message) mergeEncoding(encoding string) {
	if m.Encoding == "" {
		m.Encoding = encoding
	} else {
		m.Encoding = strings.Join([]string{m.Encoding, encoding}, "/")
	}
}
