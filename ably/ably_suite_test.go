package ably_test

import (
	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"

	"testing"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

func TestAbly(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ably Suite")
}

var (
	testApp *ablytest.Sandbox
	client  *ably.RestClient
)

var _ = BeforeSuite(func() {
	app, err := ablytest.NewSandbox(nil)
	Expect(err).NotTo(HaveOccurred())
	testApp = app
})

var _ = BeforeEach(func() {
	cl, err := ably.NewRestClient(testApp.Options())
	Expect(err).NotTo(HaveOccurred())
	client = cl
})

var _ = AfterSuite(func() {
	err := testApp.Close()
	Expect(err).NotTo(HaveOccurred())
})
