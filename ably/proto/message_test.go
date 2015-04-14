package proto_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ably/ably-go/ably/proto"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

type CryptoData struct {
	Algorithm string `json:"algorithm"`
	Mode      string `json:"mode"`
	KeyLen    int    `json:"keylength"`
	Key       string `json:"key"`
	IV        string `json:"iv"`
	Items     []struct {
		Encoded   proto.Message `json:"encoded"`
		Encrypted proto.Message `json:"encrypted"`
	} `json:"items"`
}

func load(rel string, t GinkgoTInterface) (*CryptoData, map[string]string) {
	data := &CryptoData{}
	f, err := os.Open(filepath.Join("..", "..", "common", filepath.FromSlash(rel)))
	if err != nil {
		t.Skip("ensure the git submodules are initialized", err)
	}
	err = json.NewDecoder(f).Decode(data)
	f.Close()
	if err != nil {
		t.Fatal("unable to unmarshal test cases", err)
	}
	key, err := base64.StdEncoding.DecodeString(data.Key)
	if err != nil {
		t.Fatal("unable to unbase64 key", err)
	}
	iv, err := base64.StdEncoding.DecodeString(data.IV)
	if err != nil {
		t.Fatal("unable to unbase64 IV", err)
	}
	return data, map[string]string{"key": string(key), "iv": string(iv)}
}

var _ = Describe("Message", func() {
	var (
		message      *proto.Message
		aes128Config map[string]string
	)

	BeforeEach(func() {
		key, err := base64.StdEncoding.DecodeString("WUP6u0K7MXI5Zeo0VppPwg==")
		Expect(err).NotTo(HaveOccurred())

		aes128Config = map[string]string{
			"key": string(key),
			"iv":  "",
		}
	})

	Describe("DecodeData", func() {
		Context("with a json/utf-8 encoding", func() {
			BeforeEach(func() {
				message = &proto.Message{Data: `{ "string": "utf-8™" }`, Encoding: "json/utf-8"}
			})

			It("returns the same string", func() {
				err := message.DecodeData(aes128Config)
				Expect(err).NotTo(HaveOccurred())
				Expect(message.Data).To(Equal(`{ "string": "utf-8™" }`))
			})

			It("can decode data without the aes config", func() {
				err := message.DecodeData(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(message.Data).To(Equal(`{ "string": "utf-8™" }`))
			})

			It("leaves message intact with empty payload", func() {
				empty := &proto.Message{Encoding: message.Encoding}
				err := empty.DecodeData(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(empty).To(Equal(&proto.Message{Encoding: message.Encoding}))
			})
		})

		Context("with base64", func() {
			BeforeEach(func() {
				message = &proto.Message{Data: "dXRmLTjihKIK", Encoding: "base64"}
			})

			It("decodes it into a byte array", func() {
				err := message.DecodeData(aes128Config)
				Expect(err).NotTo(HaveOccurred())
				Expect(message.Data).To(Equal("utf-8™\n"))
			})

			It("can decode data without the aes config", func() {
				err := message.DecodeData(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(message.Data).To(Equal("utf-8™\n"))
			})

			It("leaves message intact with empty payload", func() {
				empty := &proto.Message{Encoding: message.Encoding}
				err := empty.DecodeData(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(empty).To(Equal(&proto.Message{Encoding: message.Encoding}))
			})
		})

		Context("with json/utf-8/cipher+aes-128-cbc/base64", func() {
			var (
				encodedData string
				decodedData string
			)

			BeforeEach(func() {
				encodedData = "HO4cYSP8LybPYBPZPHQOtvmStzmExkdjvrn51J6cmaTZrGl+EsJ61sgxmZ6j6jcA"
				decodedData = "[\"example\",\"json\",\"array\"]"
				message = &proto.Message{
					Data:     encodedData,
					Encoding: "json/utf-8/cipher+aes-128-cbc/base64",
				}
			})

			It("decodes it into a byte array", func() {
				err := message.DecodeData(aes128Config)
				Expect(err).NotTo(HaveOccurred())
				Expect(message.Data).To(Equal(decodedData))
			})

			It("fails to decode data without an aes config", func() {
				err := message.DecodeData(nil)
				Expect(err).To(HaveOccurred())
			})

			It("leaves message intact with empty payload", func() {
				empty := &proto.Message{Encoding: message.Encoding}
				err := empty.DecodeData(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(empty).To(Equal(&proto.Message{Encoding: message.Encoding}))
			})
		})
	})

	Describe("EncodeData", func() {
		var encodeInto string

		Context("with a json/utf-8 encoding", func() {
			BeforeEach(func() {
				message = &proto.Message{Data: `{ "string": "utf-8™" }`}
				encodeInto = "json/utf-8"

				err := message.EncodeData(encodeInto, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the same string", func() {
				Expect(message.Data).To(Equal(`{ "string": "utf-8™" }`))
			})

			It("sets the encoding to json/utf-8", func() {
				Expect(message.Encoding).To(Equal(encodeInto))
			})
		})

		Context("with base64", func() {
			var (
				str       string
				base64Str string
			)

			BeforeEach(func() {
				str = "utf8\n"
				encodeInto = "base64"
				base64Str = base64.StdEncoding.EncodeToString([]byte(str))

				message = &proto.Message{Data: str}
				err := message.EncodeData(encodeInto, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the base64 encoded string", func() {
				Expect(message.Data).To(Equal(base64Str))
			})

			It("sets the encoding to json/utf-8", func() {
				Expect(message.Encoding).To(Equal(encodeInto))
			})
		})

		Context("with json/utf-8/cipher+aes-128-cbc/base64", func() {
			var (
				str         string
				encodedData string
			)

			BeforeEach(func() {
				str = `The quick brown fox jumped over the lazy dog`
				encodedData = "HO4cYSP8LybPYBPZPHQOtmHItcxYdSvcNUC6kXVpMn0VFL+9z2/5tJ6WFbR0SBT1xhFRuJ+MeBGTU3yOY9P5ow=="
				encodeInto = "utf-8/cipher+aes-128-cbc/base64"

				iv, err := base64.StdEncoding.DecodeString("HO4cYSP8LybPYBPZPHQOtg==")
				Expect(err).NotTo(HaveOccurred())

				aes128Config["iv"] = string(iv)

				message = &proto.Message{Data: str}
				err = message.EncodeData(encodeInto, aes128Config)
				Expect(err).NotTo(HaveOccurred())
			})

			It("inserts the encoding in the Encoding field", func() {
				Expect(message.Encoding).To(Equal(encodeInto))
			})

			It("is decode-able through the DecodeData method", func() {
				err := message.DecodeData(aes128Config)
				Expect(err).NotTo(HaveOccurred())
				Expect(message.Data).To(Equal(str))
			})

			It("has the expected encoded value", func() {
				Expect(message.Data).To(Equal(encodedData))
			})
		})
	})

	acceptance := func(fixture string, t GinkgoTInterface) {
		test, cfg := load(fixture, t)

		It("fixture decode", func() {
			for _, item := range test.Items {
				err := item.Encoded.DecodeData(nil)
				Expect(err).NotTo(HaveOccurred())
				err = item.Encrypted.DecodeData(cfg)
				Expect(err).NotTo(HaveOccurred())
				Expect(item.Encrypted.Name).To(Equal(item.Encoded.Name))
				Expect(item.Encrypted.Data).To(Equal(item.Encoded.Data))
			}
		})

		return

		It("fixture encode", func() {
			for _, item := range test.Items {
				err := item.Encoded.EncodeData(item.Encrypted.Encoding, cfg)
				Expect(err).NotTo(HaveOccurred())
				Expect(item.Encoded.Name).To(Equal(item.Encrypted.Name))
				Expect(item.Encoded.Data).To(Equal(item.Encrypted.Data))
			}
		})
	}

	Describe("CryptoDataFixtures", func() {
		Context("with a 128 keylength", func() {
			acceptance("test-resources/crypto-data-128.json", GinkgoT())
		})

		Context("with a 256 keylength", func() {
			acceptance("test-resources/crypto-data-256.json", GinkgoT())
		})
	})
})
