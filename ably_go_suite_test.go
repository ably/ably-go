package ably_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAblyGo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AblyGo Suite")
}
