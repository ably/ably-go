package config_test

import (
	"net/url"

	"github.com/ably/ably-go/config"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("ScopeParams", func() {
	var (
		params config.ScopeParams
		values *url.Values
	)

	Describe("Values", func() {
		BeforeEach(func() {
			params = config.ScopeParams{}
		})

		Context("with an invalid range", func() {
			BeforeEach(func() {
				params.Start = 123
				params.End = 122
			})

			It("returns an error", func() {
				err := params.EncodeValues(values)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
