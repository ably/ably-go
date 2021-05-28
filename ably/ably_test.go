package ably_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"
)

type Result interface {
	Wait(context.Context) error
}

func nonil(err ...error) error {
	for _, err := range err {
		if err != nil {
			return err
		}
	}
	return nil
}

type closeClient struct {
	io.Closer
	skip []int
}

func (c *closeClient) Close() error {
	e := c.Closer.Close()
	if a, ok := e.(*ably.ErrorInfo); ok {
		for _, v := range c.skip {
			if a.StatusCode == v {
				return nil
			}
		}
	}
	return e
}

func safeclose(t *testing.T, closers ...io.Closer) {
	type failed struct {
		i   int
		c   io.Closer
		err error
	}
	var errors []failed
	for i, closer := range closers {
		err := closer.Close()
		if err != nil {
			errors = append(errors, failed{i, closer, err})
		}
	}
	if len(errors) != 0 {
		for _, err := range errors {
			t.Logf("safeclose %d: failed to close %T: %s", err.i, err.c, err.err)
		}
	}
}

type closeFunc func() error

func (f closeFunc) Close() error {
	return f()
}

func checkError(code ably.ErrorCode, err error) error {
	switch e, ok := err.(*ably.ErrorInfo); {
	case !ok:
		return fmt.Errorf("want err to be *ably.ErrorInfo; was %T: %v", err, err)
	case e.Code != code:
		return fmt.Errorf("want e.Code=%d; got %d: %s", code, e.Code, err)
	default:
		return nil
	}
}

func assertEquals(t *testing.T, expected interface{}, actual interface{}) {
	if expected != actual {
		t.Errorf("%v is not equal to %v", expected, actual)
	}
}

func assertTrue(t *testing.T, value bool) {
	if !value {
		t.Errorf("%v is not true", value)
	}
}

func assertFalse(t *testing.T, value bool) {
	if value {
		t.Errorf("%v is not false", value)
	}
}

func assertNil(t *testing.T, object interface{}) {
	if object != nil {
		value := reflect.ValueOf(object)
		if !value.IsNil() {
			t.Errorf("%v is not nil", object)
		}
	}
}

func assertDeepEquals(t *testing.T, expected interface{}, actual interface{}) {
	areEqual := reflect.DeepEqual(expected, actual)
	if !areEqual {
		t.Errorf("%v is not equal to %v", expected, actual)
	}
}

func init() {
	ablytest.ClientOptionsInspector.UseBinaryProtocol = func(o []ably.ClientOption) bool {
		return !ably.ApplyOptionsWithDefaults(o...).NoBinaryProtocol
	}
	ablytest.ClientOptionsInspector.HTTPClient = func(o []ably.ClientOption) *http.Client {
		return ably.ApplyOptionsWithDefaults(o...).HTTPClient
	}
}

type messages chan *ably.Message

func (ms messages) Receive(m *ably.Message) {
	ms <- m
}

type connMock struct {
	SendFunc    func(*proto.ProtocolMessage) error
	ReceiveFunc func(deadline time.Time) (*proto.ProtocolMessage, error)
	CloseFunc   func() error
}

func (r connMock) Send(a0 *proto.ProtocolMessage) error {
	return r.SendFunc(a0)
}

func (r connMock) Receive(deadline time.Time) (*proto.ProtocolMessage, error) {
	return r.ReceiveFunc(deadline)
}

func (r connMock) Close() error {
	return r.CloseFunc()
}
