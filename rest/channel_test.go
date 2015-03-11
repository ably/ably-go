package rest_test

import (
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/rest"
)

var _ = Describe("Channel", func() {
	const (
		event   = "sendMessage"
		message = "A message in a bottle"
	)

	Describe("publishing a message", func() {
		It("does not raise an error", func() {
			err := channel.Publish(event, message)
			Expect(err).NotTo(HaveOccurred())
		})

		It("is available in the history", func() {
			paginatedMessages, err := channel.History(nil)
			Expect(err).NotTo(HaveOccurred())

			messages := paginatedMessages.Current
			Expect(messages[0].Name).To(Equal(event))
			Expect(messages[0].Data).To(Equal(message))
		})
	})

	Describe("History", func() {
		var historyChannel *rest.Channel

		BeforeEach(func() {
			historyChannel = client.Channel("history")

			for i := 0; i < 3; i++ {
				historyChannel.Publish("breakingnews", "Another Shark attack!!")
			}
		})

		It("returns a paginated result", func() {
			messages1, err := historyChannel.History(&config.PaginateParams{Limit: 1})
			Expect(err).NotTo(HaveOccurred())
			Expect(messages1).To(BeAssignableToTypeOf(&rest.PaginatedMessages{}))
			Expect(len(messages1.Current)).To(Equal(1))

			messages2, err := messages1.NextPage()
			Expect(err).NotTo(HaveOccurred())
			Expect(messages2).To(BeAssignableToTypeOf(&rest.PaginatedMessages{}))
			Expect(len(messages2.Current)).To(Equal(1))
		})
	})
})
