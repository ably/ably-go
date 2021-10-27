package ably

import (
	"fmt"
	"runtime"
)

// constants for rsc7
const (
	ablyVersionHeader      = "X-Ably-Version"
	ablyErrorCodeHeader    = "X-Ably-Errorcode"
	ablyErrorMessageHeader = "X-Ably-Errormessage"
	libraryVersion         = "1.2.3"
	libraryName            = "go"
	ablyVersion            = "1.2"
	ablyClientIDHeader     = "X-Ably-ClientId"
	hostHeader             = "Host"
	ablyAgentHeader        = "Ably-Agent"                // RSC7d
	ablySDKIdentifier      = "ably-go/" + libraryVersion // RSC7d1
)

var goRuntimeIdentifier = func() string {
	return fmt.Sprintf("%s/%s", libraryName, runtime.Version()[2:])
}()

var ablyAgentIdentifier = func() string {
	osIdentifier := goOSIdentifier()
	if empty(osIdentifier) {
		return fmt.Sprintf("%s %s", ablySDKIdentifier, goRuntimeIdentifier)
	} else {
		return fmt.Sprintf("%s %s %s", ablySDKIdentifier, goRuntimeIdentifier, osIdentifier)
	}
}()
