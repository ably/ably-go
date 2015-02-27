package ably_test

import (
	"github.com/ably/ably-go"
	"github.com/ably/ably-go/realtime"
	"github.com/ably/ably-go/rest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ably package", func() {
	It("can create a Rest client from the main package", func() {
		Expect(ably.NewRestClient(&ably.ClientOptions{})).To(BeAssignableToTypeOf(&rest.Client{}))
	})

	It("can create a Realtime client from the main package", func() {
		Expect(ably.NewRealtimeClient(&ably.ClientOptions{})).To(BeAssignableToTypeOf(&realtime.Client{}))
	})
})
