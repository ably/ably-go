// +build !windows

package ably

import (
	"bytes"
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
	return runtime.GOOS + "/" + string(release)
}
