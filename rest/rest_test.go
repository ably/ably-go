package rest_test

import (
	"github.com/ably/ably-go/rest"
	. "github.com/ably/ably-go/test/support"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	_ "crypto/sha512"
)

var (
	client  *rest.Client
	channel *rest.Channel
)

var _ = BeforeSuite(func() {
	_, err := TestAppInstance.Create()
	Expect(err).NotTo(HaveOccurred())

	client = rest.NewClient(TestAppInstance.Params)
	channel = client.Channel("test")
})

var _ = AfterSuite(func() {
	_, err := TestAppInstance.Delete()
	Expect(err).NotTo(HaveOccurred())
})

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
