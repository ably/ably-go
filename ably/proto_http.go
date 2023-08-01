package ably

import (
	"fmt"
	"runtime"
	"strings"
)

// constants for rsc7
const (
	ablyVersionHeader      = "X-Ably-Version"
	ablyErrorCodeHeader    = "X-Ably-Errorcode"
	ablyErrorMessageHeader = "X-Ably-Errormessage"
	libraryVersion         = "1.2.13"
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

func ablyAgentIdentifier(agents map[string]string) string {
	identifiers := []string{
		ablySDKIdentifier,
		goRuntimeIdentifier,
	}

	osIdentifier := goOSIdentifier()
	if !empty(osIdentifier) {
		identifiers = append(identifiers, osIdentifier)
	}

	for product, version := range agents {
		if empty(version) {
			identifiers = append(identifiers, product)
		} else {
			identifiers = append(identifiers, product+"/"+version)
		}
	}

	return strings.Join(identifiers, " ")
}
