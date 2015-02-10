package rest_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
			messages, err := channel.History()
			Expect(err).NotTo(HaveOccurred())
			Expect(messages[0].Name).To(Equal(event))
			Expect(messages[0].Data).To(Equal(message))
		})
	})
})
