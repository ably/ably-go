//go:build !unit
// +build !unit

package ably_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"
)

func TestMessage_CryptoDataFixtures_RSL6a1_RSL5b_RSL5c(t *testing.T) {
	fixtures := []struct {
		desc, file string
		keylength  int
	}{
		{"with a 128 keylength", "test-resources/crypto-data-128.json", 128},
		{"with a 256 keylength", "test-resources/crypto-data-256.json", 126},
	}

	for _, fixture := range fixtures {
		// pin
		fixture := fixture
		t.Run(fixture.desc, func(t *testing.T) {
			test, key, iv, err := ablytest.LoadCryptoData(fixture.file)
			if err != nil {
				t.Fatal(err)
			}
			params := ably.CipherParams{
				Algorithm: ably.CipherAES,
				Key:       key,
			}
			params.SetIV(iv)
			opts := &ably.ProtoChannelOptions{
				Cipher: params,
			}
			t.Run("fixture encode", func(t *testing.T) {
				for _, item := range test.Items {
					cipher, _ := opts.GetCipher()

					var encoded ably.Message
					err := json.Unmarshal(item.Encoded, &encoded)
					if err != nil {
						t.Fatal(err)
					}
					encoded, err = ably.MessageWithDecodedData(encoded, cipher)
					if err != nil {
						t.Fatal(err)
					}

					var encrypted ably.Message
					err = json.Unmarshal(item.Encoded, &encrypted)
					if err != nil {
						t.Fatal(err)
					}
					encrypted, err = ably.MessageWithDecodedData(encrypted, cipher)
					if err != nil {
						t.Fatal(err)
					}

					if encoded.Name != encrypted.Name {
						t.Errorf("expected %s got %s", encoded.Name, encrypted.Name)
					}
					if !reflect.DeepEqual(encoded.Data, encrypted.Data) {
						t.Errorf("expected %s got %s :encoding %s",
							encoded.Data, encrypted.Data, encoded.Encoding)
					}
				}
			})
		})
	}
}
