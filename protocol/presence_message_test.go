package protocol_test

import (
	"encoding/json"

	"github.com/ably/ably-go/protocol"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("PresenceMessage", func() {
	var presenceMessage protocol.PresenceMessage

	BeforeEach(func() {
		presenceMessage = protocol.PresenceMessage{
			Message: protocol.Message{
				Data: "hello",
			},
		}
	})

	It("can access properties of Message directly", func() {
		Expect(presenceMessage.Data).To(Equal("hello"))
	})

	It("supports encoding", func() {
		err := presenceMessage.EncodeData("json/utf-8", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(presenceMessage.Data).To(Equal("hello"))
	})

	Context("json encoding", func() {
		var presenceMessageJSON string

		BeforeEach(func() {
			presenceMessageJSON = `{"data":"hello","action":0,"clientId":"","connectionId":"","timestamp":0}`
		})

		It("is decodable", func() {
			decodedMessage := protocol.PresenceMessage{}
			err := json.Unmarshal([]byte(presenceMessageJSON), &decodedMessage)
			Expect(err).NotTo(HaveOccurred())
			Expect(decodedMessage).To(Equal(presenceMessage))
		})

		It("is encodable", func() {
			encodedMessage, err := json.Marshal(presenceMessage)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(encodedMessage)).To(Equal(presenceMessageJSON))
		})
	})
})
