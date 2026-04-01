//go:build !integration
// +build !integration

package ably

import (
	"io"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

// This test verifies that when an ACK message contains a "res" array with multiple results,
// each result is correctly associated with its corresponding message based on msgSerial.
//
// Scenario: SDK sends multiple messages with consecutive msgSerials
// Server responds with ACK: msgSerial=X, count=N, res=[result1, result2, ..., resultN]
// Expected behavior: Each message should receive serials from only its corresponding res element
func TestPendingEmitter_AckResult(t *testing.T) {
	t.Run("two messages with single serial each", func(t *testing.T) {
		testLogger := logger{l: &stdLogger{log.New(io.Discard, "", 0)}}
		emitter := newPendingEmitter(testLogger)

		// Track what serials each message receives
		var msg1Serials, msg2Serials []string

		// Create two protocol messages with consecutive msgSerials
		protoMsg1 := &protocolMessage{
			MsgSerial: 5,
			Action:    actionMessage,
			Channel:   "test-channel",
		}
		callback1 := &ackCallback{
			onAckWithSerials: func(serials []string, err error) {
				msg1Serials = serials
			},
		}

		protoMsg2 := &protocolMessage{
			MsgSerial: 6,
			Action:    actionMessage,
			Channel:   "test-channel",
		}
		callback2 := &ackCallback{
			onAckWithSerials: func(serials []string, err error) {
				msg2Serials = serials
			},
		}

		// Enqueue both messages
		emitter.Enqueue(protoMsg1, callback1)
		emitter.Enqueue(protoMsg2, callback2)

		// Simulate receiving an ACK with msgSerial=5, count=2, and res array with two distinct results
		ackMsg := &protocolMessage{
			Action:    actionAck,
			MsgSerial: 5,
			Count:     2,
			Res: []*protocolPublishResult{
				{Serials: []string{"serial-for-msg-5"}}, // Should only go to message 1
				{Serials: []string{"serial-for-msg-6"}}, // Should only go to message 2
			},
		}

		// Process the ACK
		emitter.Ack(ackMsg, nil)

		// Verify each message received only its corresponding serials
		assert.Equal(t, []string{"serial-for-msg-5"}, msg1Serials,
			"Message 1 (msgSerial=5) should only receive serials from res[0]")
		assert.Equal(t, []string{"serial-for-msg-6"}, msg2Serials,
			"Message 2 (msgSerial=6) should only receive serials from res[1]")
	})

	t.Run("two messages with multiple serials each", func(t *testing.T) {
		testLogger := logger{l: &stdLogger{log.New(io.Discard, "", 0)}}
		emitter := newPendingEmitter(testLogger)

		var msg1Serials, msg2Serials []string

		protoMsg1 := &protocolMessage{
			MsgSerial: 10,
			Action:    actionMessage,
			Channel:   "test-channel",
		}
		callback1 := &ackCallback{
			onAckWithSerials: func(serials []string, err error) {
				msg1Serials = serials
			},
		}

		protoMsg2 := &protocolMessage{
			MsgSerial: 11,
			Action:    actionMessage,
			Channel:   "test-channel",
		}
		callback2 := &ackCallback{
			onAckWithSerials: func(serials []string, err error) {
				msg2Serials = serials
			},
		}

		emitter.Enqueue(protoMsg1, callback1)
		emitter.Enqueue(protoMsg2, callback2)

		// ACK with multiple serials per result
		ackMsg := &protocolMessage{
			Action:    actionAck,
			MsgSerial: 10,
			Count:     2,
			Res: []*protocolPublishResult{
				{Serials: []string{"serial-10-a", "serial-10-b"}}, // Both should go to message 1
				{Serials: []string{"serial-11-a", "serial-11-b"}}, // Both should go to message 2
			},
		}

		emitter.Ack(ackMsg, nil)

		assert.Equal(t, []string{"serial-10-a", "serial-10-b"}, msg1Serials,
			"Message 1 (msgSerial=10) should receive both serials from res[0]")
		assert.Equal(t, []string{"serial-11-a", "serial-11-b"}, msg2Serials,
			"Message 2 (msgSerial=11) should receive both serials from res[1]")
	})

	t.Run("three messages", func(t *testing.T) {
		testLogger := logger{l: &stdLogger{log.New(io.Discard, "", 0)}}
		emitter := newPendingEmitter(testLogger)

		var msg1Serials, msg2Serials, msg3Serials []string

		protoMsg1 := &protocolMessage{MsgSerial: 1, Action: actionMessage, Channel: "test"}
		callback1 := &ackCallback{onAckWithSerials: func(serials []string, err error) { msg1Serials = serials }}

		protoMsg2 := &protocolMessage{MsgSerial: 2, Action: actionMessage, Channel: "test"}
		callback2 := &ackCallback{onAckWithSerials: func(serials []string, err error) { msg2Serials = serials }}

		protoMsg3 := &protocolMessage{MsgSerial: 3, Action: actionMessage, Channel: "test"}
		callback3 := &ackCallback{onAckWithSerials: func(serials []string, err error) { msg3Serials = serials }}

		emitter.Enqueue(protoMsg1, callback1)
		emitter.Enqueue(protoMsg2, callback2)
		emitter.Enqueue(protoMsg3, callback3)

		ackMsg := &protocolMessage{
			Action:    actionAck,
			MsgSerial: 1,
			Count:     3,
			Res: []*protocolPublishResult{
				{Serials: []string{"serial-1"}},
				{Serials: []string{"serial-2"}},
				{Serials: []string{"serial-3"}},
			},
		}

		emitter.Ack(ackMsg, nil)

		assert.Equal(t, []string{"serial-1"}, msg1Serials, "Message 1 should receive serial-1")
		assert.Equal(t, []string{"serial-2"}, msg2Serials, "Message 2 should receive serial-2")
		assert.Equal(t, []string{"serial-3"}, msg3Serials, "Message 3 should receive serial-3")
	})
}
