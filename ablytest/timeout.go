package ablytest

import (
	"fmt"
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
func (wt WithTimeout) Recv(t *testing.T, into, from interface{}, fail func(fmt string, args ...interface{}), failExtraArgs ...interface{}) (ok bool) {
	t.Helper()
	ok, timeout := wt.recv(into, from)
	if timeout {
		fail("timed out waiting for channel receive" + fmtExtraArgs(failExtraArgs))
	}
	return ok
}

// NoRecv is like Recv, except it asserts no value is received.
func (wt WithTimeout) NoRecv(t *testing.T, into, from interface{}, fail func(fmt string, args ...interface{}), failExtraArgs ...interface{}) (ok bool) {
	t.Helper()
	if into == nil {
		into = &into
	}
	ok, timeout := wt.recv(into, from)
	if !timeout {
		fail("unexpectedly received in channel: %v"+fmtExtraArgs(failExtraArgs), into)
	}
	return ok
}

// Send is like Recv, except it sends.
func (wt WithTimeout) Send(t *testing.T, ch, v interface{}, fail func(fmt string, args ...interface{}), failExtraArgs ...interface{}) (ok bool) {
	t.Helper()
	if timeout := wt.send(ch, v); timeout {
		fail("timed out waiting for channel send" + fmtExtraArgs(failExtraArgs))
	}
	return ok
}

func fmtExtraArgs(args []interface{}) string {
	if len(args) == 0 {
		return ""
	}
	return fmt.Sprintf(" [%s]", fmt.Sprint(args...))
}

func (wt WithTimeout) recv(into, from interface{}) (ok, timeout bool) {
	chosen, recv, ok := reflect.Select([]reflect.SelectCase{{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(from),
	}, {
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(time.After(wt.before)),
	}})
	if chosen == 0 && ok && into != nil {
		reflect.ValueOf(into).Elem().Set(recv)
	}
	return ok, chosen == 1
}

func (wt WithTimeout) send(ch, v interface{}) (timeout bool) {
	chosen, _, _ := reflect.Select([]reflect.SelectCase{{
		Dir:  reflect.SelectSend,
		Chan: reflect.ValueOf(ch),
		Send: reflect.ValueOf(v),
	}, {
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(time.After(wt.before)),
	}})
	return chosen == 1
}

func (wt WithTimeout) IsTrue(pred func() bool) bool {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(wt.before)
	defer timeout.Stop()
	for {
		if pred() {
			return true
		}
		select {
		case <-ticker.C:
		case <-timeout.C:
			return pred()
		}
	}
}
