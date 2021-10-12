package ablytest

import (
	"fmt"
	"testing"
)

// A FmtFunc is a non-failing function analogous to fmt.Printf.
type FmtFunc func(format string, args ...interface{})

// Wrap wraps a FmtFunc in another. The wrapper FmtFunc first uses the fixed
// format and args, plus the format it's called with, to create a new format
// string by calling fmt.Sprintf. Then, it calls the wrapped FmtFunc with this
// format string and the args the wrapper is called with.
//
// It's useful to have some fixed context on everything that is formatted with
// a FmtFunc, e.g. test scope context.
func (f FmtFunc) Wrap(t *testing.T, format string, args ...interface{}) FmtFunc {
	return func(wrappedFormat string, wrappedArgs ...interface{}) {
		if t != nil {
			t.Helper()
		}
		f(fmt.Sprintf(format, append(args, wrappedFormat)...), wrappedArgs...)
	}
}
