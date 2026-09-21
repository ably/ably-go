package pubsub_test

import (
	"context"
	"testing"
	"time"

	"github.com/ably/ably-pubsub-go/device"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReExportedAPIIsSelfContained builds a realistic device client and
// touches the re-exported API around it, so that a symbol dropped from the
// generated surface fails here rather than in a user's build.
func TestReExportedAPIIsSelfContained(t *testing.T) {
	client, err := pubsub.NewClient(
		pubsub.WithKey("fake:key"),
		pubsub.WithClientID("me"),
		pubsub.WithAutoConnect(false),
		pubsub.WithEchoMessages(false),
		pubsub.WithLogLevel(pubsub.LogError),
		pubsub.WithRealtimeRequestTimeout(time.Second),
		pubsub.WithTokenDetails(&pubsub.TokenDetails{Token: "token"}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	channel := client.Channels.Get("some-channel",
		pubsub.ChannelWithParams("rewind", "1"),
		pubsub.ChannelWithModes(pubsub.ChannelModeSubscribe),
	)
	assert.Equal(t, pubsub.ChannelStateInitialized, channel.State())

	var (
		_ pubsub.ConnectionState      = client.Connection.State()
		_ *pubsub.RealtimeChannels    = client.Channels
		_ *pubsub.RealtimePresence    = channel.Presence
		_ pubsub.HistoryRequest       = channel.History(pubsub.HistoryWithLimit(10), pubsub.HistoryWithDirection(pubsub.Backwards))
		_ pubsub.StatsRequest         = client.Stats(pubsub.StatsWithUnit(pubsub.PeriodHour))
		_ func(context.Context) error = channel.Attach
	)

	unsubscribe := client.Connection.OnAll(func(pubsub.ConnectionStateChange) {})
	unsubscribe()

	msg := &pubsub.Message{Name: "event", Data: "payload"}
	assert.Equal(t, "event", msg.Name)

	_, err = pubsub.NewClient(pubsub.WithKey("malformed"))
	var info *pubsub.ErrorInfo
	require.ErrorAs(t, err, &info)
	assert.Equal(t, pubsub.ErrInvalidCredential, info.Code)
}
