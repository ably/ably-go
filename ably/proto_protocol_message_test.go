//go:build !integration
// +build !integration

package ably_test

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMsgpackEncodesBytesAsBytes(t *testing.T) {
	msg := ably.Message{Data: []byte("abc")}

	encoded, err := ablyutil.MarshalMsgpack(msg)
	assert.NoError(t, err)

	var decodedMsg ably.Message
	err = ablyutil.UnmarshalMsgpack(encoded, &decodedMsg)
	require.NoError(t, err)
	assert.IsType(t, []byte{}, decodedMsg.Data)
}

func TestMsgpackEncodesStringAsString(t *testing.T) {
	msg := ably.Message{Data: "abc"}

	encoded, err := ablyutil.MarshalMsgpack(msg)
	assert.NoError(t, err)

	var decodedMsg ably.Message
	err = ablyutil.UnmarshalMsgpack(encoded, &decodedMsg)
	require.NoError(t, err)
	assert.IsType(t, "", decodedMsg.Data)
}

// TestProtocolMessageEncodeZeroSerials tests that zero-valued serials are
// explicitly encoded into msgpack (as required by the realtime API)
func TestProtocolMessageEncodeZeroSerials(t *testing.T) {
	msg := ably.ProtocolMessage{
		ID:               "test",
		MsgSerial:        0,
		ConnectionSerial: 0,
	}
	encoded, err := ablyutil.MarshalMsgpack(msg)
	assert.NoError(t, err)
	// expect a 3-element map with both the serial fields set to zero
	expected := []byte("\x83\xB0connectionSerial\x00\xA2id\xA4test\xA9msgSerial\x00")
	assert.True(t, bytes.Equal(encoded, expected),
		"unexpected msgpack encoding\nexpected: %x\nactual:   %x", expected, encoded)
}

func TestUpdateEmptyMessageFields_TM2a_TM2c_TM2f(t *testing.T) {
	messages := []*ably.Message{
		{
			ID:           "",
			ConnectionID: "",
			Timestamp:    0,
		},
		{
			ID:           "",
			ConnectionID: "",
			Timestamp:    0,
		},
		{
			ID:           "",
			ConnectionID: "",
			Timestamp:    0,
		},
	}

	presenceMessages := []*ably.PresenceMessage{
		{
			Message: ably.Message{
				ID:           "",
				ConnectionID: "",
				Timestamp:    0,
			},
			Action: 0,
		},
		{
			Message: ably.Message{
				ID:           "",
				ConnectionID: "",
				Timestamp:    0,
			},
			Action: 0,
		},
		{
			Message: ably.Message{
				ID:           "",
				ConnectionID: "",
				Timestamp:    0,
			},
			Action: 0,
		},
		{
			Message: ably.Message{
				ID:           "",
				ConnectionID: "",
				Timestamp:    0,
			},
			Action: 0,
		},
	}
	protoMsg := ably.ProtocolMessage{
		Messages:     messages,
		Presence:     presenceMessages,
		ID:           "msg-id",
		ConnectionID: "conn-id",
		Timestamp:    3453,
	}

	protoMsg.UpdateEmptyFields()

	for msgIndex, msg := range protoMsg.Messages {
		assert.Equal(t, protoMsg.ID+":"+strconv.Itoa(msgIndex), msg.ID)
		assert.Equal(t, protoMsg.ConnectionID, msg.ConnectionID)
		assert.Equal(t, protoMsg.Timestamp, msg.Timestamp)
	}

	for presenceMsgIndex, presenceMessage := range protoMsg.Presence {
		assert.Equal(t, protoMsg.ID+":"+strconv.Itoa(presenceMsgIndex), presenceMessage.ID)
		assert.Equal(t, protoMsg.ConnectionID, presenceMessage.ConnectionID)
		assert.Equal(t, protoMsg.Timestamp, presenceMessage.Timestamp)
	}
}

func TestIfFlagIsSet(t *testing.T) {
	flags := ably.FlagAttachResume
	flags.Set(ably.FlagPresence)
	flags.Set(ably.FlagPublish)
	flags.Set(ably.FlagSubscribe)
	flags.Set(ably.FlagPresenceSubscribe)

	assert.Equal(t, ably.FlagPresence, flags&ably.FlagPresence,
		"Expected %v, actual %v", ably.FlagPresence, flags&ably.FlagPresence)
	assert.Equal(t, ably.FlagPublish, flags&ably.FlagPublish,
		"Expected %v, actual %v", ably.FlagPublish, flags&ably.FlagPublish)
	assert.Equal(t, ably.FlagSubscribe, flags&ably.FlagSubscribe,
		"Expected %v, actual %v", ably.FlagSubscribe, flags&ably.FlagSubscribe)
	assert.Equal(t, ably.FlagPresenceSubscribe, flags&ably.FlagPresenceSubscribe,
		"Expected %v, actual %v", ably.FlagPresenceSubscribe, flags&ably.FlagPresenceSubscribe)
	assert.Equal(t, ably.FlagAttachResume, flags&ably.FlagAttachResume,
		"Expected %v, actual %v", ably.FlagAttachResume, flags&ably.FlagAttachResume)
	assert.NotEqual(t, ably.FlagHasBacklog, flags&ably.FlagAttachResume,
		"Shouldn't contain flag %v", ably.FlagHasBacklog)
}

func TestIfHasFlg(t *testing.T) {
	flags := ably.FlagAttachResume | ably.FlagPresence | ably.FlagPublish
	assert.True(t, flags.Has(ably.FlagAttachResume),
		"Should contain flag %v", ably.FlagAttachResume)
	assert.True(t, flags.Has(ably.FlagPresence),
		"Should contain flag %v", ably.FlagPresence)
	assert.True(t, flags.Has(ably.FlagPublish),
		"Should contain flag %v", ably.FlagPublish)
	assert.False(t, flags.Has(ably.FlagHasBacklog),
		"Shouldn't contain flag %v", ably.FlagHasBacklog)
}
