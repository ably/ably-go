// Package pubsub provides the Ably Pub/Sub clients for servers: trusted
// environments whose connections are exempt from monthly-active-user counting.
//
// Clients from this package declare themselves as servers, so authenticate them
// with an API key. Token authentication works only with a token carrying the
// signed x-ably-clientType=server claim, which today means a JWT. With any other
// token, Ably rejects the connection or request with error 40167 ("Client type
// declaration not signed by token").
//
// It is imported from github.com/ably/ably-pubsub-go/v2/server/pubsub. Both entry
// points are named pubsub, after the product, so that they read the same at
// the call site and the server segment of the import path alone says which side
// the code runs on.
//
// The rest of the API this package exposes — channels, messages, presence,
// options, errors — is re-exported from the implementation in internal/ably
// and lives in api.go.
package pubsub

import "github.com/ably/ably-pubsub-go/v2/internal/ably"

// agentName is added to the Ably-Agent header of every client this package
// creates, to tell Ably the client runs on a server (RSC7d). Ably recognises
// server clients by the "-server" ending and doesn't count their connections
// as monthly active users, so keep that ending if you rename it.
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
