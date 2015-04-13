package ably_test

import (
	"github.com/ably/ably-go/ably"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("Auth", func() {
	Describe("RequestToken", func() {
		It("gets a token from the API", func() {
			ttl := 60 * 60
			capability := ably.Capability{"foo": []string{"publish"}}
			token, err := client.Auth.RequestToken(ttl, capability)

			Expect(err).NotTo(HaveOccurred())
			Expect(token.ID).To(ContainSubstring(testApp.Config.AppID))
			Expect(token.Key).To(Equal(testApp.AppKeyId()))
			Expect(token.Capability).To(Equal(capability))
		})
	})

	Describe("CreateTokenRequest", func() {
		It("gets a token from the API", func() {
			ttl := 60 * 60
			capability := ably.Capability{"foo": []string{"publish"}}
			tokenRequest := client.Auth.CreateTokenRequest(ttl, capability)

			Expect(tokenRequest.ID).To(ContainSubstring(testApp.Config.AppID))
			Expect(tokenRequest.Mac).NotTo(BeNil())
		})
	})
})
