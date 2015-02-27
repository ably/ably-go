package ably_test

import (
	"log"

	"github.com/ably/ably-go"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ClientOptions", func() {
	var (
		clientOptions *ably.ClientOptions
		buffer        *gbytes.Buffer
	)

	BeforeEach(func() {
		buffer = gbytes.NewBuffer()

		clientOptions = &ably.ClientOptions{
			ApiKey: "id:secret",
		}
	})

	It("parses ApiKey into a set of known parameters", func() {
		Expect(clientOptions.ToParams().AppID).To(Equal("id"))
		Expect(clientOptions.ToParams().AppSecret).To(Equal("secret"))
	})

	Context("when ApiKey is invalid", func() {
		BeforeEach(func() {
			clientOptions = &ably.ClientOptions{
				ApiKey: "invalid",
				Logger: log.New(buffer, "", log.Lmicroseconds|log.Llongfile),
			}

			clientOptions.ToParams()
		})

		It("prints an error", func() {
			Expect(string(buffer.Contents())).To(
				ContainSubstring("ERRO: ApiKey doesn't use the right format. Ignoring this parameter"),
			)
		})
	})
})
