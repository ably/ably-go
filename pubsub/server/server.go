// Package server provides the Ably Pub/Sub clients for servers: trusted
// environments which typically authenticate with an API key and whose
// connections are exempt from monthly-active-user counting.
package server

import "github.com/ably/ably-go/ably"

// agentName declares the side in the Ably-Agent header (RSC7d) so that
// traffic from clients constructed by this package is classified as
// server-side.
const agentName = "ably-pubsub-server"

// HTTPClient is a server Pub/Sub client that operates entirely over HTTP.
type HTTPClient = ably.HTTPClient

// RealtimeClient is a server Pub/Sub client with a realtime connection.
type RealtimeClient = ably.Realtime

// HTTPChannels is the collection of channels accessible from an
// [HTTPClient], via its Channels field.
type HTTPChannels = ably.HTTPChannels

// HTTPChannel is a single channel accessed over HTTP, as returned by
// [HTTPChannels.Get].
type HTTPChannel = ably.HTTPChannel

// HTTPPresence gives access to a channel's presence set over HTTP, via
// [HTTPChannel]'s Presence field.
type HTTPPresence = ably.HTTPPresence

// HTTPRequest is a request prepared by [HTTPClient.Request], ready to be
// performed by its Pages or Items methods.
type HTTPRequest = ably.HTTPRequest

// HTTPPaginatedResponse is an iterator over the pages of a response to
// [HTTPClient.Request].
type HTTPPaginatedResponse = ably.HTTPPaginatedResponse

// HTTPPaginatedItems is an iterator over the items of a response to
// [HTTPClient.Request].
type HTTPPaginatedItems = ably.HTTPPaginatedItems

// NewHTTPClient constructs a server Pub/Sub client that operates entirely
// over HTTP: publish, history, presence reads, stats, token issuing.
func NewHTTPClient(opts ...ably.ClientOption) (*HTTPClient, error) {
	opts = append(opts[:len(opts):len(opts)], ably.WithAgents(map[string]string{agentName: ""}))
	return ably.NewHTTPClient(opts...)
}

// NewRealtimeClient constructs a server Pub/Sub client with a persistent
// realtime connection: everything the HTTP client does, plus subscribing
// to channels and entering presence.
func NewRealtimeClient(opts ...ably.ClientOption) (*RealtimeClient, error) {
	opts = append(opts[:len(opts):len(opts)], ably.WithAgents(map[string]string{agentName: ""}))
	return ably.NewRealtime(opts...)
}
