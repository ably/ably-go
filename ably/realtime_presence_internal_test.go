//go:build !integration
// +build !integration

package ably

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerifyChanState(t *testing.T) {
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
		`No error if the channel is in state: "SUSPENDED"`: {
			channel:     mockChannelWithState(&ChannelStateSuspended, nil),
			expectedErr: nil,
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
			err := presence.verifyChanState()
			assert.Equal(t, test.expectedErr, err)
		})
	}
}

func TestSend(t *testing.T) {
	tests := map[string]struct {
		channel        *RealtimeChannel
		msg            PresenceMessage
		expectedResult result
		expectedErr    error
	}{
		`No error sending presence if the channel is in state: "ATTACHED"`: {
			channel: mockChannelWithState(&ChannelStateAttached, nil),
			msg: PresenceMessage{
				Message: Message{Name: "Hello"},
				Action:  PresenceActionEnter,
			},
			expectedErr: nil,
		},
		`Error if channel is: "DETACHED" and connection is :"CLOSED"`: {
			channel: mockChannelWithState(&ChannelStateDetached, &ConnectionStateClosed),
			msg: PresenceMessage{
				Message: Message{Name: "Hello"},
				Action:  PresenceActionEnter,
			},
			expectedErr: newError(80000, errors.New("cannot Attach channel because connection is in CLOSED state")),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			presence := newRealtimePresence(test.channel)
			_, err := presence.send(&test.msg)
			assert.Equal(t, test.expectedErr, err)
		})
	}
}
