//go:build !integration

package ably_test

import (
	"crypto/aes"
	"testing"

	"github.com/ably/ably-go/ably"

	"github.com/stretchr/testify/assert"
)

func TestCrypto_RSE1_GetDefaultParams(t *testing.T) {

	for _, c := range []struct {
		name          string
		in            ably.CipherParams
		expected      ably.CipherParams
		expectedPanic bool
	}{
		{
			name: "RSE1a, RSE1b, RSE1d: sets defaults",
			in: ably.CipherParams{
				Key: make([]byte, 256/8),
			},
			expected: ably.CipherParams{
				Key:       make([]byte, 256/8),
				KeyLength: 256,
				Algorithm: ably.CipherAES,
				Mode:      ably.CipherCBC,
			},
		},
		{
			name: "RSE1b: no key panics",
			in: ably.CipherParams{
				Algorithm: ably.CipherAES,
				Mode:      ably.CipherCBC,
			},
			expectedPanic: true,
		},
		{
			name: "RSE1e: wrong key length panics (AES 256)",
			in: ably.CipherParams{
				Key: make([]byte, 256/8-1),
			},
			expectedPanic: true,
		},
		{
			name: "RSE1e: valid key length works (AES 128)",
			in: ably.CipherParams{
				Key: make([]byte, 128/8),
			},
			expected: ably.CipherParams{
				Key:       make([]byte, 128/8),
				KeyLength: 128,
				Algorithm: ably.CipherAES,
				Mode:      ably.CipherCBC,
			},
		},
	} {
		c := c
		t.Run(c.name, func(t *testing.T) {

			defer func() {
				r := recover()
				if r != nil && !c.expectedPanic {
					panic(r)
				} else if r == nil && c.expectedPanic {
					t.Fatal("expected panic")
				}
			}()

			got := ably.Crypto.GetDefaultParams(c.in)
			assert.Equal(t, c.expected, got,
				"expected: %#v; got: %#v", c.expected, got)
		})
	}
}

func TestCrypto_RSE2_GenerateRandomKey(t *testing.T) {
	t.Run("must use default key length", func(ts *testing.T) {
		key, err := ably.Crypto.GenerateRandomKey(0)
		assert.NoError(ts, err)
		bitCount := len(key) * 8
		assert.Equal(ts, ably.DefaultCipherKeyLength, bitCount,
			"expected %d got %d", ably.DefaultCipherKeyLength, bitCount)
	})
	t.Run("must use optional key length", func(ts *testing.T) {
		keyLength := 128
		key, err := ably.Crypto.GenerateRandomKey(keyLength)
		assert.NoError(ts, err)
		bitCount := len(key) * 8
		assert.Equal(ts, 128, bitCount,
			"expected %d got %d", keyLength, bitCount)
	})
}

func Test_Issue330_IVReuse(t *testing.T) {

	params, err := ably.DefaultCipherParams()
	assert.NoError(t, err)
	cipher, err := ably.NewCBCCipher(*params)
	assert.NoError(t, err)
	cipherText1, err := cipher.Encrypt([]byte("foo"))
	assert.NoError(t, err)
	cipherText2, err := cipher.Encrypt([]byte("foo"))
	assert.NoError(t, err)
	iv1 := string(cipherText1[:aes.BlockSize])
	iv2 := string(cipherText2[:aes.BlockSize])
	assert.NotEqual(t, iv1, iv2, "IV shouldn't be reused")
}
