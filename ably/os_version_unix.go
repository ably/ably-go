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
	runtimeOS := runtime.GOOS
	if runtimeOS == "darwin" {
		runtimeOS = "macOS-iOS"
	}
	return fmt.Sprintf("%s/%s", runtimeOS, release)
}
