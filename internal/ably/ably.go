package ably

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}

func max(i, j int) int {
	if i > j {
		return i
	}
	return j
}

func nonil(err ...error) error {
	for _, err := range err {
		if err != nil {
			return err
		}
	}
	return nil
}

func nonempty(s ...string) string {
	for _, s := range s {
		if s != "" {
			return s
		}
	}
	return ""
}

func randomString(n int) string {
	p := make([]byte, n/2+1)
	rand.Read(p)
	return hex.EncodeToString(p)[:n]
}

// unixMilli returns the given time as a timestamp in milliseconds since epoch.
func unixMilli(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}
