package ably_test

import (
	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/proto"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("RestChannel", func() {
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
			Expect(len(messages)).NotTo(Equal(0))
			Expect(messages[0].Name).To(Equal(event))
			Expect(messages[0].Data).To(Equal(message))
		})
	})

	Describe("History", func() {
		var historyRestChannel *ably.RestChannel

		BeforeEach(func() {
			historyRestChannel = client.Channel("history")

			for i := 0; i < 2; i++ {
				historyRestChannel.Publish("breakingnews", "Another Shark attack!!")
			}
		})

		It("returns a paginated result", func() {
			page1, err := historyRestChannel.History(&ably.PaginateParams{Limit: 1})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(page1.Messages())).To(Equal(1))
			Expect(len(page1.Items())).To(Equal(1))

			page2, err := page1.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(page2.Messages())).To(Equal(1))
			Expect(len(page2.Items())).To(Equal(1))
		})
	})

	Describe("PublishAll", func() {
		var encodingRestChannel *ably.RestChannel

		BeforeEach(func() {
			encodingRestChannel = client.Channel("encoding")
		})

		It("allows to send multiple messages at once", func() {
			messages := []*proto.Message{
				{Name: "send", Data: "test data 1"},
				{Name: "send", Data: "test data 2"},
			}
			err := encodingRestChannel.PublishAll(messages)
			Expect(err).NotTo(HaveOccurred())

			page, err := encodingRestChannel.History(&ably.PaginateParams{Limit: 2})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(page.Messages())).To(Equal(2))
			Expect(len(page.Items())).To(Equal(2))
		})
	})
})
