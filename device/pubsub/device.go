// Package pubsub provides the Ably Pub/Sub client for devices: applications
// running on end-user devices, whose connections are identified by a
// clientId and counted on accounts with monthly-active-user billing.
//
// It is imported from github.com/ably/ably-pubsub-go/device/pubsub. Both entry
// points are named pubsub, after the product, so that they read the same at
// the call site and the device segment of the import path alone says which side
// the code runs on.
//
// The rest of the API this package exposes — channels, messages, presence,
// options, errors — is re-exported from the implementation in internal/ably
// and lives in api_gen.go. The parts of it that only a server's HTTP client
// can reach are not re-exported here.
package pubsub

//go:generate go run github.com/ably/ably-pubsub-go/internal/cmd/genapi -target device

import "github.com/ably/ably-pubsub-go/internal/ably"

// agentName declares the side in the Ably-Agent header (RSC7d) so that
// traffic from clients constructed by this package is classified as
// device-side.
const agentName = "ably-pubsub-device"

// Client is a device Pub/Sub client.
type Client = ably.Realtime

// NewClient constructs a device Pub/Sub client: a realtime connection to
// Ably with channels, presence and history.
func NewClient(opts ...ClientOption) (*Client, error) {
	opts = append(opts[:len(opts):len(opts)], ably.WithAgents(map[string]string{agentName: ""}))
	return ably.NewRealtime(opts...)
}
