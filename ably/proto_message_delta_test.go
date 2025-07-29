package ably

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeltaExtras_extractDeltaExtras(t *testing.T) {
	tests := []struct {
		name     string
		extras   map[string]interface{}
		expected *DeltaExtras
	}{
		{
			name:     "nil extras",
			extras:   nil,
			expected: nil,
		},
		{
			name:     "empty extras",
			extras:   map[string]interface{}{},
			expected: nil,
		},
		{
			name: "no delta field",
			extras: map[string]interface{}{
				"other": "value",
			},
			expected: nil,
		},
		{
			name: "valid delta extras",
			extras: map[string]interface{}{
				"delta": map[string]interface{}{
					"from":   "message-id-123",
					"format": "vcdiff",
				},
			},
			expected: &DeltaExtras{
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
			expected: &DeltaExtras{
				From:   "message-id-456",
				Format: "",
			},
		},
		{
			name: "invalid delta format",
			extras: map[string]interface{}{
				"delta": "not-a-map",
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDeltaExtras(tt.extras)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.From, result.From)
				assert.Equal(t, tt.expected.Format, result.Format)
			}
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
			expectedBasePayload: nil,
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
			expectedError: "missing VCdiff decoder plugin (code 40019)",
		},
		{
			name: "vcdiff without base payload",
			message: Message{
				Data:     []byte("delta-data"),
				Encoding: "vcdiff",
			},
			context: &DecodingContext{
				VCDiffPlugin: &mockVCDiffDecoder{},
			},
			expectedError: "delta message decode failure - no base payload available (code 40018)",
		},
		{
			name: "vcdiff with mismatched message ID",
			message: Message{
				Data:     []byte("delta-data"),
				Encoding: "vcdiff",
				Extras: map[string]interface{}{
					"delta": map[string]interface{}{
						"from":   "different-id",
						"format": "vcdiff",
					},
				},
			},
			context: &DecodingContext{
				VCDiffPlugin:  &mockVCDiffDecoder{},
				BasePayload:   []byte("base-data"),
				LastMessageID: "expected-id",
			},
			expectedError: "delta message decode failure - delta reference ID \"different-id\" does not match stored ID \"expected-id\" (code 40018)",
		},
		{
			name: "successful vcdiff decoding",
			message: Message{
				ID:       "new-message-id",
				Data:     []byte("delta-data"),
				Encoding: "vcdiff",
				Extras: map[string]interface{}{
					"delta": map[string]interface{}{
						"from":   "base-message-id",
						"format": "vcdiff",
					},
				},
			},
			context: &DecodingContext{
				VCDiffPlugin:  &mockVCDiffDecoder{},
				BasePayload:   []byte("base-data"),
				LastMessageID: "base-message-id",
			},
			expectedData:        []byte("delta-data"), // Mock decoder returns delta as-is
			expectedBasePayload: []byte("delta-data"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, basePayload, err := tt.message.withDecodedDataAndContext(nil, tt.context)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedData, result.Data)
				if tt.expectedBasePayload != nil {
					assert.Equal(t, tt.expectedBasePayload, basePayload)
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

	result, basePayload, err := message.withDecodedDataAndContext(nil, context)

	assert.NoError(t, err)
	assert.Equal(t, jsonData, result.Data)
	assert.Equal(t, jsonBytes, basePayload)
	assert.Empty(t, result.Encoding) // Should be fully decoded
}

func TestMessage_withDecodedDataAndContext_VCDiffWithBase64(t *testing.T) {
	// Test message with vcdiff/base64 encoding
	message := Message{
		ID:       "msg-id",
		Data:     "ZGVsdGEtZGF0YQ==", // "delta-data" in base64
		Encoding: "vcdiff/base64",
		Extras: map[string]interface{}{
			"delta": map[string]interface{}{
				"from":   "base-id",
				"format": "vcdiff",
			},
		},
	}

	context := &DecodingContext{
		VCDiffPlugin:  &mockVCDiffDecoder{},
		BasePayload:   []byte("base-payload"),
		LastMessageID: "base-id",
	}

	result, basePayload, err := message.withDecodedDataAndContext(nil, context)

	assert.NoError(t, err)
	assert.Equal(t, []byte("delta-data"), result.Data) // Mock returns delta as-is
	assert.Equal(t, []byte("delta-data"), basePayload)
	assert.Empty(t, result.Encoding) // Should be fully decoded
}

func TestContainsErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		code     int
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			code:     40018,
			expected: false,
		},
		{
			name:     "error with correct code at end",
			err:      NewErrorInfo(40018, fmt.Errorf("vcdiff decode failed")),
			code:     40018,
			expected: true,
		},
		{
			name:     "error with different code",
			err:      NewErrorInfo(40019, fmt.Errorf("different error")),
			code:     40018,
			expected: false,
		},
		{
			name:     "error without code",
			err:      fmt.Errorf("simple error"),
			code:     40018,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsErrorCode(tt.err, tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// mockVCDiffDecoder for testing
type mockVCDiffDecoder struct{}

func (m *mockVCDiffDecoder) Decode(delta []byte, base []byte) ([]byte, error) {
	// Simple mock: just return the delta as the result
	return delta, nil
}
