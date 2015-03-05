package rest_test

import (
	"github.com/ably/ably-go/protocol"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = FDescribe("Presence", func() {
	BeforeEach(func() {
		channel = client.Channel("persisted:presence_fixtures")
	})

	Describe("Get", func() {
		It("returns a list of members", func() {
			members, err := channel.Presence.Get()
			Expect(err).NotTo(HaveOccurred())
			Expect(members).To(BeAssignableToTypeOf([]*protocol.PresenceMessage{}))
			Expect(len(members)).To(Equal(len(testApp.Config.Channels[0].Presence)))
		})
	})

	Describe("History", func() {
		It("returns a list of presence messages", func() {
			presenceMessages, err := channel.Presence.History()
			Expect(err).NotTo(HaveOccurred())
			Expect(presenceMessages).To(BeAssignableToTypeOf([]*protocol.PresenceMessage{}))
			Expect(len(presenceMessages)).To(Equal(len(testApp.Config.Channels[0].Presence)))
		})
	})
})
