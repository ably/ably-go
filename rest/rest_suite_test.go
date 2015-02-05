package rest_test

import (
	_ "crypto/sha512"

	"github.com/ably/ably-go/rest"
	. "github.com/ably/ably-go/test/support"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rest Suite")
}

var _ = BeforeSuite(func() {
	_, err := TestAppInstance.Create()
	Expect(err).NotTo(HaveOccurred())

	client = rest.NewClient(TestAppInstance.Params)
	channel = client.Channel("test")
})

var _ = AfterSuite(func() {
	_, err := TestAppInstance.Delete()
	Expect(err).NotTo(HaveOccurred())
})
