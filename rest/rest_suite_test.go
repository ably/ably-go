package rest_test

import (
	"github.com/ably/ably-go/rest"
	"github.com/ably/ably-go/test/support"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
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
	client = rest.NewClient(testApp.Params)
	channel = client.Channel("test")
})

var _ = AfterSuite(func() {
	_, err := testApp.Delete()
	Expect(err).NotTo(HaveOccurred())
})
