package ably_test

import (
	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/testutil"

	"testing"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

func TestAbly(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ably Suite")
}

var (
	testApp *testutil.App
	client  *ably.RestClient
	channel *ably.RestChannel
)

var _ = BeforeSuite(func() {
	testApp = testutil.NewApp()
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
