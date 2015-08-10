package ably_test

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
)

var timeout = 2 * time.Second

func init() {
	ably.Log.Level = ably.LogVerbose
	if s := os.Getenv("ABLY_TIMEOUT"); s != "" {
		if t, err := time.ParseDuration(s); err == nil {
			timeout = t
		}
	}
}

func nonil(err ...error) error {
	for _, err := range err {
		if err != nil {
			return err
		}
	}
	return nil
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
		return fmt.Errorf("want err to be *ably.Error; was %T", err)
	case e.Code != code:
		return fmt.Errorf("want e.Code=%d; got %d", code, e.Code)
	default:
		return nil
	}
}
