package realtime_test

import (
	"sync"

	"github.com/ably/ably-go/realtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Connection", func() {
	It("connects to ably on initialization", func() {
		Eventually(func() realtime.ConnState {
			return client.Connection.State
		}, "2s").Should(Equal(realtime.ConnStateConnected))
	})

	It("accepts calls to Connect", func() {
		err := client.Connection.Connect()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("with normal connection workflow", func() {
		var (
			phases          []realtime.ConnState
			eventsTriggered []realtime.ConnState
			mu              sync.RWMutex
		)

		BeforeEach(func() {
			phases = []realtime.ConnState{
				realtime.ConnStateConnecting,
				realtime.ConnStateConnected,
			}
		})

		XIt("goes through the normal connection state workflow", func(done Done) {
			var wg sync.WaitGroup

			for i := range phases {
				wg.Add(1)
				client.Connection.On(phases[i], func() {
					mu.Lock()
					defer wg.Done()
					defer mu.Unlock()
					eventsTriggered = append(eventsTriggered, phases[i])
				})
			}

			wg.Wait()

			Expect(eventsTriggered).To(Equal(phases))
		}, 2.0)
	})
})
