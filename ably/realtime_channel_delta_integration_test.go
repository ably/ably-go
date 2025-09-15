//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"
	"github.com/stretchr/testify/assert"
)

var testData = []map[string]interface{}{
	{"foo": "bar", "count": 1, "status": "active"},
	{"foo": "bar", "count": 2, "status": "active"},
	{"foo": "bar", "count": 2, "status": "inactive"},
	{"foo": "bar", "count": 3, "status": "inactive"},
	{"foo": "bar", "count": 3, "status": "active"},
}

func TestRealtime_ChannelParams_DeltaSupport(t *testing.T) {
	// Simple mock VCDiff decoder for testing
	ablyVCDiffPlugin := ably.NewVCDiffPlugin()

	app, client := ablytest.NewRealtime(ably.WithVCDiffPlugin(ablyVCDiffPlugin))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err, "Connect()=%v", err)

	channel := client.Channels.Get("test",
		ably.ChannelWithParams("test", "blah"),
		ably.ChannelWithParams("test2", "blahblah"),
		ably.ChannelWithVCDiff())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	channelStateChanges := make(chan ably.ChannelStateChange, 10)
	channel.OnAll(func(change ably.ChannelStateChange) {
		channelStateChanges <- change
	})

	err = channel.Attach(ctx)
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

// TestDelta_PluginBasicFunctionality tests that the delta plugin works correctly (@spec PC3).
func Skip_TestDelta_PluginBasicFunctionality(t *testing.T) {
	// mockVcdiffPlugin := &MockVCDiffDecoder{CallCount: 0}

	app, realtime := ablytest.NewRealtime(ably.WithVCDiffPlugin(ably.NewVCDiffPlugin()))
	defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

	err := ablytest.Wait(ablytest.ConnWaiter(realtime, nil, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err, "Connect()=%v", err)

	channel := realtime.Channels.Get("deltaPlugin", ably.ChannelWithVCDiff())

	messages := make(chan *ably.Message, len(testData))
	channel.SubscribeAll(context.Background(), func(msg *ably.Message) {
		messages <- msg
	})

	err = channel.Attach(context.Background())
	assert.NoError(t, err)

	// Wait for the channel to be attached before publishing
	err = ablytest.Wait(ablytest.AssertionWaiter(func() bool {
		return channel.State() == ably.ChannelStateAttached
	}), nil)
	assert.NoError(t, err)

	// Publish test messages
	for i, data := range testData {
		err := channel.Publish(context.Background(), fmt.Sprintf("%d", i), data)
		assert.NoError(t, err)
	}

	// Verify messages are received
	receivedCount := 0
	timeout := time.After(10 * time.Second)

	for receivedCount < len(testData) {
		select {
		case msg := <-messages:
			index := msg.Name
			expectedData := testData[receivedCount]

			// Compare JSON representations since data types might differ
			expectedJSON, _ := json.Marshal(expectedData)
			actualJSON, _ := json.Marshal(msg.Data)
			assert.JSONEq(t, string(expectedJSON), string(actualJSON),
				"Message data mismatch for message %s", index)

			receivedCount++

		case <-timeout:
			t.Fatalf("Timeout waiting for messages. Received %d of %d", receivedCount, len(testData))
		}
	}
	assert.Equal(t, receivedCount, len(testData)-1)
}
