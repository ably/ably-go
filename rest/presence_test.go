package rest_test

import (
	"time"

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
				paginatedMessages, err := presence.Get(nil)
				messages := paginatedMessages.Current
				Expect(err).NotTo(HaveOccurred())
				Expect(messages).To(BeAssignableToTypeOf([]*protocol.PresenceMessage{}))
				Expect(len(messages)).To(Equal(len(testApp.Config.Channels[0].Presence)))
			})

			Context("with a limit option", func() {
				It("returns a paginated response", func() {
					paginatedMessages, err := presence.Get(&config.PaginateParams{Limit: 2})
					messagesSet1 := paginatedMessages.Current
					Expect(err).NotTo(HaveOccurred())
					Expect(len(messagesSet1)).To(Equal(2))

					messagesSet2, err := paginatedMessages.NextPage()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(messagesSet2)).To(Equal(2))
					Expect(messagesSet1).NotTo(Equal(messagesSet2))

					_, err = paginatedMessages.NextPage()
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Describe("History", func() {
			It("returns a list of presence messages", func() {
				paginatedMessages, err := presence.History(nil)
				Expect(err).NotTo(HaveOccurred())

				messages := paginatedMessages.Current
				Expect(messages).To(BeAssignableToTypeOf([]*protocol.PresenceMessage{}))
				Expect(len(messages)).To(Equal(len(testApp.Config.Channels[0].Presence)))
			})

			Context("with start and end time", func() {
				It("can return older items from a certain date given a start / end timestamp", func() {
					params := &config.PaginateParams{
						ScopeParams: config.ScopeParams{
							Start: config.NewTimestamp(time.Now().Add(-24 * time.Hour)),
							End:   config.NewTimestamp(time.Now()),
						},
					}
					paginatedMessages, err := presence.History(params)
					Expect(err).NotTo(HaveOccurred())

					messages := paginatedMessages.Current
					Expect(messages).To(BeAssignableToTypeOf([]*protocol.PresenceMessage{}))
					Expect(len(messages)).To(Equal(len(testApp.Config.Channels[0].Presence)))
				})
			})

		})
	})
})
