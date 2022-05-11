//go:build !integration
// +build !integration

package ablytest_test

import (
	"fmt"

	"github.com/ably/ably-go/ablytest"
)

func ExampleFmtFunc_Wrap() {
	myPrintln := ablytest.FmtFunc(func(format string, args ...interface{}) {
		fmt.Println(fmt.Sprintf(format, args...))
	})

	id := 42
	wrapped := myPrintln.Wrap(nil, "for ID %d: %s", id)

	wrapped("everything's OK")

	errMsg := "all hell broke loose"
	wrapped("something failed: %s", errMsg)

	// Output:
	// for ID 42: everything's OK
	// for ID 42: something failed: all hell broke loose
}
