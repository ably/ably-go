package proto_test

import (
	"crypto/aes"
	"testing"

	"github.com/ably/ably-go/ably/proto"
)

func TestGenerateRandomKey(t *testing.T) {
	t.Run("must use default key length", func(ts *testing.T) {
		key, err := proto.GenerateRandomKey()
		if err != nil {
			ts.Fatal(err)
		}
		got := len(key) * 8 // count bits
		if got != proto.DefaultKeyLength {
			ts.Errorf("expected %d got %d", proto.DefaultKeyLength, got)
		}
	})
	t.Run("must use optional key length", func(ts *testing.T) {
		keyLength := 128
		key, err := proto.GenerateRandomKey(keyLength)
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
	t.Parallel()

	params, err := proto.DefaultCipherParams()
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := proto.NewCBCCipher(*params)
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
