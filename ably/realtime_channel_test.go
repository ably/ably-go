package ably_test

import (
	"github.com/ably/ably-go/ably"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("RealtimeChannel", func() {
	var (
		client  *ably.RealtimeClient
		channel *ably.RealtimeChannel
	)

	BeforeEach(func() {
		client = ably.NewRealtimeClient(testApp.Params)
		channel = client.RealtimeChannel("test")
	})

	AfterEach(func() {
		client.Close()
	})

	Context("When the connection is ready", func() {
		XIt("publish messages", func() {
			err := channel.Publish("hello", "world")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
