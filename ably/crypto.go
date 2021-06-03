package ably

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

var Crypto struct {
	// GenerateRandomKey returns a random key. keyLength is optional; if
	// non-zero, it should be in bits.
	GenerateRandomKey func(keyLength int) ([]byte, error)

	// GetDefaultParams returns the provided CipherParams with missing fields
	// set to default values. The Key field must be provided.
	GetDefaultParams func(CipherParams) CipherParams
}

func init() {
	Crypto.GenerateRandomKey = generateRandomKey
	Crypto.GetDefaultParams = defaultCipherParams
}

func generateRandomKey(keyLength int) ([]byte, error) {
	if keyLength <= 0 {
		keyLength = defaultCipherKeyLength
	}
	key := make([]byte, keyLength/8)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

func defaultCipherParams(c CipherParams) CipherParams {
	if len(c.Key) == 0 {
		panic(errors.New("cipher key must be provided"))
	}
	if c.Algorithm == 0 {
		c.Algorithm = defaultCipherAlgorithm
	}
	c.KeyLength = len(c.Key) * 8
	if !c.Algorithm.isValidKeyLength(c.KeyLength) {
		panic(fmt.Sprintf("invalid key length for algorithm %v: %d", c.Algorithm, c.KeyLength))
	}
	if c.Mode == 0 {
		c.Mode = defaultCipherMode
	}
	return c
}
