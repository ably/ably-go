// Package pubsub provides the Ably Pub/Sub clients for servers: trusted
// environments whose connections are exempt from monthly-active-user counting.
//
// Clients constructed by this package declare themselves as servers, so
// authenticate them with an API key. Token authentication is accepted only when
// the token carries the signed x-ably-clientType=server claim, which today means
// a JWT; otherwise Ably rejects the connection or request with error 40167
// ("Client type declaration not signed by token").
//
// It is imported from github.com/ably/ably-pubsub-go/server/pubsub. Both entry
// points are named pubsub, after the product, so that they read the same at
// the call site and the server segment of the import path alone says which side
// the code runs on.
//
// The rest of the API this package exposes — channels, messages, presence,
// options, errors — is re-exported from the implementation in internal/ably
// and lives in api.go.
package pubsub

import "github.com/ably/ably-pubsub-go/internal/ably"

// agentName declares the side in the Ably-Agent header (RSC7d) so that
// traffic from clients constructed by this package is classified as
// server-side.
//
// The -server suffix is load-bearing, not cosmetic: on API-key auth Ably grants
// the server exemption from monthly-active-user counting by matching an agent
// entry ending in -server. Renaming it without keeping the suffix silently
// reclassifies every client this package constructs.
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
