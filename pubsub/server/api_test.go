package server_test

import (
	"context"
	"testing"
	"time"

	"github.com/ably/ably-go/pubsub/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReExportedAPIIsSelfContained builds both server clients and touches the
// re-exported API around them, so that a symbol dropped from the generated
// surface fails here rather than in a user's build.
func TestReExportedAPIIsSelfContained(t *testing.T) {
	http, err := server.NewHTTPClient(
		server.WithKey("fake:key"),
		server.WithClientID("me"),
		server.WithLogLevel(server.LogError),
		server.WithHTTPRequestTimeout(time.Second),
		server.WithIdempotentHTTPPublishing(true),
	)
	require.NoError(t, err)

	channel := http.Channels.Get("some-channel", server.ChannelWithCipherKey([]byte("0123456789abcdef0123456789abcdef")))
	assert.Equal(t, "some-channel", channel.Name)

	var (
		_ *server.HTTPChannels                     = http.Channels
		_ *server.HTTPPresence                     = channel.Presence
		_ server.HistoryRequest                    = channel.History(server.HistoryWithLimit(10))
		_ server.PresenceRequest                   = channel.Presence.History(server.PresenceHistoryWithLimit(10))
		_ server.StatsRequest                      = http.Stats(server.StatsWithUnit(server.PeriodDay))
		_ server.HTTPRequest                       = http.Request("GET", "/time", server.RequestWithParams(nil))
		_ *server.Auth                             = http.Auth
		_ func(context.Context) (time.Time, error) = http.Time
	)

	realtime, err := server.NewRealtimeClient(
		server.WithKey("fake:key"),
		server.WithAutoConnect(false),
	)
	require.NoError(t, err)
	t.Cleanup(func() { realtime.Close() })

	assert.Equal(t, server.ConnectionStateInitialized, realtime.Connection.State())
	assert.Equal(t, server.ChannelStateInitialized, realtime.Channels.Get("some-channel").State())

	_, err = server.NewHTTPClient(server.WithKey("malformed"))
	var info *server.ErrorInfo
	require.ErrorAs(t, err, &info)
	assert.Equal(t, server.ErrInvalidCredential, info.Code)
}
