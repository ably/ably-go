package ably_test

import (
	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/testutil"

	"testing"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

func TestAbly(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ably Suite")
}

var (
	testApp *testutil.Sandbox
	client  *ably.RestClient
	channel *ably.RestChannel
)

var _ = BeforeSuite(func() {
	app, err := testutil.NewSandbox(nil)
	Expect(err).NotTo(HaveOccurred())
	testApp = app
})

var _ = BeforeEach(func() {
	cl, err := ably.NewRestClient(testApp.Options())
	Expect(err).NotTo(HaveOccurred())
	client = cl
	channel = client.Channel("test")
})

var _ = AfterSuite(func() {
	err := testApp.Close()
	Expect(err).NotTo(HaveOccurred())
})
