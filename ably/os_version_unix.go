//go:build linux || darwin
// +build linux darwin

package ably

import (
	"bytes"
	"fmt"
	"runtime"

	"golang.org/x/sys/unix"
)

func goOSIdentifier() string {
	utsname := unix.Utsname{}
	err := unix.Uname(&utsname)
	if err != nil {
		return ""
	}
	release := bytes.Trim(utsname.Release[:], "\x00")
	return fmt.Sprintf("%s/%s", runtime.GOOS, release)
}
