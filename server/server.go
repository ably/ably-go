// Package pubsub provides the Ably Pub/Sub clients for servers: trusted
// environments which typically authenticate with an API key and whose
// connections are exempt from monthly-active-user counting.
//
// It is imported from github.com/ably/ably-pubsub-go/server, and referred to as
// pubsub: the package is named after the product rather than after its
// directory, so that the two entry points read the same at the call site and
// the import path alone says which side the code runs on.
//
// The rest of the API this package exposes — channels, messages, presence,
// options, errors — is re-exported from the implementation in internal/ably
// and lives in api_gen.go.
package pubsub

//go:generate go run github.com/ably/ably-pubsub-go/internal/cmd/genapi -target server

import "github.com/ably/ably-pubsub-go/internal/ably"

// agentName declares the side in the Ably-Agent header (RSC7d) so that
// traffic from clients constructed by this package is classified as
// server-side.
const agentName = "ably-pubsub-server"

// HTTPClient is a server Pub/Sub client that operates entirely over HTTP.
type HTTPClient = ably.HTTPClient

// RealtimeClient is a server Pub/Sub client with a realtime connection.
type RealtimeClient = ably.Realtime

// NewHTTPClient constructs a server Pub/Sub client that operates entirely
// over HTTP: publish, history, presence reads, stats, token issuing.
func NewHTTPClient(opts ...ClientOption) (*HTTPClient, error) {
	opts = append(opts[:len(opts):len(opts)], ably.WithAgents(map[string]string{agentName: ""}))
	return ably.NewHTTPClient(opts...)
}

// NewRealtimeClient constructs a server Pub/Sub client with a persistent
// realtime connection: everything the HTTP client does, plus subscribing
// to channels and entering presence.
func NewRealtimeClient(opts ...ClientOption) (*RealtimeClient, error) {
	opts = append(opts[:len(opts):len(opts)], ably.WithAgents(map[string]string{agentName: ""}))
	return ably.NewRealtime(opts...)
}
