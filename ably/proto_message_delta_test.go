package ably

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeltaExtras_extractDeltaExtras(t *testing.T) {
	tests := []struct {
		name     string
		extras   map[string]interface{}
		expected DeltaExtras
	}{
		{
			name:     "nil extras",
			extras:   nil,
			expected: DeltaExtras{},
		},
		{
			name:     "empty extras",
			extras:   map[string]interface{}{},
			expected: DeltaExtras{},
		},
		{
			name: "no delta field",
			extras: map[string]interface{}{
				"other": "value",
			},
			expected: DeltaExtras{},
		},
		{
			name: "valid delta extras",
			extras: map[string]interface{}{
				"delta": map[string]interface{}{
					"from":   "message-id-123",
					"format": "vcdiff",
				},
			},
			expected: DeltaExtras{
				From:   "message-id-123",
				Format: "vcdiff",
			},
		},
		{
			name: "partial delta extras",
			extras: map[string]interface{}{
				"delta": map[string]interface{}{
					"from": "message-id-456",
				},
			},
			expected: DeltaExtras{
				From:   "message-id-456",
				Format: "",
			},
		},
		{
			name: "invalid delta format",
			extras: map[string]interface{}{
				"delta": "not-a-map",
			},
			expected: DeltaExtras{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDeltaExtras(tt.extras)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMessage_withDecodedDataAndContext(t *testing.T) {
	tests := []struct {
		name                string
		message             Message
		context             *DecodingContext
		expectedError       string
		expectedData        interface{}
		expectedBasePayload []byte
	}{
		{
			name: "message without encoding",
			message: Message{
				Data: "test data",
			},
			context:             &DecodingContext{},
			expectedData:        "test data",
			expectedBasePayload: nil, // No encoding means early return, BasePayload not set
		},
		{
			name: "base64 encoded message",
			message: Message{
				Data:     "aGVsbG8=", // "hello" in base64
				Encoding: "base64",
			},
			context:             &DecodingContext{},
			expectedData:        []byte("hello"),
			expectedBasePayload: []byte("hello"),
		},
		{
			name: "utf-8 encoded message",
			message: Message{
				Data:     []byte("hello"),
				Encoding: "utf-8",
			},
			context:             &DecodingContext{},
			expectedData:        "hello",
			expectedBasePayload: []byte("hello"),
		},
		{
			name: "json encoded message",
			message: Message{
				Data:     []byte(`{"key": "value"}`),
				Encoding: "json",
			},
			context:             &DecodingContext{},
			expectedData:        map[string]interface{}{"key": "value"},
			expectedBasePayload: []byte(`{"key": "value"}`),
		},
		{
			name: "vcdiff without plugin",
			message: Message{
				Data:     []byte("delta-data"),
				Encoding: "vcdiff",
			},
			context:       &DecodingContext{},
			expectedError: "missing VCdiff decoder plugin",
		},
		{
			name: "vcdiff with successful decoding",
			message: Message{
				Data:     []byte("delta-data"),
				Encoding: "vcdiff",
			},
			context: &DecodingContext{
				VCDiffPlugin: &mockVCDiffDecoder{},
			},
			expectedData:        []byte("delta-data"), // Mock decoder returns delta as-is
			expectedBasePayload: []byte("delta-data"), // Gets stored as new base payload
		},
		{
			name: "successful vcdiff decoding",
			message: Message{
				ID:       "new-message-id",
				Data:     []byte("delta-data"),
				Encoding: "vcdiff",
			},
			context: &DecodingContext{
				VCDiffPlugin: &mockVCDiffDecoder{},
				BasePayload:  []byte("base-data"),
			},
			expectedData:        []byte("delta-data"), // Mock decoder returns delta as-is
			expectedBasePayload: []byte("delta-data"), // Gets stored as new base payload
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of the context to avoid side effects
			var ctx *DecodingContext
			if tt.context != nil {
				contextCopy := *tt.context
				ctx = &contextCopy
			}

			result, err := tt.message.withDecodedDataAndContext(nil, ctx)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedData, result.Data)
				if tt.expectedBasePayload != nil && ctx != nil {
					assert.Equal(t, tt.expectedBasePayload, ctx.BasePayload)
				}
			}
		})
	}
}

func TestMessage_withDecodedDataAndContext_MultipleEncodings(t *testing.T) {
	// Test message with multiple encodings: json/base64
	jsonData := map[string]interface{}{"key": "value"}
	jsonBytes, _ := json.Marshal(jsonData)

	message := Message{
		Data:     "eyJrZXkiOiJ2YWx1ZSJ9", // base64 encoded JSON
		Encoding: "json/base64",
	}

	context := &DecodingContext{}

	result, err := message.withDecodedDataAndContext(nil, context)

	assert.NoError(t, err)
	assert.Equal(t, jsonData, result.Data)
	assert.Equal(t, jsonBytes, context.BasePayload)
	assert.Empty(t, result.Encoding) // Should be fully decoded
}

func TestMessage_withDecodedDataAndContext_VCDiffWithBase64(t *testing.T) {
	// Test message with vcdiff/base64 encoding
	message := Message{
		ID:       "msg-id",
		Data:     "ZGVsdGEtZGF0YQ==", // "delta-data" in base64
		Encoding: "vcdiff/base64",
	}

	context := &DecodingContext{
		VCDiffPlugin: &mockVCDiffDecoder{},
		BasePayload:  []byte("base-payload"),
	}

	result, err := message.withDecodedDataAndContext(nil, context)

	assert.NoError(t, err)
	assert.Equal(t, []byte("delta-data"), result.Data) // Mock returns delta as-is
	assert.Equal(t, []byte("delta-data"), context.BasePayload)
	assert.Empty(t, result.Encoding) // Should be fully decoded
}

// mockVCDiffDecoder for testing
type mockVCDiffDecoder struct{}

func (m *mockVCDiffDecoder) Decode(delta []byte, base []byte) ([]byte, error) {
	// Simple mock: just return the delta as the result
	return delta, nil
}
