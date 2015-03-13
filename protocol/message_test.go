package protocol_test

import (
	"fmt"

	"github.com/ably/ably-go/protocol"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("Message", func() {
	var (
		message      *protocol.Message
		aes128Config map[string]string
	)

	BeforeEach(func() {
		aes128Config = map[string]string{
			"key": "\xb3\x25\x06\x00\x5c\x9d\x00\x27\x81\x7d\xdf\x81\xf3\x7f\xaa\xa7",
			"iv":  "\x88\xc2\x93\xd8\x81\x9d\x64\x53\x75\x3e\xb9\x66\xab\x02\x29\x93",
		}
	})

	Describe("DecodeData", func() {
		Context("with a json/utf-8 encoding", func() {
			BeforeEach(func() {
				message = &protocol.Message{Data: []byte(`{ "string": "utf-8™" }`), Encoding: "json/utf-8"}
			})

			It("returns a same string", func() {
				message.DecodeData(aes128Config)
				Expect(string(message.Data)).To(Equal(`{ "string": "utf-8™" }`))
			})
		})

		Context("with base64", func() {
			BeforeEach(func() {
				message = &protocol.Message{Data: []byte("dXRmLTjihKIK"), Encoding: "base64"}
			})

			It("decodes it into a byte array", func() {
				err := message.DecodeData(aes128Config)
				Expect(err).NotTo(HaveOccurred())
				Expect(message.Data).To(Equal([]byte("utf-8™\n")))
			})
		})

		Context("with json/utf-8/cipher+aes-128-cbc/base64", func() {
			BeforeEach(func() {
				message = &protocol.Message{
					Data:     []byte("iMKT2IGdZFN1PrlmqwIpk81cutSpuiuHPaE2IrRiRPboO+UoIr/cAY0i3z0oUOs2"),
					Encoding: "json/utf-8/cipher+aes-128-cbc/base64",
				}
			})

			It("decodes it into a byte array", func() {
				err := message.DecodeData(aes128Config)
				fmt.Printf("% x\n", message.Data)
				Expect(err).NotTo(HaveOccurred())
				Expect(message.Data).To(Equal([]byte(`{"string":"utf-8™"}`)))
			})
		})
	})
})
