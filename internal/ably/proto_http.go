package ably

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
)

// constants for rsc7
const (
	ablyProtocolVersionHeader = "X-Ably-Version"
	ablyErrorCodeHeader       = "X-Ably-Errorcode"
	ablyErrorMessageHeader    = "X-Ably-Errormessage"
	clientLibraryVersion      = "2.0.0"
	clientRuntimeName         = "go"
	// ablyProtocolVersion is the default Ably protocol version used for all requests.
	// Protocol v5 is required for message operations (publish/update/delete/append) to return
	// message serials and version information.
	//
	// Note: Stats requests explicitly override this to use protocol v2 to maintain compatibility
	// with the existing nested Stats structure. Migrating stats to the flattened v3+ format
	// requires breaking API changes and is planned for ably-go v2.0.
	ablyProtocolVersion = "5" // CSV2
	ablyClientIDHeader  = "X-Ably-ClientId"
	hostHeader          = "Host"
	ablyAgentHeader     = "Ably-Agent"                             // RSC7d
	ablySDKIdentifier   = "ably-pubsub-go/" + clientLibraryVersion // RSC7d1
)

// AblyAgentHeaderName is the name of the Ably-Agent header (RSC7d), exported for
// use by test helpers that issue their own requests to Ably.
const AblyAgentHeaderName = ablyAgentHeader

var goRuntimeIdentifier = func() string {
	return fmt.Sprintf("%s/%s", clientRuntimeName, runtime.Version()[2:])
}()

// AgentIdentifier returns the value the library sends in the Ably-Agent header
// (RSC7d) for the given additional agents. It is exported so that test helpers
// can identify their own requests to Ably with the same agent string the client
// uses, without duplicating the library version.
func AgentIdentifier(agents map[string]string) string {
	return ablyAgentIdentifier(agents)
}

func ablyAgentIdentifier(agents map[string]string) string {
	identifiers := []string{
		ablySDKIdentifier,
		goRuntimeIdentifier,
	}

	osIdentifier := goOSIdentifier()
	if !empty(osIdentifier) {
		identifiers = append(identifiers, osIdentifier)
	}

	// Sort the products so the header is the same on every request; map
	// iteration order is random.
	products := make([]string, 0, len(agents))
	for product := range agents {
		products = append(products, product)
	}
	sort.Strings(products)
	for _, product := range products {
		version := agents[product]
		if empty(version) {
			identifiers = append(identifiers, product)
		} else {
			identifiers = append(identifiers, product+"/"+version)
		}
	}

	return strings.Join(identifiers, " ")
}
