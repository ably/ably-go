//go:build !unit
// +build !unit

package ably_test

import (
	"encoding/json"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"

	"github.com/stretchr/testify/assert"
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
					assert.NoError(t, err)

					encoded, err = ably.MessageWithDecodedData(encoded, cipher)
					assert.NoError(t, err)

					var encrypted ably.Message
					err = json.Unmarshal(item.Encoded, &encrypted)
					assert.NoError(t, err)

					encrypted, err = ably.MessageWithDecodedData(encrypted, cipher)
					assert.NoError(t, err)
					assert.Equal(t, encoded.Name, encrypted.Name,
						"expected %s got %s", encoded.Name, encrypted.Name)
					assert.Equal(t, encoded.Data, encrypted.Data,
						"expected %s got %s :encoding %s", encoded.Data, encrypted.Data, encoded.Encoding)
				}
			})
		})
	}
}

func TestMessage_CryptoDataFixtures_RSL6a1_RSL5b_RSL5c_TM3(t *testing.T) {
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
			assert.NoError(t, err)

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
					assert.NoError(t, err)

					encoded, err = ably.MessageWithDecodedData(encoded, cipher)
					assert.NoError(t, err)

					var encrypted ably.Message
					err = json.Unmarshal(item.Encoded, &encrypted)
					assert.NoError(t, err)

					encrypted, err = ably.MessageWithDecodedData(encrypted, cipher)
					assert.NoError(t, err)
					assert.Equal(t, encoded.Name, encrypted.Name)
					assert.Equal(t, encoded.Data, encrypted.Data)
				}
			})
		})
	}
}

