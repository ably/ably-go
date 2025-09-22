//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

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

	var channelStates []ably.ChannelStateChange
	channel.OnAll(func(change ably.ChannelStateChange) {
		channelStates = append(channelStates, change)
	})
	var receivedMsgs []*ably.Message
	channel.SubscribeAll(context.Background(), func(msg *ably.Message) {
		receivedMsgs = append(receivedMsgs, msg)
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

	// Wait for messages to be received
	err = ablytest.Wait(ablytest.AssertionWaiter(func() bool {
		return len(receivedMsgs) == len(testData)
	}), nil)
	assert.NoError(t, err)

	// Loop over each received message and compare with test data
	for i, msg := range receivedMsgs {
		expectedData := testData[i]

		// Compare JSON representations since data types might differ
		expectedJSON, _ := json.Marshal(expectedData)
		actualJSON, _ := json.Marshal(msg.Data)
		assert.JSONEq(t, string(expectedJSON), string(actualJSON),
			"Message data mismatch for message %d", i)
	}
	// Verify channel state changes
	assert.Equal(t, len(channelStates), 2, "Expected 2 channel state changes")
	assert.Equal(t, channelStates[0].Current, ably.ChannelStateAttaching, "Expected first state to be ATTACHING")
	assert.Equal(t, channelStates[1].Current, ably.ChannelStateAttached, "Expected second state to be ATTACHED")
}

var testData2 = []map[string]interface{}{
	{"foo": "bar", "count": 1, "status": "active"},
	{"foo": "bar", "count": 1, "status": "active"},
	{"foo": "bar", "count": 2, "status": "inactive"},
	{"foo": "bar", "count": 3, "status": "inactive"},
	{"foo": "bar", "count": 4, "status": "active"},
}

func TestDelta_PluginRecovery(t *testing.T) {
	dial := func(proto string, url *url.URL, timeout time.Duration) (ably.Conn, error) {
		return ably.DialWebsocket(proto, url, timeout)
	}
	wrappedDialWebsocket, interceptMsg := DialIntercept(dial)

	app, realtime := ablytest.NewRealtime(
		ably.WithVCDiffPlugin(ably.NewVCDiffPlugin()),
		ably.WithDial(wrappedDialWebsocket),
	)
	defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

	err := ablytest.Wait(ablytest.ConnWaiter(realtime, nil, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err, "Connect()=%v", err)

	channel := realtime.Channels.Get("deltaPlugin", ably.ChannelWithVCDiff())

	var channelStates []ably.ChannelStateChange
	channel.OnAll(func(change ably.ChannelStateChange) {
		channelStates = append(channelStates, change)
	})
	var receivedMsgs []*ably.Message
	channel.SubscribeAll(context.Background(), func(msg *ably.Message) {
		receivedMsgs = append(receivedMsgs, msg)
	})

	err = channel.Attach(context.Background())
	assert.NoError(t, err)

	// Wait for the channel to be attached before publishing
	err = ablytest.Wait(ablytest.AssertionWaiter(func() bool {
		return channel.State() == ably.ChannelStateAttached
	}), nil)
	assert.NoError(t, err)

	// Publish test messages in a separate goroutine
	readyForNext, ready := context.WithCancel(context.Background())
	go func() {
		for i, data := range testData2 {
			err := channel.Publish(context.Background(), fmt.Sprintf("%d", i), data)
			assert.NoError(t, err)
			<-readyForNext.Done() // Wait until it's ok to publish the next message
		}
	}()

	// Ignore the first base message
	<-interceptMsg(canceledCtx, ably.ActionMessage)

	// Intercept and pre-process next message
	ctx, cancel := context.WithCancel(context.Background())
	deltaMsgCh := interceptMsg(ctx, ably.ActionMessage)
	ready() // Allow publishing to start
	deltaMsg := (<-deltaMsgCh).Messages[0]
	deltaMsg.Extras["delta"] = map[string]interface{}{
		"format": "vcdiff",
		"from":   "non-existent-message-id",
	} // Corrupt the delta info to force recovery
	cancel() // Process this message

	// Wait for messages to be received
	err = ablytest.Wait(ablytest.AssertionWaiter(func() bool {
		return len(receivedMsgs) == len(testData)
	}), nil)
	assert.NoError(t, err)

	// Loop over each received message and compare with test data
	for i, msg := range receivedMsgs {
		expectedData := testData2[i]

		// Compare JSON representations since data types might differ
		expectedJSON, _ := json.Marshal(expectedData)
		actualJSON, _ := json.Marshal(msg.Data)
		assert.JSONEq(t, string(expectedJSON), string(actualJSON),
			"Message data mismatch for message %d", i)
	}
	// Verify channel state changes
	assert.Equal(t, len(channelStates), 4, "Expected 2 channel state changes")
	assert.Equal(t, channelStates[0].Current, ably.ChannelStateAttaching, "Expected first state to be ATTACHING")
	assert.Equal(t, channelStates[1].Current, ably.ChannelStateAttached, "Expected second state to be ATTACHED")
	assert.Equal(t, channelStates[2].Current, ably.ChannelStateAttaching, "Expected third state to be ATTACHING")

	reattachReason := channelStates[2].Reason
	assert.NotNil(t, reattachReason, "Expected reattach reason to be non-nil")
	assert.Equal(t, ably.ErrDeltaDecodingFailed, reattachReason.Code, "Expected reattach reason code to be ErrDeltaDecodingFailed")

	assert.Equal(t, channelStates[3].Current, ably.ChannelStateAttached, "Expected fourth state to be ATTACHED")
}
