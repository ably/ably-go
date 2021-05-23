package proto_test

import (
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
