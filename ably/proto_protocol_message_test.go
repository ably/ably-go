package ably_test

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"
)

// TestProtocolMessageEncodeZeroSerials tests that zero-valued serials are
// explicitly encoded into msgpack (as required by the realtime API)
func TestProtocolMessageEncodeZeroSerials(t *testing.T) {
	msg := ably.ProtocolMessage{
		ID:               "test",
		MsgSerial:        0,
		ConnectionSerial: 0,
	}
	encoded, err := ablyutil.MarshalMsgpack(msg)
	if err != nil {
		t.Fatal(err)
	}
	// expect a 3-element map with both the serial fields set to zero
	expected := []byte("\x83\xB0connectionSerial\x00\xA2id\xA4test\xA9msgSerial\x00")
	if !bytes.Equal(encoded, expected) {
		t.Fatalf("unexpected msgpack encoding\nexpected: %x\nactual:   %x", expected, encoded)
	}
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
		assertEquals(t, protoMsg.ID+":"+strconv.Itoa(msgIndex), msg.ID)
		assertEquals(t, protoMsg.ConnectionID, msg.ConnectionID)
		assertEquals(t, protoMsg.Timestamp, msg.Timestamp)
	}

	for presenceMsgIndex, presenceMessage := range protoMsg.Presence {
		assertEquals(t, protoMsg.ID+":"+strconv.Itoa(presenceMsgIndex), presenceMessage.ID)
		assertEquals(t, protoMsg.ConnectionID, presenceMessage.ConnectionID)
		assertEquals(t, protoMsg.Timestamp, presenceMessage.Timestamp)
	}
}

func TestIfFlagIsSet(t *testing.T) {
	flags := ably.FlagAttachResume
	flags.Set(ably.FlagPresence)
	flags.Set(ably.FlagPublish)
	flags.Set(ably.FlagSubscribe)
	flags.Set(ably.FlagPresenceSubscribe)

	assertEquals(t, ably.FlagAttachResume, flags&ably.FlagAttachResume)
	assertEquals(t, ably.FlagPresence, flags&ably.FlagPresence)
	assertEquals(t, ably.FlagPublish, flags&ably.FlagPublish)
	assertEquals(t, ably.FlagSubscribe, flags&ably.FlagSubscribe)
	assertEquals(t, ably.FlagPresenceSubscribe, flags&ably.FlagPresenceSubscribe)
	assertNotEquals(t, ably.FlagHasBacklog, flags&ably.FlagHasBacklog)
}

func TestIfHasFlg(t *testing.T) {
	flags := ably.FlagAttachResume | ably.FlagPresence | ably.FlagPublish
	if !flags.Has(ably.FlagAttachResume) {
		t.Fatalf("Should contain flag %v", ably.FlagAttachResume)
	}
	if !flags.Has(ably.FlagPresence) {
		t.Fatalf("Should contain flag %v", ably.FlagPresence)
	}
	if !flags.Has(ably.FlagPublish) {
		t.Fatalf("Should contain flag %v", ably.FlagPublish)
	}
	if flags.Has(ably.FlagHasBacklog) {
		t.Fatalf("Shouldn't contain flag %v", ably.FlagHasBacklog)
	}
}
