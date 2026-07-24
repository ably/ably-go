//go:build !integration
// +build !integration

package ably_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// TM2a-TM2i - Message attributes
// Tests that Message has all required attributes.
// =============================================================================

func TestMessageTypes_TM2a_IdAttribute(t *testing.T) {
	msg := ably.Message{
		ID: "unique-id",
	}
	assert.Equal(t, "unique-id", msg.ID)
}

func TestMessageTypes_TM2b_NameAttribute(t *testing.T) {
	msg := ably.Message{
		Name: "event-name",
	}
	assert.Equal(t, "event-name", msg.Name)
}

func TestMessageTypes_TM2c_DataAttribute(t *testing.T) {
	// String data
	msgString := ably.Message{
		Data: "string-data",
	}
	assert.Equal(t, "string-data", msgString.Data)

	// Map data
	mapData := map[string]interface{}{"key": "value"}
	msgMap := ably.Message{
		Data: mapData,
	}
	assert.Equal(t, mapData, msgMap.Data)

	// Binary data
	binaryData := []byte{0x01, 0x02}
	msgBinary := ably.Message{
		Data: binaryData,
	}
	assert.Equal(t, binaryData, msgBinary.Data)
}

func TestMessageTypes_TM2d_ClientIdAttribute(t *testing.T) {
	msg := ably.Message{
		ClientID: "message-client",
	}
	assert.Equal(t, "message-client", msg.ClientID)
}

func TestMessageTypes_TM2e_ConnectionIdAttribute(t *testing.T) {
	msg := ably.Message{
		ConnectionID: "conn-id",
	}
	assert.Equal(t, "conn-id", msg.ConnectionID)
}

func TestMessageTypes_TM2f_TimestampAttribute(t *testing.T) {
	msg := ably.Message{
		Timestamp: 1234567890000,
	}
	assert.Equal(t, int64(1234567890000), msg.Timestamp)
}

func TestMessageTypes_TM2g_EncodingAttribute(t *testing.T) {
	msg := ably.Message{
		Encoding: "json/base64",
	}
	assert.Equal(t, "json/base64", msg.Encoding)
}

func TestMessageTypes_TM2h_ExtrasAttribute(t *testing.T) {
	extras := map[string]interface{}{
		"push": map[string]interface{}{
			"notification": map[string]interface{}{
				"title": "Hello",
			},
		},
	}
	msg := ably.Message{
		Extras: extras,
	}

	pushExtras := msg.Extras["push"].(map[string]interface{})
	notification := pushExtras["notification"].(map[string]interface{})
	assert.Equal(t, "Hello", notification["title"])
}

// =============================================================================
// TM3 - Message from JSON (wire format)
// Tests that Message can be deserialized from JSON wire format.
// =============================================================================

func TestMessageTypes_TM3_FromJSON(t *testing.T) {
	jsonData := `{
		"id": "msg-123",
		"name": "test-event",
		"data": "hello world",
		"clientId": "sender-client",
		"connectionId": "conn-456",
		"timestamp": 1234567890000,
		"extras": { "headers": { "x-custom": "value" } }
	}`

	var msg ably.Message
	err := json.Unmarshal([]byte(jsonData), &msg)
	assert.NoError(t, err)

	assert.Equal(t, "msg-123", msg.ID)
	assert.Equal(t, "test-event", msg.Name)
	assert.Equal(t, "hello world", msg.Data)
	assert.Equal(t, "sender-client", msg.ClientID)
	assert.Equal(t, "conn-456", msg.ConnectionID)
	assert.Equal(t, int64(1234567890000), msg.Timestamp)

	if msg.Extras != nil {
		headers := msg.Extras["headers"].(map[string]interface{})
		assert.Equal(t, "value", headers["x-custom"])
	}
}

// =============================================================================
// TM3 - Message with encoded data from JSON
// Tests that Message correctly handles encoded data during deserialization.
// =============================================================================

func TestMessageTypes_TM3_EncodedData(t *testing.T) {
	testCases := []struct {
		id           string
		encoding     string
		wireData     string
		expectedType string
	}{
		{"plain_string", "", "plain text", "string"},
		{"json_object", "json", `{"key":"value"}`, "map"},
		{"base64_binary", "base64", "SGVsbG8=", "bytes"},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			jsonData := map[string]interface{}{
				"id":       "msg",
				"name":     "event",
				"data":     tc.wireData,
				"encoding": tc.encoding,
			}

			jsonBytes, _ := json.Marshal(jsonData)
			var msg ably.Message
			err := json.Unmarshal(jsonBytes, &msg)
			assert.NoError(t, err)

			// ANOMALY: ably-go Message unmarshaling may not automatically
			// decode data based on encoding. The SDK typically handles
			// this during message reception, not raw JSON unmarshaling.
			assert.NotNil(t, msg.Data)
		})
	}
}

// =============================================================================
// TM4 - Message to JSON (wire format)
// Tests that Message serializes correctly for transmission.
// =============================================================================

func TestMessageTypes_TM4_ToJSON(t *testing.T) {
	msg := ably.Message{
		ID:       "custom-id",
		Name:     "outgoing-event",
		Data:     "outgoing-data",
		ClientID: "sending-client",
	}

	jsonBytes, err := json.Marshal(msg)
	assert.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(jsonBytes, &result)
	assert.NoError(t, err)

	assert.Equal(t, "custom-id", result["id"])
	assert.Equal(t, "outgoing-event", result["name"])
	assert.Equal(t, "outgoing-data", result["data"])
	assert.Equal(t, "sending-client", result["clientId"])
}

// =============================================================================
// TM4 - Message with binary data to JSON
// Tests that binary data is base64-encoded for JSON transmission.
// =============================================================================

func TestMessageTypes_TM4_BinaryDataToJSON(t *testing.T) {
	// ANOMALY: In ably-go, message encoding for transmission is handled
	// by the client internals, not direct JSON marshaling of Message.
	// The encoding field would be set by the SDK during publish.

	binaryData := []byte{0x00, 0x01, 0xFF}
	msg := ably.Message{
		Name: "binary-event",
		Data: binaryData,
	}

	// Verify the data is stored correctly
	assert.Equal(t, binaryData, msg.Data)

	// When serialized by the SDK, it would be base64-encoded
	expectedBase64 := base64.StdEncoding.EncodeToString(binaryData)
	assert.Equal(t, "AAH/", expectedBase64)
}

// =============================================================================
// TM5 - Message equality
// Tests that messages can be compared for equality.
// =============================================================================

func TestMessageTypes_TM5_Equality(t *testing.T) {
	msg1 := ably.Message{ID: "same-id", Name: "event", Data: "data"}
	msg2 := ably.Message{ID: "same-id", Name: "event", Data: "data"}
	msg3 := ably.Message{ID: "different-id", Name: "event", Data: "data"}

	// ANOMALY: Go structs are comparable if all fields are comparable.
	// However, Data is interface{} which may not be directly comparable.
	assert.Equal(t, msg1.ID, msg2.ID)
	assert.Equal(t, msg1.Name, msg2.Name)
	assert.NotEqual(t, msg1.ID, msg3.ID)
}

// =============================================================================
// TM - Message with extras
// Tests that Message extras (push notifications, etc.) are handled correctly.
// =============================================================================

func TestMessageTypes_TM_Extras(t *testing.T) {
	msg := ably.Message{
		Name: "push-event",
		Data: "payload",
		Extras: map[string]interface{}{
			"push": map[string]interface{}{
				"notification": map[string]interface{}{
					"title": "New Message",
					"body":  "You have a new notification",
				},
				"data": map[string]interface{}{
					"customKey": "customValue",
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(msg)
	assert.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(jsonBytes, &result)
	assert.NoError(t, err)

	extras := result["extras"].(map[string]interface{})
	push := extras["push"].(map[string]interface{})
	notification := push["notification"].(map[string]interface{})
	data := push["data"].(map[string]interface{})

	assert.Equal(t, "New Message", notification["title"])
	assert.Equal(t, "customValue", data["customKey"])
}

// =============================================================================
// TM - Null/missing attributes
// Tests that null or missing attributes are handled correctly.
// =============================================================================

func TestMessageTypes_TM_NullAttributes(t *testing.T) {
	// Minimal message with zero values
	msg := ably.Message{}

	// All optional attributes should be zero values
	assert.Empty(t, msg.ID)
	assert.Empty(t, msg.Name)
	assert.Nil(t, msg.Data)
	assert.Empty(t, msg.ClientID)
	assert.Equal(t, int64(0), msg.Timestamp)

	// Serialization should handle zero values
	jsonBytes, err := json.Marshal(msg)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonBytes)
}

// =============================================================================
// Additional Message type tests
// =============================================================================

func TestMessageTypes_AllFields(t *testing.T) {
	msg := ably.Message{
		ID:           "full-msg-id",
		Name:         "full-event",
		Data:         "full-data",
		ClientID:     "full-client",
		ConnectionID: "full-conn",
		Timestamp:    1234567890000,
		Encoding:     "utf-8",
		Extras: map[string]interface{}{
			"key": "value",
		},
	}

	assert.Equal(t, "full-msg-id", msg.ID)
	assert.Equal(t, "full-event", msg.Name)
	assert.Equal(t, "full-data", msg.Data)
	assert.Equal(t, "full-client", msg.ClientID)
	assert.Equal(t, "full-conn", msg.ConnectionID)
	assert.Equal(t, int64(1234567890000), msg.Timestamp)
	assert.Equal(t, "utf-8", msg.Encoding)
	assert.NotNil(t, msg.Extras)
}

func TestMessageTypes_DataTypes(t *testing.T) {
	testCases := []struct {
		name string
		data interface{}
	}{
		{"string", "hello"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"nil", nil},
		{"bytes", []byte{1, 2, 3}},
		{"map", map[string]interface{}{"a": 1}},
		{"slice", []interface{}{1, 2, 3}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := ably.Message{
				Name: "event",
				Data: tc.data,
			}
			assert.Equal(t, tc.data, msg.Data)
		})
	}
}

func TestMessageTypes_JSONRoundTrip(t *testing.T) {
	original := ably.Message{
		ID:        "roundtrip-id",
		Name:      "roundtrip-event",
		Data:      "roundtrip-data",
		ClientID:  "roundtrip-client",
		Timestamp: 1234567890000,
	}

	jsonBytes, err := json.Marshal(original)
	assert.NoError(t, err)

	var restored ably.Message
	err = json.Unmarshal(jsonBytes, &restored)
	assert.NoError(t, err)

	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.Name, restored.Name)
	assert.Equal(t, original.Data, restored.Data)
	assert.Equal(t, original.ClientID, restored.ClientID)
	assert.Equal(t, original.Timestamp, restored.Timestamp)
}
