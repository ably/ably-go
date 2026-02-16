package ably

import (
	"encoding/json"
	"testing"

	"github.com/ugorji/go/codec"
)

func TestMessageAction_JSON_Encoding(t *testing.T) {
	tests := []struct {
		action   MessageAction
		expected string
	}{
		{MessageActionCreate, "0"},
		{MessageActionUpdate, "1"},
		{MessageActionDelete, "2"},
		{MessageActionAppend, "5"},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			data, err := json.Marshal(tt.action)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(data))
			}
		})
	}
}

func TestMessageAction_JSON_Decoding(t *testing.T) {
	tests := []struct {
		input    string
		expected MessageAction
	}{
		{"0", MessageActionCreate},
		{"1", MessageActionUpdate},
		{"2", MessageActionDelete},
		{"5", MessageActionAppend},
		{"999", MessageActionCreate}, // Unknown values default to Create
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var action MessageAction
			err := json.Unmarshal([]byte(tt.input), &action)
			if err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}
			if action != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, action)
			}
		})
	}
}

func TestMessageAction_Codec_Encoding(t *testing.T) {
	tests := []struct {
		action MessageAction
		num    int
	}{
		{MessageActionCreate, 0},
		{MessageActionUpdate, 1},
		{MessageActionDelete, 2},
		{MessageActionAppend, 5},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			var buf []byte
			enc := codec.NewEncoderBytes(&buf, &codec.MsgpackHandle{})
			tt.action.CodecEncodeSelf(enc)

			// Decode to verify
			var result int
			dec := codec.NewDecoderBytes(buf, &codec.MsgpackHandle{})
			dec.MustDecode(&result)

			if result != tt.num {
				t.Errorf("Expected %d, got %d", tt.num, result)
			}
		})
	}
}

func TestMessageAction_Codec_Decoding(t *testing.T) {
	tests := []struct {
		num      int
		expected MessageAction
	}{
		{0, MessageActionCreate},
		{1, MessageActionUpdate},
		{2, MessageActionDelete},
		{5, MessageActionAppend},
		{999, MessageActionCreate}, // Unknown values default to Create
	}

	for _, tt := range tests {
		t.Run(string(tt.expected), func(t *testing.T) {
			var buf []byte
			enc := codec.NewEncoderBytes(&buf, &codec.MsgpackHandle{})
			enc.MustEncode(tt.num)

			var action MessageAction
			dec := codec.NewDecoderBytes(buf, &codec.MsgpackHandle{})
			action.CodecDecodeSelf(dec)

			if action != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, action)
			}
		})
	}
}

func TestMessageVersion_Serialization(t *testing.T) {
	version := &MessageVersion{
		Serial:      "abc123",
		Timestamp:   1234567890,
		ClientID:    "client1",
		Description: "Test update",
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(version)
	if err != nil {
		t.Fatalf("Failed to marshal MessageVersion: %v", err)
	}

	var decoded MessageVersion
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal MessageVersion: %v", err)
	}

	if decoded.Serial != version.Serial {
		t.Errorf("Serial: expected %s, got %s", version.Serial, decoded.Serial)
	}
	if decoded.Timestamp != version.Timestamp {
		t.Errorf("Timestamp: expected %d, got %d", version.Timestamp, decoded.Timestamp)
	}
	if decoded.ClientID != version.ClientID {
		t.Errorf("ClientID: expected %s, got %s", version.ClientID, decoded.ClientID)
	}
	if decoded.Description != version.Description {
		t.Errorf("Description: expected %s, got %s", version.Description, decoded.Description)
	}
	if len(decoded.Metadata) != len(version.Metadata) {
		t.Errorf("Metadata length: expected %d, got %d", len(version.Metadata), len(decoded.Metadata))
	}
}

func TestMessage_NewFields_Serialization(t *testing.T) {
	msg := &Message{
		ID:     "msg123",
		Data:   "test data",
		Serial: "serial123",
		Action: MessageActionUpdate,
		Version: &MessageVersion{
			Serial:      "version_serial",
			ClientID:    "client1",
			Description: "Updated message",
		},
	}

	// Test JSON serialization
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal Message: %v", err)
	}

	var decoded Message
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal Message: %v", err)
	}

	if decoded.Serial != msg.Serial {
		t.Errorf("Serial: expected %s, got %s", msg.Serial, decoded.Serial)
	}
	if decoded.Action != msg.Action {
		t.Errorf("Action: expected %s, got %s", msg.Action, decoded.Action)
	}
	if decoded.Version == nil {
		t.Fatal("Version should not be nil")
	}
	if decoded.Version.Serial != msg.Version.Serial {
		t.Errorf("Version.Serial: expected %s, got %s", msg.Version.Serial, decoded.Version.Serial)
	}
}

func TestValidateMessageSerial(t *testing.T) {
	t.Run("nil message", func(t *testing.T) {
		err := validateMessageSerial(nil)
		if err == nil {
			t.Fatal("Expected error for nil message")
		}
		if code(err) != 40003 {
			t.Errorf("Expected error code 40003, got %d", code(err))
		}
	})

	t.Run("empty serial", func(t *testing.T) {
		msg := &Message{Data: "test"}
		err := validateMessageSerial(msg)
		if err == nil {
			t.Fatal("Expected error for message without serial")
		}
		if code(err) != 40003 {
			t.Errorf("Expected error code 40003, got %d", code(err))
		}
		// Verify exact error message matches TypeScript
		expectedMsg := "This message lacks a serial and cannot be updated. Make sure you have enabled \"Message annotations, updates, and deletes\" in channel settings on your dashboard."
		if err.(*ErrorInfo).Message() != expectedMsg {
			t.Errorf("Error message mismatch.\nExpected: %s\nGot: %s", expectedMsg, err.(*ErrorInfo).Message())
		}
	})

	t.Run("valid serial", func(t *testing.T) {
		msg := &Message{Data: "test", Serial: "abc123"}
		err := validateMessageSerial(msg)
		if err != nil {
			t.Errorf("Expected no error for valid message, got %v", err)
		}
	})
}
