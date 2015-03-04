package realtime_test

import (
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("Channel", func() {
	Context("When the connection is ready", func() {
		XIt("publish messages", func() {
			err := channel.Publish("hello", "world")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
