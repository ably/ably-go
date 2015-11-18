package proto_test

import (
	"encoding/base64"

	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("Message", func() {
	var (
		message   *proto.Message
		aes128Key []byte
		aes128IV  []byte
	)

	BeforeEach(func() {
		key, err := base64.StdEncoding.DecodeString("WUP6u0K7MXI5Zeo0VppPwg==")
		Expect(err).NotTo(HaveOccurred())

		aes128Key = key
	})

	Describe("DecodeData", func() {
		Context("with a json/utf-8 encoding", func() {
			BeforeEach(func() {
				message = &proto.Message{Data: `{ "string": "utf-8™" }`, Encoding: "json/utf-8"}
			})

			It("returns the same string", func() {
				err := message.DecodeData(aes128Key)
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
				err := message.DecodeData(aes128Key)
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
				err := message.DecodeData(aes128Key)
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

				err := message.EncodeData(encodeInto, nil, nil)
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
				err := message.EncodeData(encodeInto, nil, nil)
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

				aes128IV = iv

				message = &proto.Message{Data: str}
				err = message.EncodeData(encodeInto, aes128Key, aes128IV)
				Expect(err).NotTo(HaveOccurred())
			})

			It("inserts the encoding in the Encoding field", func() {
				Expect(message.Encoding).To(Equal(encodeInto))
			})

			It("is decode-able through the DecodeData method", func() {
				err := message.DecodeData(aes128Key)
				Expect(err).NotTo(HaveOccurred())
				Expect(message.Data).To(Equal(str))
			})

			It("has the expected encoded value", func() {
				Expect(message.Data).To(Equal(encodedData))
			})
		})
	})

	Describe("CryptoDataFixtures", func() {
		EncodeDecodeFixture := func(fixture string) func() {
			return func() {
				var (
					test    *ablytest.CryptoData
					key, iv []byte
				)

				BeforeEach(func() {
					var err error
					test, key, iv, err = ablytest.LoadCryptoData(fixture)
					Expect(err).NotTo(HaveOccurred())
				})

				It("fixture decode", func() {
					for _, item := range test.Items {
						// All test-cases from the common fixtures files are encoded
						// for binary transports. Decode the input message first,
						// to ensure we're not comparing decoded binary payloads
						// with its base64.
						err := item.Encoded.DecodeData(nil)
						Expect(err).NotTo(HaveOccurred())

						err = item.Encrypted.DecodeData(key)
						Expect(err).NotTo(HaveOccurred())
						Expect(item.Encrypted.Name).To(Equal(item.Encoded.Name))
						Expect(item.Encrypted.Data).To(Equal(item.Encoded.Data))
					}
				})

				It("fixture encode", func() {
					for _, item := range test.Items {
						// All test-cases from the common fixtures files are encoded
						// for binary transports. Decode the input message first,
						// to ensure we're not encrypting base64d payloads.
						err := item.Encoded.DecodeData(nil)
						Expect(err).NotTo(HaveOccurred())

						err = item.Encoded.EncodeData(item.Encrypted.Encoding, key, iv)
						Expect(err).NotTo(HaveOccurred())
						Expect(item.Encoded.Name).To(Equal(item.Encrypted.Name))
						Expect(item.Encoded.Data).To(Equal(item.Encrypted.Data))
					}
				})
			}
		}

		Context("with a 128 keylength", EncodeDecodeFixture("test-resources/crypto-data-128.json"))
		Context("with a 256 keylength", EncodeDecodeFixture("test-resources/crypto-data-256.json"))
	})
})
