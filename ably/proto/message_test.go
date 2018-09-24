package proto_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ably/proto"
)

func TestMessage_DecodeData(t *testing.T) {
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

	t.Run("with a json/utf-8 encoding", func(ts *testing.T) {
		ts.Run("it returns the same string", func(ts *testing.T) {
			data := `{ "string": "utf-8™" }`
			message := &proto.Message{
				Data: &proto.DataValue{
					Value: data,
				},
				Encoding: "json/utf-8",
			}
			err := message.DecodeData(&proto.ChannelOptions{
				Cipher: proto.CipherParams{
					Key:       key,
					KeyLength: 128,
					Algorithm: proto.AES,
				},
			})
			if err != nil {
				ts.Fatal(err)
			}
			expect := `{ "string": "utf-8™" }`
			if message.Data.ToString() != expect {
				t.Errorf("expected %s got %s", expect, message.Data.ToString())
			}
		})
		ts.Run("can decode data without the aes config", func(ts *testing.T) {
			message := &proto.Message{
				Data: &proto.DataValue{
					Value: `{ "string": "utf-8™" }`,
				},
				Encoding: "json/utf-8",
			}
			err := message.DecodeData(nil)
			if err != nil {
				ts.Fatal(err)
			}
			expect := `{ "string": "utf-8™" }`
			if message.Data.ToString() != expect {
				t.Errorf("expected %s got %s", expect, message.Data.ToString())
			}
		})
		ts.Run("leaves message intact with empty payload", func(ts *testing.T) {
			empty := &proto.Message{Encoding: "json/utf-8"}
			err := empty.DecodeData(nil)
			if err != nil {
				ts.Fatal(err)
			}
			if empty.Data != nil {
				t.Error("expected data to be nil")
			}
		})
	})

	t.Run("with base64", func(ts *testing.T) {
		ts.Run("decodes it into a byte array", func(ts *testing.T) {
			message := &proto.Message{
				Data: &proto.DataValue{
					Value: "dXRmLTjihKIK",
				},
				Encoding: "base64",
			}
			err := message.DecodeData(opts)
			if err != nil {
				ts.Fatal(err)
			}
			expect := []byte("utf-8™\n")
			got := message.Data.ToBytes()

			if !bytes.Equal(got, expect) {
				t.Errorf("expected %s got %s", string(expect), string(got))
			}
		})
		ts.Run("can decode data without the channel options", func(ts *testing.T) {
			message := &proto.Message{
				Data: &proto.DataValue{
					Value: "dXRmLTjihKIK",
				},
				Encoding: "base64",
			}
			err := message.DecodeData(nil)
			if err != nil {
				ts.Fatal(err)
			}
			expect := []byte("utf-8™\n")
			got := message.Data.ToBytes()
			if !bytes.Equal(got, expect) {
				t.Errorf("expected %s got %s", string(expect), string(got))
			}
		})
	})
	t.Run("with json/utf-8/cipher+aes-128-cbc/base64", func(ts *testing.T) {
		encodedData := "HO4cYSP8LybPYBPZPHQOtvmStzmExkdjvrn51J6cmaTZrGl+EsJ61sgxmZ6j6jcA"
		decodedData := "[\"example\",\"json\",\"array\"]"
		ts.Run("decodes it into a byte array", func(ts *testing.T) {
			message := &proto.Message{
				Data: &proto.DataValue{
					Value: encodedData,
				},
				Encoding: "json/utf-8/cipher+aes-128-cbc/base64",
			}
			err := message.DecodeData(opts)
			if err != nil {
				ts.Fatal(err)
			}
			expect := []byte(decodedData)
			got := message.Data.ToBytes()
			if !bytes.Equal(got, expect) {
				t.Errorf("expected %s got %s", string(expect), string(got))
			}
		})
		ts.Run("fails to decode data without an aes config", func(ts *testing.T) {
			message := &proto.Message{
				Data: &proto.DataValue{
					Value: encodedData,
				},
				Encoding: "json/utf-8/cipher+aes-128-cbc/base64",
			}
			err := message.DecodeData(nil)
			if err == nil {
				ts.Fatal("expected an error")
			}
		})
		ts.Run("leaves message intact with empty payload", func(ts *testing.T) {
			message := &proto.Message{
				Encoding: "json/utf-8/cipher+aes-128-cbc/base64",
			}
			err := message.DecodeData(nil)
			if err != nil {
				ts.Fatal(err)
			}
			if message.Data != nil {
				t.Error("expected data to be nil")
			}
		})
	})
}

func TestMessage_EncodeData(t *testing.T) {
	t.Run("with a json/utf-8 encoding RSL4d3", func(ts *testing.T) {
		message := &proto.Message{Data: &proto.DataValue{
			Value: `{ "string": "utf-8™" }`,
		}}
		encodeInto := "json/utf-8"
		message.EncodeData(encodeInto, nil)
		expect := `{ "string": "utf-8™" }`
		if message.Data.ToString() != expect {
			ts.Errorf("expected %s got %s", expect, message.Data.ToString())
		}
		if message.Encoding != encodeInto {
			t.Errorf("expected %s got %s", expect, message.Encoding)
		}
	})

	t.Run("with base64", func(ts *testing.T) {
		str := "utf8\n"
		encodeInto := "base64"
		expect := base64.StdEncoding.EncodeToString([]byte(str))

		message := &proto.Message{Data: &proto.DataValue{
			Value: str,
		}}
		err := message.EncodeData(encodeInto, nil)
		if err != nil {
			ts.Fatal(err)
		}
		if message.Data.ToString() != expect {
			t.Errorf("expected %s got %s", expect, message.Data.ToString())
		}
		if message.Encoding != encodeInto {
			t.Errorf("expected %s got %s", encodeInto, message.Encoding)
		}
	})
	t.Run("with json/utf-8/cipher+aes-128-cbc/base64", func(ts *testing.T) {
		key, err := base64.StdEncoding.DecodeString("WUP6u0K7MXI5Zeo0VppPwg==")
		if err != nil {
			ts.Fatal(err)
		}
		iv, err := base64.StdEncoding.DecodeString("HO4cYSP8LybPYBPZPHQOtg==")
		if err != nil {
			ts.Fatal(err)
		}
		opts := &proto.ChannelOptions{
			Cipher: proto.CipherParams{
				Key:       key,
				KeyLength: 128,
				IV:        iv,
				Algorithm: proto.AES,
			},
		}
		str := `The quick brown fox jumped over the lazy dog`
		encodedData := "HO4cYSP8LybPYBPZPHQOtmHItcxYdSvcNUC6kXVpMn0VFL+9z2/5tJ6WFbR0SBT1xhFRuJ+MeBGTU3yOY9P5ow=="
		encodeInto := "utf-8/cipher+aes-128-cbc/base64"

		message := &proto.Message{Data: &proto.DataValue{
			Value: str,
		}}
		err = message.EncodeData(encodeInto, opts)
		if err != nil {
			ts.Fatal(err)
		}
		if message.Encoding != encodeInto {
			t.Errorf("expected %s got %s", encodeInto, message.Encoding)
		}

		ts.Run("is decode-able through the DecodeData method", func(ts *testing.T) {
			message := &proto.Message{Data: &proto.DataValue{
				Value: str,
			}}
			err = message.EncodeData(encodeInto, opts)
			if err != nil {
				ts.Fatal(err)
			}
			err := message.DecodeData(opts)
			if err != nil {
				ts.Error(err)
			}
		})
		if message.Data.ToString() != encodedData {
			t.Errorf("expected %s got %s", encodedData, message.Data.ToString())
		}
	})
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
					err := item.Encoded.DecodeData(nil)
					if err != nil {
						ts.Fatal(err)
					}
					err = item.Encoded.EncodeData(item.Encrypted.Encoding, opts)
					if err != nil {
						ts.Error(err)
					}
					if item.Encoded.Name != item.Encrypted.Name {
						ts.Errorf("expected %s got %s", item.Encoded.Name, item.Encrypted.Name)
					}
					if !reflect.DeepEqual(item.Encoded.Data, item.Encrypted.Data) {
						ts.Errorf("expected %s got %s :encoding %s",
							item.Encoded.Data.Value, item.Encrypted.Data.Value, item.Encrypted.Encoding)
					}
				}
			})
		})
	}
}

func TestDataValue(t *testing.T) {
	type Value struct {
		Data *proto.DataValue
	}

	t.Run("marshals/unmarshal to json", func(ts *testing.T) {
		sample := []struct {
			src    interface{}
			expect string
			empty  interface{}
		}{
			{src: "hello", expect: `{"Data":"hello"}`, empty: ""},
			{src: []byte("hello"), expect: `{"Data":"aGVsbG8="}`, empty: []byte{}},
			{src: map[string]interface{}{
				"key": "value",
			}, expect: `{"Data":{"key":"value"}}`, empty: map[string]interface{}{}},
			{src: []int{1, 2, 3}, expect: `{"Data":[1,2,3]}`, empty: []int{}},
		}
		for _, v := range sample {
			d, err := proto.NewDataValue(v.src)
			if err != nil {
				ts.Fatal(err)
			}
			value := Value{Data: d}
			b, err := json.Marshal(value)
			if err != nil {
				ts.Fatal(err)
			}
			got := string(b)
			if got != v.expect {
				t.Errorf("expected %s got %s", v.expect, got)
			}

			d, err = proto.NewDataValue(v.empty)
			if err != nil {
				ts.Fatal(err)
			}
			value = Value{Data: d}
			err = json.Unmarshal(b, &value)
			if err != nil {
				ts.Error(err)
			}
			if !reflect.DeepEqual(value.Data.Value, v.src) {
				ts.Errorf("expected %v got %v", v.src, value.Data.Value)
			}
		}
	})
	t.Run("marshals/unmarshal to msgpack", func(ts *testing.T) {
		sample := []struct {
			reason string
			src    interface{}
			expect string
			empty  interface{}
		}{
			{reason: "string", src: "hello", expect: `gaREYXRhpWhlbGxv`, empty: ""},
			{reason: "raw bytes", src: []byte("hello"), expect: `gaREYXRhpWhlbGxv`, empty: []byte{}},
			{reason: "more raw bytes", src: []byte("hello, some  \""), expect: `gaREYXRhrmhlbGxvLCBzb21lICAi`, empty: []byte{}},
			{reason: "map", src: map[string]interface{}{
				"key": "value",
			}, expect: `gaREYXRhr3sia2V5IjoidmFsdWUifQ==`, empty: map[string]interface{}{}},
			{reason: "array", src: []int{1, 2, 3}, expect: `gaREYXRhp1sxLDIsM10=`, empty: []int{}},
		}
		for _, v := range sample {
			d, err := proto.NewDataValue(v.src)
			if err != nil {
				ts.Fatal(err)
			}
			value := Value{Data: d}
			b, err := ablyutil.Marshal(value)
			if err != nil {
				ts.Fatalf("%s %v", v.reason, err)
			}
			e := base64.StdEncoding.EncodeToString(b)
			got := e
			if got != v.expect {
				t.Errorf("expected %s got %s", v.expect, got)
			}

			d, err = proto.NewDataValue(v.empty)
			if err != nil {
				ts.Fatalf("%s %v", v.reason, err)
			}
			value = Value{Data: d}
			err = ablyutil.Unmarshal(b, &value)
			if err != nil {
				ts.Fatalf("%s %v %v", v.reason, err, string(b))
			}
			if !reflect.DeepEqual(value.Data.Value, v.src) {
				ts.Errorf("expected %v got %v", v.src, value.Data.Value)
			}
		}
	})
}
