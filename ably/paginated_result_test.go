package ably_test

import (
	"github.com/ably/ably-go/ably"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("PaginatedResult", func() {
	var result *ably.PaginatedResult

	Describe("BuildPath", func() {
		BeforeEach(func() {
			result = &ably.PaginatedResult{}
		})

		It("returns a string pointing to the new path based on the given path", func() {
			newPath, err := result.BuildPath("/path/to/resource?hello", "./newresource?world")
			Expect(err).NotTo(HaveOccurred())
			Expect(newPath).To(Equal("/path/to/newresource?world"))
		})
	})
})
