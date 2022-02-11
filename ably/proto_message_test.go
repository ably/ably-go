//go:build !integration
// +build !integration

package ably_test

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ably/ably-go/ably"
)

type custom struct{}

func (custom) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string{"custom"})
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
	params := ably.CipherParams{
		Key:       key,
		KeyLength: 128,
		Algorithm: ably.CipherAES,
	}
	params.SetIV(iv)
	opts := &ably.ProtoChannelOptions{
		Cipher: params,
	}
	sample := []struct {
		desc        string
		data        interface{}
		opts        *ably.ProtoChannelOptions
		encodedJSON string
		decoded     interface{}
	}{
		{
			// utf-8 string data should not have an encoding set, see:
			// https://github.com/ably/docs/issues/1165
			desc:        "with valid utf-8 string data",
			data:        "a string",
			decoded:     "a string",
			encodedJSON: `{"data":"a string"}`,
		},
		{
			// invalid utf-8 string data should be base64 encoded
			desc:        "with invalid utf-8 string data",
			data:        "\xf0\x80\x80",
			decoded:     []byte("\xf0\x80\x80"),
			encodedJSON: `{"data":"8ICA","encoding":"base64"}`,
		},
		{
			desc: "with a json encoding RSL4d3 map data",
			data: map[string]interface{}{
				"string": ably.EncUTF8,
			},
			decoded: map[string]interface{}{
				"string": ably.EncUTF8,
			},
			encodedJSON: `{"data":"{\"string\":\"utf-8\"}","encoding":"json"}`,
		},
		{
			desc: "with a json encoding RSL4d3 array data",
			data: []int64{1, 2, 3},
			decoded: []interface{}{
				float64(1.0),
				float64(2.0),
				float64(3.0),
			},
			encodedJSON: `{"data":"[1,2,3]","encoding":"json"}`,
		},
		{
			desc:        "with a json encoding RSL4d3 json.Marshaler data",
			data:        custom{},
			encodedJSON: `{"data":"[\"custom\"]","encoding":"json"}`,
			decoded:     []interface{}{"custom"},
		},
		{
			desc:        "with a base64 encoding RSL4d3 binary data",
			data:        []byte(ably.EncBase64),
			decoded:     []byte(ably.EncBase64),
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
		// pin
		v := v
		t.Run(v.desc, func(t *testing.T) {
			cipher, _ := v.opts.GetCipher()
			msg, err := ably.MessageWithEncodedData(ably.Message{
				Data: v.data,
			}, cipher)
			if err != nil {
				t.Fatal(err)
			}
			b, err := json.Marshal(msg)
			if err != nil {
				t.Fatal(err)
			}
			got := string(b)
			if got != v.encodedJSON {
				t.Errorf("expected %s got %s", v.encodedJSON, got)
			}

			var encoded ably.Message
			err = json.Unmarshal(b, &encoded)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := ably.MessageWithDecodedData(encoded, cipher)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decoded.Data, v.decoded) {
				t.Errorf("expected %#v got %#v", v.decoded, decoded.Data)
			}
		})
	}
}
