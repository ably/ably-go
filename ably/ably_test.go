package ably_test

import (
	"io"
	"os"
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

func multiclose(closers ...io.Closer) {
	var err error
	for _, closer := range closers {
		err = nonil(err, closer.Close())
	}
	if err != nil {
		panic(err)
	}
}
