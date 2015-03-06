package rest_test

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/protocol"
	"github.com/ably/ably-go/rest"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("Presence", func() {
	Context("tested against presence fixture data set up in test app", func() {
		var presence *rest.Presence

		BeforeEach(func() {
			channel = client.Channel("persisted:presence_fixtures")
			presence = channel.Presence
		})

		Describe("Get", func() {
			It("returns current members on the channel", func() {
				members, err := presence.Get(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(members).To(BeAssignableToTypeOf([]*protocol.PresenceMessage{}))
				Expect(len(members)).To(Equal(len(testApp.Config.Channels[0].Presence)))
			})

			Context("with a limit option", func() {
				It("returns a paginated response", func() {
					members, err := presence.Get(&config.PaginateParams{Limit: 2})
					Expect(err).NotTo(HaveOccurred())
					Expect(len(members)).To(Equal(2))
				})
			})
		})

		Describe("History", func() {
			It("returns a list of presence messages", func() {
				presenceMessages, err := presence.History(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(presenceMessages).To(BeAssignableToTypeOf([]*protocol.PresenceMessage{}))
				Expect(len(presenceMessages)).To(Equal(len(testApp.Config.Channels[0].Presence)))
			})
		})
	})
})
