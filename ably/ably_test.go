package ably_test

import (
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
