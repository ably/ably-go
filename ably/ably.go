package ably

//go:generate go run ../scripts/errors.go -json ../common/protocol/errors.json -o errors.go

import (
	"crypto/rand"
	"encoding/hex"
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
