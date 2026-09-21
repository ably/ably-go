// Package device provides the Ably Pub/Sub client for devices: applications
// running on end-user devices, whose connections are identified by a
// clientId and counted on accounts with monthly-active-user billing.
package device

import "github.com/ably/ably-go/ably"

// agentName declares the side in the Ably-Agent header (RSC7d) so that
// traffic from clients constructed by this package is classified as
// device-side.
const agentName = "ably-pubsub-device"

// Client is a device Pub/Sub client.
type Client = ably.Realtime

// NewClient constructs a device Pub/Sub client: a realtime connection to
// Ably with channels, presence and history.
func NewClient(opts ...ably.ClientOption) (*Client, error) {
	opts = append(opts[:len(opts):len(opts)], ably.WithAgents(map[string]string{agentName: ""}))
	return ably.NewRealtime(opts...)
}
