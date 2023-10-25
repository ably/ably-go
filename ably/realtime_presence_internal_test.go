//go:build !integration
// +build !integration

package ably

import (
	"bytes"
	"context"
	"errors"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestVerifyChanState_RTP16(t *testing.T) {
	tests := map[string]struct {
		channel     *RealtimeChannel
		expectedErr error
	}{
		`No error if the channel is in state: "INITIALIZED"`: {
			channel:     mockChannelWithState(&ChannelStateInitialized, nil),
			expectedErr: nil,
		},
		`No error if the channel is in state: "ATTACHING"`: {
			channel:     mockChannelWithState(&ChannelStateAttaching, nil),
			expectedErr: nil,
		},
		`No error if the channel is in state: "ATTACHED"`: {
			channel:     mockChannelWithState(&ChannelStateAttached, nil),
			expectedErr: nil,
		},
		`Error if the channel is in state: "SUSPENDED"`: {
			channel:     mockChannelWithState(&ChannelStateSuspended, nil),
			expectedErr: newError(91001, errors.New("unable to enter presence channel (invalid channel state: SUSPENDED)")),
		},
		`Error if the channel is in state: "DETACHING"`: {
			channel:     mockChannelWithState(&ChannelStateDetaching, nil),
			expectedErr: newError(91001, errors.New("unable to enter presence channel (invalid channel state: DETACHING)")),
		},
		`Error if the channel is in state: "DETACHED"`: {
			channel:     mockChannelWithState(&ChannelStateDetached, nil),
			expectedErr: newError(91001, errors.New("unable to enter presence channel (invalid channel state: DETACHED)")),
		},
		`Error if the channel is in state: "FAILED"`: {
			channel:     mockChannelWithState(&ChannelStateFailed, nil),
			expectedErr: newError(91001, errors.New("unable to enter presence channel (invalid channel state: FAILED)")),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			presence := newRealtimePresence(test.channel)
			err := presence.isValidChannelState()
			assert.Equal(t, test.expectedErr, err)
		})
	}
}

func TestSend(t *testing.T) {
	tests := map[string]struct {
		channel        *RealtimeChannel
		msg            Message
		expectedResult result
		expectedErr    error
	}{
		`No error sending presence if the channel is in state: "ATTACHED"`: {
			channel:     mockChannelWithState(&ChannelStateAttached, nil),
			msg:         Message{Name: "Hello"},
			expectedErr: (*ErrorInfo)(nil),
		},
		`Error if channel is: "ATTACHED" and connection is :"CLOSED"`: {
			channel:     mockChannelWithState(&ChannelStateAttached, &ConnectionStateClosed),
			msg:         Message{Name: "Hello"},
			expectedErr: newError(80017, errors.New("Connection unavailable")),
		},
		`Error if channel is: "DETACHED" and connection is :"CLOSED"`: {
			channel:     mockChannelWithState(&ChannelStateDetached, &ConnectionStateClosed),
			msg:         Message{Name: "Hello"},
			expectedErr: newError(91001, errors.New("unable to enter presence channel (invalid channel state: DETACHED)")),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			presence := newRealtimePresence(test.channel)
			err := presence.EnterClient(context.Background(), "clientId", &test.msg)
			assert.Equal(t, test.expectedErr, err.(*ErrorInfo))
		})
	}
}
