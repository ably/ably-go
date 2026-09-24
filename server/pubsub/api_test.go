package pubsub_test

import (
	"context"
	"testing"
	"time"

	"github.com/ably/ably-pubsub-go/server/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReExportedAPIIsSelfContained builds both server clients and touches the
// re-exported API around them, so that a symbol dropped from the generated
// surface fails here rather than in a user's build.
func TestReExportedAPIIsSelfContained(t *testing.T) {
	http, err := pubsub.NewHTTPClient(
		pubsub.WithKey("fake:key"),
		pubsub.WithClientID("me"),
		pubsub.WithLogLevel(pubsub.LogError),
		pubsub.WithHTTPRequestTimeout(time.Second),
		pubsub.WithIdempotentHTTPPublishing(true),
	)
	require.NoError(t, err)

	channel := http.Channels.Get("some-channel", pubsub.ChannelWithCipherKey([]byte("0123456789abcdef0123456789abcdef")))
	assert.Equal(t, "some-channel", channel.Name)

	var (
		_ *pubsub.HTTPChannels                     = http.Channels
		_ *pubsub.HTTPPresence                     = channel.Presence
		_ pubsub.HistoryRequest                    = channel.History(pubsub.HistoryWithLimit(10))
		_ pubsub.PresenceRequest                   = channel.Presence.History(pubsub.PresenceHistoryWithLimit(10))
		_ pubsub.StatsRequest                      = http.Stats(pubsub.StatsWithUnit(pubsub.PeriodDay))
		_ pubsub.HTTPRequest                       = http.Request("GET", "/time", pubsub.RequestWithParams(nil))
		_ *pubsub.Auth                             = http.Auth
		_ func(context.Context) (time.Time, error) = http.Time
	)

	realtime, err := pubsub.NewRealtimeClient(
		pubsub.WithKey("fake:key"),
		pubsub.WithAutoConnect(false),
	)
	require.NoError(t, err)
	t.Cleanup(func() { realtime.Close() })

	assert.Equal(t, pubsub.ConnectionStateInitialized, realtime.Connection.State())
	assert.Equal(t, pubsub.ChannelStateInitialized, realtime.Channels.Get("some-channel").State())

	_, err = pubsub.NewHTTPClient(pubsub.WithKey("malformed"))
	var info *pubsub.ErrorInfo
	require.ErrorAs(t, err, &info)
	assert.Equal(t, pubsub.ErrInvalidCredential, info.Code)
}
