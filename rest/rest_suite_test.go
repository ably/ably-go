package rest_test

import (
	"github.com/ably/ably-go"
	"github.com/ably/ably-go/rest"
	"github.com/ably/ably-go/test/support"

	"testing"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

func TestRest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rest Suite")
}

var (
	testApp *support.TestApp
	client  *rest.Client
	channel *rest.Channel
)

var _ = BeforeSuite(func() {
	testApp = support.NewTestApp()
	_, err := testApp.Create()
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {
	client = ably.NewRestClient(testApp.Params)
	channel = client.Channel("test")
})

var _ = AfterSuite(func() {
	_, err := testApp.Delete()
	Expect(err).NotTo(HaveOccurred())
})
