package ablytest_test

import (
	"fmt"

	"github.com/ably/ably-go/ably/ablytest"
)

func ExampleFmtFunc_Wrap() {
	println := ablytest.FmtFunc(func(format string, args ...interface{}) {
		fmt.Println(fmt.Sprintf(format, args...))
	})

	id := 42
	wrapped := println.Wrap("for ID %d: %s", id)

	wrapped("everything's OK")

	errMsg := "all hell broke loose"
	wrapped("something failed: %s", errMsg)

	// Output:
	// for ID 42: everything's OK
	// for ID 42: something failed: all hell broke loose
}
