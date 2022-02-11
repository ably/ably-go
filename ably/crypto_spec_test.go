//go:build !integration
// +build !integration

package ably_test

import (
	"crypto/aes"
	"reflect"
	"testing"

	"github.com/ably/ably-go/ably"
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
			if !reflect.DeepEqual(c.expected, got) {
				t.Fatalf("expected: %#v; got: %#v", c.expected, got)
			}
		})
	}
}

func TestCrypto_RSE2_GenerateRandomKey(t *testing.T) {
	t.Run("must use default key length", func(ts *testing.T) {
		key, err := ably.Crypto.GenerateRandomKey(0)
		if err != nil {
			ts.Fatal(err)
		}
		got := len(key) * 8 // count bits
		if got != ably.DefaultCipherKeyLength {
			ts.Errorf("expected %d got %d", ably.DefaultCipherKeyLength, got)
		}
	})
	t.Run("must use optional key length", func(ts *testing.T) {
		keyLength := 128
		key, err := ably.Crypto.GenerateRandomKey(keyLength)
		if err != nil {
			ts.Fatal(err)
		}
		got := len(key) * 8 // count bits
		if got != keyLength {
			ts.Errorf("expected %d got %d", keyLength, got)
		}
	})
}

func Test_Issue330_IVReuse(t *testing.T) {

	params, err := ably.DefaultCipherParams()
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := ably.NewCBCCipher(*params)
	if err != nil {
		t.Fatal(err)
	}
	cipherText1, err := cipher.Encrypt([]byte("foo"))
	if err != nil {
		t.Fatal(err)
	}
	cipherText2, err := cipher.Encrypt([]byte("foo"))
	if err != nil {
		t.Fatal(err)
	}
	iv1 := string(cipherText1[:aes.BlockSize])
	iv2 := string(cipherText2[:aes.BlockSize])
	if iv1 == iv2 {
		t.Fatal("IV shouldn't be reused")
	}
}
