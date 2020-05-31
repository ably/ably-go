package ablytest

import (
	"reflect"
	"testing"
	"time"
)

var Instantly = Before(10 * time.Millisecond)

var Soon = Before(Timeout)

// Before returns a WithTimeout with the given timeout duration.
func Before(d time.Duration) WithTimeout {
	return WithTimeout{before: d}
}

// WithTimeout configures test helpers with a timeout.
type WithTimeout struct {
	before time.Duration
}

// Recv asserts that a value is received through channel from before the
// timeout. If it isn't, the fail function is called.
//
// If into is non-nil, it must be a pointer to a variable of the same type as
// from's element type and it will be set to the received value, if any.
//
// It returns the second, boolean value returned by the receive operation,
// or false if the operation times out.
func (wt WithTimeout) Recv(t *testing.T, into interface{}, from interface{}, fail func(fmt string, args ...interface{})) (ok bool) {
	t.Helper()
	ok, timeout := wt.recv(into, from)
	if timeout {
		fail("timed out waiting for channel receive")
	}
	return ok
}

// NoRecv is like Recv, except it asserts no value is received.
func (wt WithTimeout) NoRecv(t *testing.T, into interface{}, from interface{}, fail func(fmt string, args ...interface{})) (ok bool) {
	t.Helper()
	if into == nil {
		into = &into
	}
	ok, timeout := wt.recv(into, from)
	if !timeout {
		fail("unexpectedly received in channel: %v", into)
	}
	return ok
}

func (wt WithTimeout) recv(into interface{}, from interface{}) (ok, timeout bool) {
	chosen, recv, ok := reflect.Select([]reflect.SelectCase{{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(from),
	}, {
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(time.After(time.Duration(wt.before))),
	}})
	if chosen == 0 && ok && into != nil {
		reflect.ValueOf(into).Elem().Set(recv)
	}
	return ok, chosen == 1
}
