package ably

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// Crypto Contains the properties required to configure the encryption of [Message]{@link Message} payloads.
var Crypto struct {

	// **LEGACY**
	// GenerateRandomKey returns a random key. keyLength is optional; if
	// non-zero, it should be in bits.
	// **CANONICAL**
	// Generates a random key to be used in the encryption of the channel. If the language cryptographic randomness primitives are blocking or async, a callback is used. The callback returns a generated binary key.
	// keyLength - The length of the key, in bits, to be generated. If not specified, this is equal to the default keyLength of the default algorithm: for AES this is 256 bits.
	// Returns - The key as a binary, for example, a byte array.
	// RSE2
	GenerateRandomKey func(keyLength int) ([]byte, error)

	// **LEGACY**
	// GetDefaultParams returns the provided CipherParams with missing fields
	// set to default values. The Key field must be provided.
	// **CANONICAL**
	// Returns a [CipherParams]{@link CipherParams} object, using the default values for any fields not supplied by the [CipherParamOptions]{@link CipherParamOptions} object.
	// CipherParamOptions - A [CipherParamOptions]{@link CipherParamOptions} object.
	// Returns - A [CipherParams]{@link CipherParams} object, using the default values for any fields not supplied.
	// RSE1
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
