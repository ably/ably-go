package rest_test

import (
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/proto"
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
			page, err := channel.History(nil)
			Expect(err).NotTo(HaveOccurred())

			messages := page.Messages()
			Expect(messages[0].Name).To(Equal(event))
			Expect(messages[0].Data).To(Equal(message))
		})
	})

	Describe("History", func() {
		var historyChannel *rest.Channel

		BeforeEach(func() {
			historyChannel = client.Channel("history")

			for i := 0; i < 2; i++ {
				historyChannel.Publish("breakingnews", "Another Shark attack!!")
			}
		})

		It("returns a paginated result", func() {
			page1, err := historyChannel.History(&config.PaginateParams{Limit: 1})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(page1.Messages())).To(Equal(1))

			page2, err := page1.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(page2.Messages())).To(Equal(1))
		})
	})

	Describe("PublishAll", func() {
		var encodingChannel *rest.Channel

		BeforeEach(func() {
			encodingChannel = client.Channel("encoding")
		})

		It("allows to send multiple messages at once", func() {
			messages := []*proto.Message{
				{Name: "send", Data: "test data 1"},
				{Name: "send", Data: "test data 2"},
			}
			err := encodingChannel.PublishAll(messages)
			Expect(err).NotTo(HaveOccurred())

			page, err := encodingChannel.History(&config.PaginateParams{Limit: 2})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(page.Messages())).To(Equal(2))
		})
	})
})
