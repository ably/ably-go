package ably

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// Crypto contains the properties required to configure the encryption of [ably.Message] payloads.
var Crypto struct {

	// GenerateRandomKey returns a random key (as a binary/a byte array) to be used in the encryption of the channel.
	// If the language cryptographic randomness primitives are blocking or async, a callback is used.
	// The callback returns a generated binary key.
	// keyLength is passed as a param. It is a length of the key, in bits, to be generated.
	// If not specified, this is equal to the default keyLength of the default algorithm:
	// for AES this is 256 bits (RSE2).
	GenerateRandomKey func(keyLength int) ([]byte, error)

	// GetDefaultParams returns a [ably.CipherParams] object, using the default values for any fields
	// not supplied by the [ably.CipherParamOptions] object. The Key field must be provided (RSE1).
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
