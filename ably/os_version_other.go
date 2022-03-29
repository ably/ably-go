//go:build !linux && !darwin && !windows
// +build !linux,!darwin,!windows

package ably

func goOSIdentifier() string {
	return ""
}
