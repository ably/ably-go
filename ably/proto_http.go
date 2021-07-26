package ably

import "runtime"

// constants for rsc7
const (
	ablyVersionHeader      = "X-Ably-Version"
	ablyLibHeader          = "X-Ably-Lib"
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

func goRuntimeIdentifier() string {
	return libraryName + "/" + runtime.Version()[2:]
}
func ablyAgentIdentifier() string {
	return ablySDKIdentifier + " " + goRuntimeIdentifier()
}
