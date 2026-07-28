// Package server provides the Ably Pub/Sub clients for servers: trusted
// environments which typically authenticate with an API key and whose
// connections are exempt from monthly-active-user counting.
package server

import "github.com/ably/ably-go/ably"

// agentName declares the side in the Ably-Agent header (RSC7d) so that
// traffic from clients constructed by this package is classified as
// server-side.
const agentName = "ably-go-pubsub-server"

// HTTPClient is a server Pub/Sub client that operates entirely over HTTP.
type HTTPClient = ably.REST

// RealtimeClient is a server Pub/Sub client with a realtime connection.
type RealtimeClient = ably.Realtime

// NewHTTPClient constructs a server Pub/Sub client that operates entirely
// over HTTP: publish, history, presence reads, stats, token issuing.
func NewHTTPClient(opts ...ably.ClientOption) (*HTTPClient, error) {
	opts = append(opts[:len(opts):len(opts)], ably.WithAgents(map[string]string{agentName: ""}))
	return ably.NewREST(opts...)
}

// NewRealtimeClient constructs a server Pub/Sub client with a persistent
// realtime connection: everything the HTTP client does, plus subscribing
// to channels and entering presence.
func NewRealtimeClient(opts ...ably.ClientOption) (*RealtimeClient, error) {
	opts = append(opts[:len(opts):len(opts)], ably.WithAgents(map[string]string{agentName: ""}))
	return ably.NewRealtime(opts...)
}
