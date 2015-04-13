package ably_test

import (
	"sync"

	"github.com/ably/ably-go/ably"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("Connection", func() {
	var client *ably.RealtimeClient

	BeforeEach(func() {
		client = ably.NewRealtimeClient(testApp.Params)
	})

	AfterEach(func() {
		defer client.Close()
		select {
		case err := <-client.Err:
			Fail("Error channel is not empty: " + err.Error())
		default:
		}
	})

	It("connects to ably on initialization", func() {
		Eventually(func() (state ably.ConnState) {
			return client.Connection.State()
		}, "2s").Should(Equal(ably.ConnStateConnected))
	})

	It("accepts calls to Connect even if it is connecting now", func() {
		err := client.Connection.Connect()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("with normal connection workflow", func() {
		var (
			phases          []ably.ConnState
			eventsTriggered []ably.ConnState
			mu              sync.RWMutex
		)

		BeforeEach(func() {
			phases = []ably.ConnState{
				ably.ConnStateConnecting,
				ably.ConnStateConnected,
			}
		})

		It("goes through the normal connection state workflow", func(done Done) {
			var wg sync.WaitGroup

			wg.Add(len(phases))
			for i := range phases {
				client.Connection.On(phases[i], func(state ably.ConnState) func() {
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
