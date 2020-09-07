package proto_test

import (
	"bytes"
	"testing"

	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ably/proto"
)

// TestProtocolMessageEncodeZeroSerials tests that zero-valued serials are
// explicitly encoded into msgpack (as required by the realtime API)
func TestProtocolMessageEncodeZeroSerials(t *testing.T) {
	msg := proto.ProtocolMessage{
		ID:               "test",
		MsgSerial:        0,
		ConnectionSerial: 0,
	}
	encoded, err := ablyutil.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	// expect a 3-element map with both the serial fields set to zero
	expected := []byte("\x83\xB0connectionSerial\x00\xA2id\xA4test\xA9msgSerial\x00")
	if !bytes.Equal(encoded, expected) {
		t.Fatalf("unexpected msgpack encoding\nexpected: %x\nactual:   %x", expected, encoded)
	}
}
