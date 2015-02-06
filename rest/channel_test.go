package rest_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Channel", func() {
	var (
		event   string
		message string
	)

	BeforeEach(func() {
		event = "sendMessage"
		message = "A message in a bottle"
	})

	Describe("#Publish", func() {
		It("publishes a message", func() {
			err := channel.Publish(event, message)
			Expect(err).NotTo(HaveOccurred())

			messages, err := channel.History()
			Expect(err).NotTo(HaveOccurred())
			Expect(messages[0].Name).To(Equal(event))
			Expect(messages[0].Data).To(Equal(message))
		})
	})

	Describe("#History", func() {
		BeforeEach(func() {
			err := channel.Publish(event, message)
			Expect(err).NotTo(HaveOccurred())
		})

		It("reads messages from history", func() {
			messages, err := channel.History()
			Expect(err).NotTo(HaveOccurred())
			Expect(messages[0].Name).To(Equal(event))
			Expect(messages[0].Data).To(Equal(message))
		})
	})
})
