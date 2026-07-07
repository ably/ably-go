//go:build !unit && ably_internal_sdk_tests_only
// +build !unit,ably_internal_sdk_tests_only

package ably_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/ably/ably-go/ablytest"
)

// TestMain tears down the shared sandbox app once after all tests in this
// package have run. The app itself is provisioned lazily on first use (see
// ablytest.NewSandbox), so there is no setup here; if no test provisions it,
// CloseSharedApp is a no-op.
func TestMain(m *testing.M) {
	code := m.Run()
	if err := ablytest.CloseSharedApp(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to tear down shared sandbox app: %v\n", err)
	}
	os.Exit(code)
}
