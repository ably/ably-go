package proto_test

import (
	"encoding/json"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
	"github.com/ably/ably-go/proto"
)

var _ = Describe("PresenceMessage", func() {
	var presenceMessage proto.PresenceMessage

	BeforeEach(func() {
		presenceMessage = proto.PresenceMessage{
			Message: proto.Message{
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
			decodedMessage := proto.PresenceMessage{}
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
