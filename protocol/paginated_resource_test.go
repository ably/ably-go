package protocol_test

import (
	"github.com/ably/ably-go/protocol"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("PaginatedResource", func() {
	var paginatedResource protocol.PaginatedResource

	Describe("BuildPath", func() {
		BeforeEach(func() {
			paginatedResource = protocol.PaginatedResource{}
		})

		It("returns a string pointing to the new path based on the given path", func() {
			newPath, err := paginatedResource.BuildPath("/path/to/resource?hello", "./newresource?world")
			Expect(err).NotTo(HaveOccurred())
			Expect(newPath).To(Equal("/path/to/newresource?world"))
		})
	})
})
