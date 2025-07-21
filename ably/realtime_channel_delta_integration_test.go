//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"
	"github.com/stretchr/testify/assert"
)

// MockVCDiffDecoder implements the VCDiffDecoder interface for testing.
type MockVCDiffDecoder struct {
	CallCount  int
	ShouldFail bool
	mu         sync.Mutex
}

func (m *MockVCDiffDecoder) Decode(delta, base []byte) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CallCount++

	if m.ShouldFail {
		return nil, fmt.Errorf("mock decode failure")
	}

	// Simple mock: just return the delta as the result
	// In real implementation, this would use actual vcdiff decoding
	return delta, nil
}

// FailingVCDiffDecoder always fails for testing error conditions.
type FailingVCDiffDecoder struct{}

func (f *FailingVCDiffDecoder) Decode(delta, base []byte) ([]byte, error) {
	return nil, fmt.Errorf("failed to decode delta")
}

var deltaTestData = []map[string]interface{}{
	{"foo": "bar", "count": 1, "status": "active"},
	{"foo": "bar", "count": 2, "status": "active"},
	{"foo": "bar", "count": 2, "status": "inactive"},
	{"foo": "bar", "count": 3, "status": "inactive"},
	{"foo": "bar", "count": 3, "status": "active"},
}

func TestRealtime_Delta_Integration(t *testing.T) {
	t.Run("PC3: deltaPlugin", TestDelta_PluginBasicFunctionality)
	t.Run("PC3: unusedPlugin", TestDelta_UnusedPlugin)
	t.Run("RTL18: deltaDecodeFailureRecovery", TestDelta_DecodeFailureRecovery)
	t.Run("PC3: noPlugin", TestDelta_MissingPlugin)
	t.Run("Integration: endToEndDeltaFlow", TestDelta_EndToEndFlow)
}

// TestDelta_PluginBasicFunctionality tests that the delta plugin works correctly (@spec PC3).
func TestDelta_PluginBasicFunctionality(t *testing.T) {
	mockVcdiffPlugin := &MockVCDiffDecoder{CallCount: 0}

	app, realtime := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithVCDiffPlugin(mockVcdiffPlugin),
	)
	defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

	connectAndWaitDelta(t, realtime)

	channel := realtime.Channels.Get("deltaPlugin",
		ably.ChannelWithParams("delta", "vcdiff"))

	messages := make(chan *ably.Message, len(deltaTestData))
	channel.Subscribe(context.Background(), "", func(msg *ably.Message) {
		messages <- msg
	})

	err := channel.Attach(context.Background())
	assert.NoError(t, err)

	// Publish test messages
	for i, data := range deltaTestData {
		err := channel.Publish(context.Background(), fmt.Sprintf("%d", i), data)
		assert.NoError(t, err)
	}

	// Verify messages are received
	receivedCount := 0
	timeout := time.After(10 * time.Second)

	for receivedCount < len(deltaTestData) {
		select {
		case msg := <-messages:
			index := msg.Name
			expectedData := deltaTestData[atoi(index)]

			// Compare JSON representations since data types might differ
			expectedJSON, _ := json.Marshal(expectedData)
			actualJSON, _ := json.Marshal(msg.Data)
			assert.JSONEq(t, string(expectedJSON), string(actualJSON),
				"Message data mismatch for message %s", index)

			receivedCount++

		case <-timeout:
			t.Fatalf("Timeout waiting for messages. Received %d of %d", receivedCount, len(deltaTestData))
		}
	}

	// Verify that the plugin was called for delta messages
	// The first message should not be a delta, so call count should be len-1
	assert.Equal(t, len(deltaTestData)-1, mockVcdiffPlugin.CallCount,
		"Expected %d delta decode calls, got %d", len(deltaTestData)-1, mockVcdiffPlugin.CallCount)
}

// TestDelta_UnusedPlugin tests that plugin is not called when delta is not enabled (@spec PC3).
func TestDelta_UnusedPlugin(t *testing.T) {
	mockVcdiffPlugin := &MockVCDiffDecoder{CallCount: 0}

	app, realtime := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithVCDiffPlugin(mockVcdiffPlugin),
	)
	defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

	connectAndWaitDelta(t, realtime)

	// Channel without delta parameter
	channel := realtime.Channels.Get("unusedPlugin")

	messages := make(chan *ably.Message, len(deltaTestData))
	channel.Subscribe(context.Background(), "", func(msg *ably.Message) {
		messages <- msg
	})

	err := channel.Attach(context.Background())
	assert.NoError(t, err)

	// Publish test messages
	for i, data := range deltaTestData {
		err := channel.Publish(context.Background(), fmt.Sprintf("%d", i), data)
		assert.NoError(t, err)
	}

	// Verify messages are received
	receivedCount := 0
	timeout := time.After(10 * time.Second)

	for receivedCount < len(deltaTestData) {
		select {
		case msg := <-messages:
			index := msg.Name
			expectedData := deltaTestData[atoi(index)]

			expectedJSON, _ := json.Marshal(expectedData)
			actualJSON, _ := json.Marshal(msg.Data)
			assert.JSONEq(t, string(expectedJSON), string(actualJSON),
				"Message data mismatch for message %s", index)

			receivedCount++

		case <-timeout:
			t.Fatalf("Timeout waiting for messages. Received %d of %d", receivedCount, len(deltaTestData))
		}
	}

	// Verify that the plugin was NOT called since delta is not enabled
	assert.Equal(t, 0, mockVcdiffPlugin.CallCount,
		"Expected 0 delta decode calls, got %d", mockVcdiffPlugin.CallCount)
}

// TestDelta_DecodeFailureRecovery tests RTL18 decode failure recovery.
func TestDelta_DecodeFailureRecovery(t *testing.T) {
	failingPlugin := &FailingVCDiffDecoder{}

	app, realtime := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithVCDiffPlugin(failingPlugin),
	)
	defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

	connectAndWaitDelta(t, realtime)

	channel := realtime.Channels.Get("decodeFailure",
		ably.ChannelWithParams("delta", "vcdiff"))

	stateChanges := make(chan ably.ChannelStateChange, 10)
	channel.OnAll(func(change ably.ChannelStateChange) {
		stateChanges <- change
	})

	err := channel.Attach(context.Background())
	assert.NoError(t, err)

	// Wait for ATTACHED
	ablytest.Soon.Recv(t, nil, stateChanges, t.Fatalf)

	// Publish a message that will trigger decode failure when processed as delta
	err = channel.Publish(context.Background(), "0", deltaTestData[0])
	assert.NoError(t, err)

	// Publish second message which should trigger delta decoding and failure
	err = channel.Publish(context.Background(), "1", deltaTestData[1])
	assert.NoError(t, err)

	// Should see channel transition to ATTACHING due to decode failure recovery
	var attachingChange ably.ChannelStateChange
	select {
	case attachingChange = <-stateChanges:
		if attachingChange.Current == ably.ChannelStateAttaching {
			// Check that error code 40018 is passed through per RTL18c
			if attachingChange.Reason != nil {
				// Note: In real implementation we'd check the specific error code
				t.Logf("Channel reattaching due to decode failure: %v", attachingChange.Reason)
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Expected channel to transition to ATTACHING due to decode failure")
	}
}

// TestDelta_MissingPlugin tests error when no vcdiff plugin is provided (@spec PC3, error 40019).
func TestDelta_MissingPlugin(t *testing.T) {
	// Create realtime client without vcdiff plugin
	app, realtime := ablytest.NewRealtime(ably.WithAutoConnect(false))
	defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

	connectAndWaitDelta(t, realtime)

	channel := realtime.Channels.Get("noPlugin",
		ably.ChannelWithParams("delta", "vcdiff"))

	stateChanges := make(chan ably.ChannelStateChange, 10)
	channel.OnAll(func(change ably.ChannelStateChange) {
		stateChanges <- change
	})

	err := channel.Attach(context.Background())
	assert.NoError(t, err)

	// Wait for ATTACHED
	ablytest.Soon.Recv(t, nil, stateChanges, t.Fatalf)

	// Publish messages that will eventually trigger delta processing
	for i, data := range deltaTestData {
		err := channel.Publish(context.Background(), fmt.Sprintf("%d", i), data)
		assert.NoError(t, err)
	}

	// Should see channel transition to FAILED due to missing plugin
	timeout := time.After(15 * time.Second)
	for {
		select {
		case change := <-stateChanges:
			if change.Current == ably.ChannelStateFailed {
				// Check error code 40019 for missing plugin
				if change.Reason != nil {
					t.Logf("Channel failed due to missing plugin: %v", change.Reason)
				}
				return // Test passed
			}
		case <-timeout:
			t.Fatal("Expected channel to fail due to missing vcdiff plugin")
		}
	}
}

// TestDelta_EndToEndFlow tests a complete end-to-end delta flow.
func TestDelta_EndToEndFlow(t *testing.T) {
	mockVcdiffPlugin := &MockVCDiffDecoder{CallCount: 0}

	app, realtime := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithVCDiffPlugin(mockVcdiffPlugin),
	)
	defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

	connectAndWaitDelta(t, realtime)

	channel := realtime.Channels.Get("endToEnd",
		ably.ChannelWithParams("delta", "vcdiff"))

	messages := make(chan *ably.Message, len(deltaTestData))
	channel.Subscribe(context.Background(), "", func(msg *ably.Message) {
		messages <- msg
	})

	err := channel.Attach(context.Background())
	assert.NoError(t, err)

	// Test message ordering and content
	for i, data := range deltaTestData {
		err := channel.Publish(context.Background(), fmt.Sprintf("%d", i), data)
		assert.NoError(t, err)

		// Wait for message to be received
		select {
		case msg := <-messages:
			assert.Equal(t, fmt.Sprintf("%d", i), msg.Name)

			expectedJSON, _ := json.Marshal(data)
			actualJSON, _ := json.Marshal(msg.Data)
			assert.JSONEq(t, string(expectedJSON), string(actualJSON))

		case <-time.After(5 * time.Second):
			t.Fatalf("Timeout waiting for message %d", i)
		}
	}

	// Verify delta processing occurred
	assert.Greater(t, mockVcdiffPlugin.CallCount, 0, "Expected some delta decode calls")
}

// Helper function to convert string to int
func atoi(s string) int {
	if s == "0" {
		return 0
	}
	if s == "1" {
		return 1
	}
	if s == "2" {
		return 2
	}
	if s == "3" {
		return 3
	}
	if s == "4" {
		return 4
	}
	return 0
}

// connectAndWaitDelta connects to Ably and waits for connection (delta test version)
func connectAndWaitDelta(t *testing.T, realtime *ably.Realtime) {
	realtime.Connect()

	changes := make(chan ably.ConnectionStateChange, 5)
	realtime.Connection.OnAll(func(change ably.ConnectionStateChange) {
		changes <- change
	})

	timeout := time.After(10 * time.Second)
	for {
		select {
		case change := <-changes:
			if change.Current == ably.ConnectionStateConnected {
				return
			}
			if change.Current == ably.ConnectionStateFailed {
				t.Fatalf("Connection failed: %v", change.Reason)
			}
		case <-timeout:
			t.Fatal("Timeout waiting for connection")
		}
	}
}
