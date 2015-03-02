package realtime_test

import (
	"github.com/ably/ably-go/realtime"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	It("connects to ably", func() {
		Eventually(func() realtime.ConnState {
			return client.Connection.State
		}, "2s").Should(Equal(realtime.ConnStateConnected))
	})
})
