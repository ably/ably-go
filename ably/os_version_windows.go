package ably

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/windows"
)

func goOSIdentifier() string {
	v := windows.RtlGetVersion()
	return fmt.Sprintf("%s/%d.%d.%d", runtime.GOOS, v.MajorVersion, v.MinorVersion, v.BuildNumber)
}
