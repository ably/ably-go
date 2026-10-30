// Package pubsub provides the Ably Pub/Sub client for devices: applications
// running on end-user devices, whose connections are identified by a
// clientId and counted on accounts with monthly-active-user billing.
//
// It is imported from github.com/ably/ably-pubsub-go/v2/device/pubsub. Both
// entry points are named pubsub, after the product, so that they read the same
// at the call site and the device segment of the import path alone says which
// side the code runs on.
//
// The rest of the API this package exposes — channels, messages, presence,
// options, errors — is re-exported from the implementation in internal/ably
// and lives in api.go. The parts of it that only a server's HTTP client
// can reach are not re-exported here.
//
// # Reference
//
// Most types here are aliases of the implementation, so their methods and
// fields are documented with it. See the [full API reference]. The import path
// shown on that page is internal and can't be imported; import this package
// instead.
//
// [full API reference]: https://pkg.go.dev/github.com/ably/ably-pubsub-go/v2/internal/ably
package pubsub

import "github.com/ably/ably-pubsub-go/v2/internal/ably"

// agentName is added to the Ably-Agent header of every client this package
// creates, to tell Ably the client runs on an end-user device (RSC7d). Ably
// recognises device clients by the "-device" ending, so keep that ending if
// you rename it.
const agentName = "ably-pubsub-device"

// Client is a device Pub/Sub client.
//
// Fields: Auth, Channels, Connection.
//
// Methods: Connect, Close, Stats, Time.
type Client = ably.Realtime

// NewClient constructs a device Pub/Sub client: a realtime connection to
// Ably with channels, presence and history.
func NewClient(opts ...ClientOption) (*Client, error) {
	opts = append(opts[:len(opts):len(opts)], ably.WithAgents(map[string]string{agentName: ""}))
	return ably.NewRealtime(opts...)
}
