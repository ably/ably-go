package proto_test

import (
	"encoding/base64"
	"testing"

	"github.com/ably/ably-go/ably/ablytest"
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
		Encrypted: true,
		Cipher: proto.CipherParams{
			Key:       key,
			KeyLength: 128,
			IV:        iv,
			Algorithm: proto.AES,
		},
	}

	t.Run("with a json/utf-8 encoding", func(ts *testing.T) {
		ts.Run("it returns the same string", func(ts *testing.T) {
			message := &proto.Message{Data: `{ "string": "utf-8™" }`, Encoding: "json/utf-8"}
			err := message.DecodeData(&proto.ChannelOptions{
				Encrypted: true,
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
			if message.Data != expect {
				t.Errorf("expected %s got %s", expect, message.Data)
			}
		})
		ts.Run("can decode data without the aes config", func(ts *testing.T) {
			message := &proto.Message{Data: `{ "string": "utf-8™" }`, Encoding: "json/utf-8"}
			err := message.DecodeData()
			if err != nil {
				ts.Fatal(err)
			}
			expect := `{ "string": "utf-8™" }`
			if message.Data != expect {
				t.Errorf("expected %s got %s", expect, message.Data)
			}
		})
		ts.Run("leaves message intact with empty payload", func(ts *testing.T) {
			empty := &proto.Message{Encoding: "json/utf-8"}
			err := empty.DecodeData(nil)
			if err != nil {
				ts.Fatal(err)
			}
			expect := ""
			if empty.Data != expect {
				t.Errorf("expected %s got %s", expect, empty.Data)
			}
		})
	})

	t.Run("with base64", func(ts *testing.T) {
		ts.Run("decodes it into a byte array", func(ts *testing.T) {
			message := &proto.Message{Data: "dXRmLTjihKIK", Encoding: "base64"}
			err := message.DecodeData(opts)
			if err != nil {
				ts.Fatal(err)
			}
			expect := "utf-8™\n"
			if message.Data != expect {
				t.Errorf("expected %s got %s", expect, message.Data)
			}
		})
		ts.Run("can decode data without the channel options", func(ts *testing.T) {
			message := &proto.Message{Data: "dXRmLTjihKIK", Encoding: "base64"}
			err := message.DecodeData()
			if err != nil {
				ts.Fatal(err)
			}
			expect := "utf-8™\n"
			if message.Data != expect {
				t.Errorf("expected %s got %s", expect, message.Data)
			}
		})
	})
	t.Run("with json/utf-8/cipher+aes-128-cbc/base64", func(ts *testing.T) {
		encodedData := "HO4cYSP8LybPYBPZPHQOtvmStzmExkdjvrn51J6cmaTZrGl+EsJ61sgxmZ6j6jcA"
		decodedData := "[\"example\",\"json\",\"array\"]"
		ts.Run("decodes it into a byte array", func(ts *testing.T) {
			message := &proto.Message{
				Data:     encodedData,
				Encoding: "json/utf-8/cipher+aes-128-cbc/base64",
			}
			err := message.DecodeData(opts)
			if err != nil {
				ts.Fatal(err)
			}
			if message.Data != decodedData {
				t.Errorf("expected %s got %s", decodedData, message.Data)
			}
		})
		ts.Run("fails to decode data without an aes config", func(ts *testing.T) {
			message := &proto.Message{
				Data:     encodedData,
				Encoding: "json/utf-8/cipher+aes-128-cbc/base64",
			}
			err := message.DecodeData()
			if err == nil {
				ts.Fatal("expected an error")
			}
		})
		ts.Run("leaves message intact with empty payload", func(ts *testing.T) {
			message := &proto.Message{
				Encoding: "json/utf-8/cipher+aes-128-cbc/base64",
			}
			err := message.DecodeData()
			if err != nil {
				ts.Fatal(err)
			}
			expect := ""
			if message.Data != expect {
				ts.Errorf("expected %s got %s", expect, message.Data)
			}
		})
	})
}

func TestMessage_EncodeData(t *testing.T) {
	t.Run("with a json/utf-8 encoding", func(ts *testing.T) {
		message := &proto.Message{Data: `{ "string": "utf-8™" }`}
		encodeInto := "json/utf-8"
		message.EncodeData(encodeInto)
		expect := `{ "string": "utf-8™" }`
		if message.Data != expect {
			ts.Errorf("expected %s got %s", expect, message.Data)
		}
		if message.Encoding != encodeInto {
			t.Errorf("expected %s got %s", expect, message.Encoding)
		}
	})

	t.Run("with base64", func(ts *testing.T) {
		str := "utf8\n"
		encodeInto := "base64"
		expect := base64.StdEncoding.EncodeToString([]byte(str))

		message := &proto.Message{Data: str}
		err := message.EncodeData(encodeInto, nil, nil)
		if err != nil {
			ts.Fatal(err)
		}
		if message.Data != expect {
			t.Errorf("expected %s got %s", expect, message.Data)
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
			Encrypted: true,
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

		message := &proto.Message{Data: str}
		err = message.EncodeData(encodeInto, opts)
		if err != nil {
			ts.Fatal(err)
		}
		if message.Encoding != encodeInto {
			t.Errorf("expected %s got %s", encodeInto, message.Encoding)
		}

		ts.Run("is decode-able through the DecodeData method", func(ts *testing.T) {
			message := &proto.Message{Data: str}
			err = message.EncodeData(encodeInto, opts)
			if err != nil {
				ts.Fatal(err)
			}
			err := message.DecodeData(opts)
			if err != nil {
				ts.Error(err)
			}
		})
		if message.Data != encodedData {
			t.Errorf("expected %s got %s", encodedData, message.Data)
		}
	})
}

func TestMessage_CryptoDataFixtures(t *testing.T) {
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
				Encrypted: true,
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
					if item.Encoded.Data != item.Encrypted.Data {
						ts.Errorf("expected %s got %s :encoding %s",
							item.Encoded.Data, item.Encrypted.Data, item.Encrypted.Encoding)
					}
				}
			})
		})
	}
}
