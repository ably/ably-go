//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"
	"github.com/stretchr/testify/assert"
)

func TestRealtime_ChannelParams_DeltaSupport(t *testing.T) {
	t.Run("RTL4k: delta parameter in ATTACH message", TestDelta_ParameterInAttachMessage)
	t.Run("RTL4k1: delta parameter exposed on channel", TestDelta_ParameterOnChannel)
}

// TestDelta_ParameterInAttachMessage tests that delta=vcdiff param is properly transmitted in ATTACH message.
func TestDelta_ParameterInAttachMessage(t *testing.T) {
	mockVcdiffPlugin := &MockVCDiffDecoder{}

	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithVCDiffPlugin(mockVcdiffPlugin),
	)
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	connectAndWaitDelta(t, client)

	channel := client.Channels.Get("test",
		ably.ChannelWithParams("test", "blah"),
		ably.ChannelWithParams("test2", "blahblah"),
		ably.ChannelWithParams("delta", "vcdiff"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := channel.Attach(ctx)
	assert.NoError(t, err)

	// Verify channel params are properly set (RTL4k)
	params := channel.Params()
	assert.NotNil(t, params, "Channel params cannot be nil")
	assert.Equal(t, "blah", params["test"], "expected \"blah\"; got %v", params["test"])
	assert.Equal(t, "blahblah", params["test2"], "expected \"blahblah\"; got %v", params["test2"])
	assert.Equal(t, "vcdiff", params["delta"], "expected \"vcdiff\"; got %v", params["delta"])
}

// TestDelta_ParameterOnChannel tests that delta params are exposed as readonly field on ATTACHED message.
func TestDelta_ParameterOnChannel(t *testing.T) {
	mockVcdiffPlugin := &MockVCDiffDecoder{}

	app, client := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithVCDiffPlugin(mockVcdiffPlugin),
	)
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	connectAndWaitDelta(t, client)

	channel := client.Channels.Get("test",
		ably.ChannelWithParams("test", "blah"),
		ably.ChannelWithParams("test2", "blahblah"),
		ably.ChannelWithParams("delta", "vcdiff"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	channelStateChanges := make(chan ably.ChannelStateChange, 10)
	channel.OnAll(func(change ably.ChannelStateChange) {
		channelStateChanges <- change
	})

	err := channel.Attach(ctx)
	assert.NoError(t, err)

	// Wait for ATTACHING
	ablytest.Soon.Recv(t, nil, channelStateChanges, t.Fatalf)
	// Wait for ATTACHED
	ablytest.Soon.Recv(t, nil, channelStateChanges, t.Fatalf)

	// Verify channel params are available
	params := channel.Params() // RTL4k1
	assert.NotEmpty(t, params, "Should receive channel params")
	assert.Equal(t, "vcdiff", params["delta"], "expected \"vcdiff\"; got %v", params["delta"])
}
