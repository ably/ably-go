package config_test

import (
	"log"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
	"github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega/gbytes"
	"github.com/ably/ably-go/config"
)

var _ = Describe("Params", func() {
	var (
		params *config.Params
		buffer *gbytes.Buffer
	)

	BeforeEach(func() {
		buffer = gbytes.NewBuffer()

		params = &config.Params{
			ApiKey: "id:secret",
		}

		params.Prepare()
	})

	It("parses ApiKey into a set of known parameters", func() {
		Expect(params.AppID).To(Equal("id"))
		Expect(params.AppSecret).To(Equal("secret"))
	})

	Context("when ApiKey is invalid", func() {
		BeforeEach(func() {
			params = &config.Params{
				ApiKey: "invalid",
				AblyLogger: &config.AblyLogger{
					Logger: log.New(buffer, "", log.Lmicroseconds|log.Llongfile),
				},
			}

			params.Prepare()
		})

		It("prints an error", func() {
			Expect(string(buffer.Contents())).To(
				ContainSubstring("ERRO: ApiKey doesn't use the right format. Ignoring this parameter"),
			)
		})
	})
})
