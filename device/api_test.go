package device_test

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
	client, err := device.NewClient(
		device.WithKey("fake:key"),
		device.WithClientID("me"),
		device.WithAutoConnect(false),
		device.WithEchoMessages(false),
		device.WithLogLevel(device.LogError),
		device.WithRealtimeRequestTimeout(time.Second),
		device.WithTokenDetails(&device.TokenDetails{Token: "token"}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	channel := client.Channels.Get("some-channel",
		device.ChannelWithParams("rewind", "1"),
		device.ChannelWithModes(device.ChannelModeSubscribe),
	)
	assert.Equal(t, device.ChannelStateInitialized, channel.State())

	var (
		_ device.ConnectionState      = client.Connection.State()
		_ *device.RealtimeChannels    = client.Channels
		_ *device.RealtimePresence    = channel.Presence
		_ device.HistoryRequest       = channel.History(device.HistoryWithLimit(10), device.HistoryWithDirection(device.Backwards))
		_ device.StatsRequest         = client.Stats(device.StatsWithUnit(device.PeriodHour))
		_ func(context.Context) error = channel.Attach
	)

	unsubscribe := client.Connection.OnAll(func(device.ConnectionStateChange) {})
	unsubscribe()

	msg := &device.Message{Name: "event", Data: "payload"}
	assert.Equal(t, "event", msg.Name)

	_, err = device.NewClient(device.WithKey("malformed"))
	var info *device.ErrorInfo
	require.ErrorAs(t, err, &info)
	assert.Equal(t, device.ErrInvalidCredential, info.Code)
}
