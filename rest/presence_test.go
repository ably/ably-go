package rest_test

import (
	"time"

	"github.com/ably/ably-go/config"
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
				page, err := presence.Get(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(page.PresenceMessages())).To(Equal(len(testApp.Config.Channels[0].Presence)))
			})

			Context("with a limit option", func() {
				It("returns a paginated response", func() {
					page1, err := presence.Get(&config.PaginateParams{Limit: 2})
					Expect(err).NotTo(HaveOccurred())
					Expect(len(page1.PresenceMessages())).To(Equal(2))

					page2, err := page1.Next()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(page2.PresenceMessages())).To(Equal(2))

					_, err = page2.Next()
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Describe("History", func() {
			It("returns a list of presence messages", func() {
				page, err := presence.History(nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(page.PresenceMessages())).To(Equal(len(testApp.Config.Channels[0].Presence)))
			})

			Context("with start and end time", func() {
				It("can return older items from a certain date given a start / end timestamp", func() {
					params := &config.PaginateParams{
						ScopeParams: config.ScopeParams{
							Start: config.NewTimestamp(time.Now().Add(-24 * time.Hour)),
							End:   config.NewTimestamp(time.Now()),
						},
					}
					page, err := presence.History(params)
					Expect(err).NotTo(HaveOccurred())
					Expect(len(page.PresenceMessages())).To(Equal(len(testApp.Config.Channels[0].Presence)))
				})
			})

		})
	})
})
