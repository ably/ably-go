//go:build !integration
// +build !integration

package ably_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/ably/ably-go/ably"

	"github.com/stretchr/testify/assert"
)

type custom struct{}

func (custom) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string{"custom"})
}

func TestMessage_EncodeDecode_TM3(t *testing.T) {
	key, err := base64.StdEncoding.DecodeString("WUP6u0K7MXI5Zeo0VppPwg==")
	assert.NoError(t, err)
	iv, err := base64.StdEncoding.DecodeString("HO4cYSP8LybPYBPZPHQOtg==")
	assert.NoError(t, err)
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
			desc:        "with valid utf-8 json data in string format",
			data:        `{"key":"value"}`,
			decoded:     `{"key":"value"}`,
			encodedJSON: `{"data":"{\"key\":\"value\"}"}`,
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
			desc:        "with valid utf-8 json data in string format and cipher enabled",
			data:        `{"key":"value"}`,
			decoded:     `{"key":"value"}`,
			opts:        opts,
			encodedJSON: `{"data":"HO4cYSP8LybPYBPZPHQOtlLxASbzZOh5h8lGaP3dX+M=","encoding":"utf-8/cipher+aes-128-cbc/base64"}`,
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
			assert.NoError(t, err)
			b, err := json.Marshal(msg)
			assert.NoError(t, err)
			assert.Equal(t, v.encodedJSON, string(b),
				"expected %s got %s", v.encodedJSON, string(b))

			var encoded ably.Message
			err = json.Unmarshal(b, &encoded)
			assert.NoError(t, err)
			decoded, err := ably.MessageWithDecodedData(encoded, cipher)
			assert.NoError(t, err)
			assert.Equal(t, v.decoded, decoded.Data,
				"expected %#v got %#v", v.decoded, decoded.Data)
		})
	}
}
