package protocol

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

type Message struct {
	Name     string `json:"name" msgpack:"name"`
	Data     []byte `json:"data" msgpack:"data"`
	Encoding string `json:"encoding,omitempty" msgpack:"encoding,omitempty"`
}

func (m *Message) DecodeData(keys map[string]string) error {
	encodings := strings.Split(m.Encoding, "/")
	for i := len(encodings) - 1; i >= 0; i-- {
		switch encodings[i] {
		case "base64":
			data, err := base64.StdEncoding.DecodeString(string(m.Data))
			if err != nil {
				return err
			}

			m.Data = data
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

func (m *Message) Decrypt(cipherStr string, keys map[string]string) error {
	cipherConf := strings.Split(cipherStr, "+")

	if len(cipherConf) != 2 || cipherConf[0] != "cipher" {
		return nil
	}

	cipherParts := strings.Split(cipherConf[1], "-")

	if cipherParts[0] != "aes" {
		// TODO log unknown encryption algorithm
		return nil
	}

	if cipherParts[2] != "cbc" {
		// TODO log unknown mode
		return nil
	}

	keylen, err := strconv.ParseInt(cipherParts[1], 10, 0)
	if err != nil {
		// TODO parsing error
		return nil
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

		blockMode := cipher.NewCBCDecrypter(block, iv)
		blockMode.CryptBlocks(m.Data, m.Data)

		m.Data = []byte(strings.TrimRight(string(m.Data), "\x0b"))
	default:
		// TODO log wrong keylen
		// Golang supports only these previous specified keys
	}

	return nil
}
