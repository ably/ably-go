package config_test

import (
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
	"github.com/ably/ably-go/config"
)

var _ = Describe("PaginationParams", func() {
	var params config.PaginateParams

	It("returns nil with no values", func() {
		params = config.PaginateParams{}
		values, err := params.Values()
		Expect(err).NotTo(HaveOccurred())
		Expect(values).NotTo(BeNil())
	})

	Describe("Values", func() {
		BeforeEach(func() {
			params = config.PaginateParams{
				Limit:     10,
				Direction: "backwards",
			}
		})

		It("does not return an error", func() {
			_, err := params.Values()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("with a value for ScopeParams", func() {
			BeforeEach(func() {
				params.Start = 123
			})

			It("creates a valid url", func() {
				values, err := params.Values()
				Expect(err).NotTo(HaveOccurred())
				Expect(values.Get("start")).To(Equal("123"))
			})
		})

		Context("with invalid value for direction", func() {
			BeforeEach(func() {
				params.Direction = "unknown"
			})

			It("resets the value to the default", func() {
				_, err := params.Values()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with invalid value for limit", func() {
			BeforeEach(func() {
				params.Limit = -1
			})

			It("resets the value to the default", func() {
				values, err := params.Values()
				Expect(err).NotTo(HaveOccurred())
				Expect(values.Get("limit")).To(Equal("100"))
			})
		})
	})
})
