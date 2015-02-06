package rest_test

import (
	"github.com/ably/ably-go"

	. "github.com/ably/ably-go/test/support"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	Describe("RequestToken", func() {
		It("gets a token from the API", func() {
			ttl := 60 * 60
			capability := &ably.Capability{"foo": []string{"publish"}}
			token, err := client.RequestToken(ttl, capability)

			Expect(err).NotTo(HaveOccurred())
			Expect(token.ID).To(ContainSubstring(TestAppInstance.Config.AppID))
			Expect(token.Key).To(Equal(TestAppInstance.AppKeyId()))
			Expect(token.Capability).To(Equal(capability))
		})
	})
})
