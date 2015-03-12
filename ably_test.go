package ably_test

import (
	"github.com/ably/ably-go"
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/realtime"
	"github.com/ably/ably-go/rest"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("ably package", func() {
	It("can create a Rest client from the main package", func() {
		Expect(ably.NewRestClient(config.Params{})).To(BeAssignableToTypeOf(&rest.Client{}))
	})

	It("can create a Realtime client from the main package", func() {
		Expect(ably.NewRealtimeClient(config.Params{})).To(BeAssignableToTypeOf(&realtime.Client{}))
	})
})
