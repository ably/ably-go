package proto_test

import (
	"encoding/base64"

	"github.com/ably/ably-go/proto"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("Message", func() {
	var (
		message      *proto.Message
		aes128Config map[string]string
	)

	BeforeEach(func() {
		aes128Config = map[string]string{
			"key": "\xb3\x25\x06\x00\x5c\x9d\x00\x27\x81\x7d\xdf\x81\xf3\x7f\xaa\xa7",
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
		})

		Context("with json/utf-8/cipher+aes-128-cbc/base64", func() {
			BeforeEach(func() {
				message = &proto.Message{
					Data:     "iMKT2IGdZFN1PrlmqwIpk81cutSpuiuHPaE2IrRiRPboO+UoIr/cAY0i3z0oUOs2",
					Encoding: "json/utf-8/cipher+aes-128-cbc/base64",
				}
			})

			It("decodes it into a byte array", func() {
				err := message.DecodeData(aes128Config)
				Expect(err).NotTo(HaveOccurred())
				Expect(message.Data).To(Equal(`{"string":"utf-8™"}`))
			})

			It("fails to decode data without an aes config", func() {
				err := message.DecodeData(nil)
				Expect(err).To(HaveOccurred())
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
			var str string

			BeforeEach(func() {
				str = `{"string":"utf-8™"}`
				encodeInto = "json/utf-8/cipher+aes-128-cbc/base64"

				message = &proto.Message{Data: str}
				err := message.EncodeData(encodeInto, aes128Config)
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
		})
	})
})
