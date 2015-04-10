package realtime_test

import (
	"sync"

	"github.com/ably/ably-go/realtime"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("Connection", func() {
	AfterEach(func() {
		select {
		case err := <-client.Err:
			Fail("Error channel is not empty: " + err.Error())
		default:
		}
	})

	It("connects to ably on initialization", func() {
		Eventually(func() (state realtime.ConnState) {
			return client.Connection.State()
		}, "2s").Should(Equal(realtime.ConnStateConnected))
	})

	It("accepts calls to Connect even if it is connecting now", func() {
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

		It("goes through the normal connection state workflow", func(done Done) {
			var wg sync.WaitGroup

			wg.Add(len(phases))
			for i := range phases {
				client.Connection.On(phases[i], func(state realtime.ConnState) func() {
					return func() {
						mu.Lock()
						defer mu.Unlock()
						defer wg.Done()
						eventsTriggered = append(eventsTriggered, state)
					}
				}(phases[i]))
			}

			wg.Wait()

			Expect(eventsTriggered).To(Equal(phases))
			done <- true
		}, 2.0)
	})
})
