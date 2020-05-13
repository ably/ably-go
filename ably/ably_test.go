package ably_test

import (
	"fmt"
	"io"
	"testing"

	"github.com/ably/ably-go/ably"
)

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
	if a, ok := e.(*ably.Error); ok {
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
		t.FailNow()
	}
}

func checkError(code int, err error) error {
	switch e, ok := err.(*ably.Error); {
	case !ok:
		return fmt.Errorf("want err to be *ably.Error; was %T: %v", err, err)
	case e.Code != code:
		return fmt.Errorf("want e.Code=%d; got %d: %s", code, e.Code, err)
	default:
		return nil
	}
}
