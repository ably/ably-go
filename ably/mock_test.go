// mocks and helpers for unit tests.

package ably

import (
	"bytes"
	"log"
)

var (
	buffer     bytes.Buffer
	mocklogger = log.New(&buffer, "logger: ", log.Lshortfile)
)

// mockChannelWithState is a test helper that returns a mock channel in a specified state
func mockChannelWithState(channelState *ChannelState, connectionState *ConnectionState) *RealtimeChannel {
	mockChannel := RealtimeChannel{
		client: &Realtime{
			rest: &REST{
				log: logger{l: &stdLogger{mocklogger}},
			},
			Connection: &Connection{},
		},
	}
	if channelState != nil {
		mockChannel.state = *channelState
	}
	if connectionState != nil {
		mockChannel.client.Connection.state = *connectionState
	}
	return &mockChannel
}
