package ably_test

import (
	"bytes"
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

func TestIfFlagIsSet(t *testing.T) {
	flags := ably.FlagAttachResume
	flags.Set(ably.FlagPresence)
	flags.Set(ably.FlagPublish)
	flags.Set(ably.FlagSubscribe)
	flags.Set(ably.FlagPresenceSubscribe)

	if expected, actual := ably.FlagPresence, flags&ably.FlagPresence; expected != actual {
		t.Fatalf("Expected %v, actual %v", expected, actual)
	}
	if expected, actual := ably.FlagPublish, flags&ably.FlagPublish; expected != actual {
		t.Fatalf("Expected %v, actual %v", expected, actual)
	}
	if expected, actual := ably.FlagSubscribe, flags&ably.FlagSubscribe; expected != actual {
		t.Fatalf("Expected %v, actual %v", expected, actual)
	}
	if expected, actual := ably.FlagPresenceSubscribe, flags&ably.FlagPresenceSubscribe; expected != actual {
		t.Fatalf("Expected %v, actual %v", expected, actual)
	}
	if expected, actual := ably.FlagAttachResume, flags&ably.FlagAttachResume; expected != actual {
		t.Fatalf("Expected %v, actual %v", expected, actual)
	}
	if expected, actual := ably.FlagHasBacklog, flags&ably.FlagAttachResume; expected == actual {
		t.Fatalf("Shouldn't contain flag %v", expected)
	}
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
