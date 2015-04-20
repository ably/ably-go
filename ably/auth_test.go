package ably_test

import (
	"github.com/ably/ably-go/ably"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("Auth", func() {
	Describe("RequestToken", func() {
		It("gets a token from the API", func() {
			req := client.Auth.CreateTokenRequest()
			req.TTL = 60 * 60 * 1000
			req.Capability = ably.Capability{"foo": []string{"publish"}}
			token, err := client.Auth.RequestToken(req)

			Expect(err).NotTo(HaveOccurred())
			Expect(token.Token).To(ContainSubstring(testApp.Config.ApiID))
			Expect(token.KeyName).To(Equal(testApp.AppKeyId()))
			Expect(token.Capability).To(Equal(req.Capability))
		})
	})

	Describe("CreateTokenRequest", func() {
		It("gets a token from the API", func() {
			req := client.Auth.CreateTokenRequest()
			req.TTL = 60 * 60 * 1000
			req.Capability = ably.Capability{"foo": []string{"publish"}}

			Expect(req.KeyName).To(ContainSubstring(testApp.Config.ApiID))
			Expect(req.Mac).NotTo(BeNil())
		})
	})
})
