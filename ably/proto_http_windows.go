package ably

import (
	"runtime"
	"strconv"

	"golang.org/x/sys/windows"
)

func goOSIdentifier() string {
	versionInfo := windows.RtlGetVersion()
	semanticVersion := strconv.Itoa(int(versionInfo.MajorVersion)) + "." +
		strconv.Itoa(int(versionInfo.MinorVersion)) + "." +
		strconv.Itoa(int(versionInfo.BuildNumber))
	return runtime.GOOS + "/" + semanticVersion
}
