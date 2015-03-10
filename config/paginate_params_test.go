package config_test

import (
	"net/url"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
	"github.com/ably/ably-go/config"
)

var _ = Describe("PaginationParams", func() {
	var (
		params config.PaginateParams
		values *url.Values
	)

	BeforeEach(func() {
		values = &url.Values{}
		params = config.PaginateParams{}
	})

	Describe("Values", func() {
		It("returns nil with no values", func() {
			params = config.PaginateParams{}
			err := params.EncodeValues(values)
			Expect(err).NotTo(HaveOccurred())
			Expect(values.Encode()).To(Equal(""))
		})

		It("returns the full params encoded", func() {
			params = config.PaginateParams{
				Limit:     1,
				Direction: "backwards",
				ScopeParams: config.ScopeParams{
					Start: 123,
					End:   124,
					Unit:  "hello",
				},
			}
			err := params.EncodeValues(values)
			Expect(err).NotTo(HaveOccurred())
			Expect(values.Encode()).To(Equal("direction=backwards&end=124&limit=1&start=123&unit=hello"))
		})

		Context("with values", func() {
			BeforeEach(func() {
				params = config.PaginateParams{
					Limit:     10,
					Direction: "backwards",
				}
			})

			It("does not return an error", func() {
				err := params.EncodeValues(values)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("with a value for ScopeParams", func() {
			BeforeEach(func() {
				params.Start = 123
			})

			It("creates a valid url", func() {
				err := params.EncodeValues(values)
				Expect(err).NotTo(HaveOccurred())
				Expect(values.Get("start")).To(Equal("123"))
			})
		})

		Context("with invalid value for direction", func() {
			BeforeEach(func() {
				params.Direction = "unknown"
			})

			It("resets the value to the default", func() {
				err := params.EncodeValues(values)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with invalid value for limit", func() {
			BeforeEach(func() {
				params.Limit = -1
			})

			It("resets the value to the default", func() {
				err := params.EncodeValues(values)
				Expect(err).NotTo(HaveOccurred())
				Expect(values.Get("limit")).To(Equal("100"))
			})
		})
	})
})
