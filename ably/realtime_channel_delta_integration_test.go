//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"
	"github.com/stretchr/testify/assert"
)

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

	channelStateChanges := make(ably.ChannelStateChanges, 10)
	channel.OnAll(channelStateChanges.Receive)

	err = channel.Attach(ctx)
	assert.NoError(t, err)

	var channelStatechange ably.ChannelStateChange
	// Wait for ATTACHING
	ablytest.Soon.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
	assert.Equal(t, ably.ChannelStateAttaching, channelStatechange.Current, "expected ATTACHING; got %s", channelStatechange.Current.String())
	// Wait for ATTACHED
	ablytest.Soon.Recv(t, &channelStatechange, channelStateChanges, t.Fatalf)
	assert.Equal(t, ably.ChannelStateAttached, channelStatechange.Current, "expected ATTACHED; got %s", channelStatechange.Current.String())

	// Verify channel params are available
	params := channel.Params() // RTL4k1
	assert.NotEmpty(t, params, "Should receive channel params")
	assert.Equal(t, "vcdiff", params["delta"], "expected \"vcdiff\"; got %v", params["delta"])
}

var testData = []map[string]interface{}{
	{"foo": "bar", "count": 1, "status": "active"},
	{"foo": "bar", "count": 2, "status": "active"},
	{"foo": "bar", "count": 2, "status": "inactive"},
	{"foo": "bar", "count": 3, "status": "inactive"},
	{"foo": "bar", "count": 3, "status": "active"},
}

// TestDelta_PluginBasicFunctionality tests that the delta plugin works correctly (@spec PC3).
func TestDelta_PluginBasicFunctionality(t *testing.T) {

	app, realtime := ablytest.NewRealtime(ably.WithVCDiffPlugin(ably.NewVCDiffPlugin()))
	defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

	err := ablytest.Wait(ablytest.ConnWaiter(realtime, nil, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err, "Connect()=%v", err)

	channel := realtime.Channels.Get("deltaPlugin", ably.ChannelWithVCDiff())

	channelStatesCh := make(chan ably.ChannelStateChange, 10) // Buffered channel to avoid blocking
	channel.OnAll(func(change ably.ChannelStateChange) {
		channelStatesCh <- change
	})

	receivedMsgsCh := make(chan *ably.Message, len(testData))
	var totalReceivedMessages int64
	channel.SubscribeAll(context.Background(), func(msg *ably.Message) {
		receivedMsgsCh <- msg
		atomic.AddInt64(&totalReceivedMessages, 1)
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

	// Loop over each received message and compare with test data
	for i := 0; i < len(testData); i++ {
		msg := <-receivedMsgsCh
		expectedData := testData[i]

		// Compare JSON representations since data types might differ
		expectedJSON, _ := json.Marshal(expectedData)
		actualJSON, _ := json.Marshal(msg.Data)
		assert.JSONEq(t, string(expectedJSON), string(actualJSON),
			"Message data mismatch for message %d", i)
	}
	assert.Equal(t, len(testData), int(atomic.LoadInt64(&totalReceivedMessages)), "Expected to receive all published messages")

	// Collect all channel state changes from channel
	var channelStates []ably.ChannelStateChange
	for len(channelStatesCh) > 0 {
		change := <-channelStatesCh
		channelStates = append(channelStates, change)
	}

	// Verify channel state changes
	assert.Equal(t, len(channelStates), 2, "Expected 2 channel state changes")
	assert.Equal(t, channelStates[0].Current, ably.ChannelStateAttaching, "Expected first state to be ATTACHING")
	assert.Equal(t, channelStates[1].Current, ably.ChannelStateAttached, "Expected second state to be ATTACHED")
}

func TestDeltaPluginRecovery(t *testing.T) {
	t.Run("WithMessagesOutOfOrder", func(t *testing.T) {
		testDeltaPluginRecovery(t, func(pm *ably.ProtocolMessage, noOfReattachCausedByDeltaDecodingFailure *int) {
			if pm.Action == ably.ActionMessage && len(pm.Messages) > 0 {
				protoMsg := pm.Messages[0]
				if deltaInfo, ok := protoMsg.Extras["delta"].(map[string]interface{}); ok {
					if format, ok := deltaInfo["format"].(string); ok && format == "vcdiff" {
						// Simulate a scenario where the delta cannot be applied by corrupting the "from" field
						deltaInfo["from"] = "non-existent-message-id"
						*noOfReattachCausedByDeltaDecodingFailure++
					}
				}
			}
		})
	})

	t.Run("WithCorruptDelta", func(t *testing.T) {
		testDeltaPluginRecovery(t, func(pm *ably.ProtocolMessage, noOfReattachCausedByDeltaDecodingFailure *int) {
			if pm.Action == ably.ActionMessage && len(pm.Messages) > 0 {
				protoMsg := pm.Messages[0]
				if deltaInfo, ok := protoMsg.Extras["delta"].(map[string]interface{}); ok {
					if format, ok := deltaInfo["format"].(string); ok && format == "vcdiff" {
						// Corrupt the delta data to simulate a decoding failure
						switch v := protoMsg.Data.(type) {
						case []byte:
							v[0] = 0 // Corrupt the first byte
							protoMsg.Data = v
						case string:
							protoMsg.Data = "Q29ycnVwdCBkYXRh" // Base64 for "Corrupt data"
						}
						*noOfReattachCausedByDeltaDecodingFailure++
					}
				}
			}
		})
	})
}

func testDeltaPluginRecovery(t *testing.T, messageProcessor func(*ably.ProtocolMessage, *int)) {
	noOfReattachCausedByDeltaDecodingFailure := 0
	wrappedDialWebsocket := DialWithMessagePreProcessor(func(pm *ably.ProtocolMessage) {
		messageProcessor(pm, &noOfReattachCausedByDeltaDecodingFailure)
	})

	app, realtime := ablytest.NewRealtime(
		ably.WithVCDiffPlugin(ably.NewVCDiffPlugin()),
		ably.WithDial(wrappedDialWebsocket),
	)
	defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

	err := ablytest.Wait(ablytest.ConnWaiter(realtime, nil, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err, "Connect()=%v", err)

	channel := realtime.Channels.Get("deltaPlugin", ably.ChannelWithVCDiff())

	channelStatesCh := make(chan ably.ChannelStateChange, 50) // Larger buffer for recovery tests
	channel.OnAll(func(change ably.ChannelStateChange) {
		channelStatesCh <- change
	})
	receivedMsgsCh := make(chan *ably.Message, len(testData))
	var totalReceivedMessages int64
	channel.SubscribeAll(context.Background(), func(msg *ably.Message) {
		receivedMsgsCh <- msg
		atomic.AddInt64(&totalReceivedMessages, 1)
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

	// Loop over each received message and compare with test data
	for i := 0; i < len(testData); i++ {
		msg := <-receivedMsgsCh
		expectedData := testData[i]

		// Compare JSON representations since data types might differ
		expectedJSON, _ := json.Marshal(expectedData)
		actualJSON, _ := json.Marshal(msg.Data)
		assert.JSONEq(t, string(expectedJSON), string(actualJSON),
			"Message data mismatch for message %d", i)
	}
	assert.Equal(t, len(testData), int(atomic.LoadInt64(&totalReceivedMessages)), "Expected to receive all published messages")

	// Collect all channel state changes from channel
	var channelStates []ably.ChannelStateChange
	for len(channelStatesCh) > 0 {
		change := <-channelStatesCh
		channelStates = append(channelStates, change)
	}

	// Verify channel state changes
	assert.Equal(t, ably.ChannelStateAttaching, channelStates[0].Current, "Expected first state to be ATTACHING")
	assert.Equal(t, ably.ChannelStateAttached, channelStates[1].Current, "Expected second state to be ATTACHED")

	originalStates := 2                                         // ATTACHING, ATTACHED
	extraStates := 2 * noOfReattachCausedByDeltaDecodingFailure // Each delta decoding failure causes 2 extra states: ATTACHING, ATTACHED
	for i := originalStates; i < originalStates+extraStates; i = i + 2 {
		assert.Equal(t, ably.ChannelStateAttaching, channelStates[i].Current, "Expected state %d to be ATTACHING", i)
		reattachReason := channelStates[i].Reason
		assert.NotNil(t, reattachReason, "Expected reattach reason to be non-nil")
		assert.Equal(t, ably.ErrDeltaDecodingFailed, reattachReason.Code, "Expected reattach reason code to be ErrDeltaDecodingFailed")
		assert.Equal(t, ably.ChannelStateAttached, channelStates[i+1].Current, "Expected state %d to be ATTACHED", i+1)
	}
}
