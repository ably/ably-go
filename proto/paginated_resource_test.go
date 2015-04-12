package proto_test

import (
	"github.com/ably/ably-go/proto"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("PaginatedResource", func() {
	var paginatedResource proto.PaginatedResource

	Describe("BuildPath", func() {
		BeforeEach(func() {
			paginatedResource = proto.PaginatedResource{}
		})

		It("returns a string pointing to the new path based on the given path", func() {
			newPath, err := proto.BuildPath(&paginatedResource, "/path/to/resource?hello", "./newresource?world")
			Expect(err).NotTo(HaveOccurred())
			Expect(newPath).To(Equal("/path/to/newresource?world"))
		})
	})
})
