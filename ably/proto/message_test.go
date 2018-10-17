package proto_test

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"
)

type custom struct{}

func (custom) MarshalJSON() ([]byte, error) {
	return json.Marshal("custom")
}

func TestMessage(t *testing.T) {
	key, err := base64.StdEncoding.DecodeString("WUP6u0K7MXI5Zeo0VppPwg==")
	if err != nil {
		t.Fatal(err)
	}
	iv, err := base64.StdEncoding.DecodeString("HO4cYSP8LybPYBPZPHQOtg==")
	if err != nil {
		t.Fatal(err)
	}
	opts := &proto.ChannelOptions{
		Cipher: proto.CipherParams{
			Key:       key,
			KeyLength: 128,
			IV:        iv,
			Algorithm: proto.AES,
		},
	}
	sample := []struct {
		desc        string
		data        interface{}
		opts        *proto.ChannelOptions
		encodedJSON string
		decoded     interface{}
	}{
		{
			desc: "with a json/utf-8 encoding RSL4d3 mad data",
			data: map[string]interface{}{
				"string": proto.UTF8,
			},
			decoded: map[string]interface{}{
				"string": proto.UTF8,
			},
			encodedJSON: `{"data":"{\"string\":\"utf-8\"}","encoding":"json"}`,
		},
		{
			desc: "with a json/utf-8 encoding RSL4d3 array data",
			data: []int64{1, 2, 3},
			decoded: []interface{}{
				float64(1.0),
				float64(2.0),
				float64(3.0),
			},
			encodedJSON: `{"data":"[1,2,3]","encoding":"json"}`,
		},
		{
			desc:        "with a json/utf-8 encoding RSL4d3 json.Marshaler data",
			data:        custom{},
			encodedJSON: `{"data":"\"custom\"","encoding":"json"}`,
			decoded:     "custom",
		},
		{
			desc:        "with a json/utf-8 encoding RSL4d3 binary data",
			data:        []byte(proto.Base64),
			decoded:     []byte(proto.Base64),
			encodedJSON: `{"data":"YmFzZTY0","encoding":"base64"}`,
		},
		{
			desc: "with json/utf-8/cipher+aes-128-cbc/base64",
			data: map[string]interface{}{
				"string": `The quick brown fox jumped over the lazy dog`,
			},
			decoded: map[string]interface{}{
				"string": `The quick brown fox jumped over the lazy dog`,
			},
			opts:        opts,
			encodedJSON: `{"data":"HO4cYSP8LybPYBPZPHQOtlT0v5P4AF9H1o0CEftPkErqe+ebUOoIPB9eMrSy092XGb9jaq3PdU2qLwz1lRqtEuUMgX8zDmtkTkweJEpE81Y=","encoding":"json/utf-8/cipher+aes-128-cbc/base64"}`,
		},
	}

	for _, v := range sample {
		t.Run(v.desc, func(ts *testing.T) {
			msg := &proto.Message{
				Data:           v.data,
				ChannelOptions: v.opts,
			}
			b, err := json.Marshal(msg)
			if err != nil {
				ts.Fatal(err)
			}
			got := string(b)
			if got != v.encodedJSON {
				ts.Errorf("expected %s got %s", v.encodedJSON, got)
			}

			decoded := &proto.Message{
				ChannelOptions: v.opts,
			}
			err = decoded.UnmarshalJSON(b)
			if err != nil {
				ts.Fatal(err)
			}
			if !reflect.DeepEqual(decoded.Data, v.decoded) {
				ts.Errorf("expected %#v got %v", v.decoded, decoded.Data)
			}
		})
	}
}

func TestMessage_CryptoDataFixtures_RSL6a1_RSL5b_RSL5c(t *testing.T) {
	fixtures := []struct {
		desc, file string
		keylength  int
	}{
		{"with a 128 keylength", "test-resources/crypto-data-128.json", 128},
		{"with a 256 keylength", "test-resources/crypto-data-256.json", 126},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.desc, func(ts *testing.T) {
			test, key, iv, err := ablytest.LoadCryptoData(fixture.file)
			if err != nil {
				ts.Fatal(err)
			}
			opts := &proto.ChannelOptions{
				Cipher: proto.CipherParams{
					Algorithm: proto.AES,
					Key:       key,
					IV:        iv,
				},
			}
			ts.Run("fixture encode", func(ts *testing.T) {
				for _, item := range test.Items {
					// All test-cases from the common fixtures files are encoded
					// for binary transports. Decode the input message first,
					// to ensure we're not encrypting base64d payloads.
					encoded := &proto.Message{ChannelOptions: opts}
					err := encoded.FromMap(item.Encoded)
					if err != nil {
						ts.Fatal(err)
					}
					encrypted := &proto.Message{ChannelOptions: opts}
					err = encrypted.FromMap(item.Encrypted)
					if err != nil {
						ts.Fatal(err)
					}
					if encoded.Name != encrypted.Name {
						ts.Errorf("expected %s got %s", encoded.Name, encrypted.Name)
					}
					if !reflect.DeepEqual(encoded.Data, encrypted.Data) {
						ts.Errorf("expected %s got %s :encoding %s",
							encoded.Data, encrypted.Data, encoded.Encoding)
					}
				}
			})
		})
	}
}
