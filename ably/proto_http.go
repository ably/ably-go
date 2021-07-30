package ably

import (
	"runtime"
	"sync"
)

// constants for rsc7
const (
	ablyVersionHeader      = "X-Ably-Version"
	ablyErrorCodeHeader    = "X-Ably-Errorcode"
	ablyErrorMessageHeader = "X-Ably-Errormessage"
	libraryVersion         = "1.2.0"
	libraryName            = "go"
	libraryString          = libraryName + "-" + libraryVersion
	ablyVersion            = "1.2"
	ablyClientIDHeader     = "X-Ably-ClientId"
	hostHeader             = "Host"
	ablyAgentHeader        = "Ably-Agent"                // RSC7d
	ablySDKIdentifier      = "ably-go/" + libraryVersion // RSC7d1
)

var ablyAgentIdentifier = "" // Need to be calculated at runtime
var agentIdentifierMutex sync.Mutex

func goRuntimeIdentifier() string {
	return libraryName + "/" + runtime.Version()[2:]
}

func getAblyAgentIdentifier() string {
	agentIdentifierMutex.Lock()
	defer agentIdentifierMutex.Unlock()
	if empty(ablyAgentIdentifier) {
		osIdentifier := goOSIdentifier()
		if empty(osIdentifier) {
			ablyAgentIdentifier = ablySDKIdentifier + " " + goRuntimeIdentifier()
		} else {
			ablyAgentIdentifier = ablySDKIdentifier + " " + goRuntimeIdentifier() + " " + osIdentifier
		}
	}
	return ablyAgentIdentifier
}
