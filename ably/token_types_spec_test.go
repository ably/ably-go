//go:build !integration
// +build !integration

package ably_test

import (
	"encoding/json"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// TD1-TD5 - TokenDetails structure
// Tests that TokenDetails has all required attributes.
// =============================================================================

func TestTokenTypes_TD1_TokenAttribute(t *testing.T) {
	tokenDetails := ably.TokenDetails{
		Token:   "test-token",
		Expires: 1234567890000,
	}
	assert.Equal(t, "test-token", tokenDetails.Token)
}

func TestTokenTypes_TD2_ExpiresAttribute(t *testing.T) {
	tokenDetails := ably.TokenDetails{
		Token:   "test-token",
		Expires: 1234567890000,
	}
	assert.Equal(t, int64(1234567890000), tokenDetails.Expires)
}

func TestTokenTypes_TD3_IssuedAttribute(t *testing.T) {
	tokenDetails := ably.TokenDetails{
		Token:   "test-token",
		Expires: 1234567890000,
		Issued:  1234567800000,
	}
	assert.Equal(t, int64(1234567800000), tokenDetails.Issued)
}

func TestTokenTypes_TD4_CapabilityAttribute(t *testing.T) {
	tokenDetails := ably.TokenDetails{
		Token:      "test-token",
		Expires:    1234567890000,
		Capability: `{"*":["*"]}`,
	}
	assert.Equal(t, `{"*":["*"]}`, tokenDetails.Capability)
}

func TestTokenTypes_TD5_ClientIdAttribute(t *testing.T) {
	tokenDetails := ably.TokenDetails{
		Token:    "test-token",
		Expires:  1234567890000,
		ClientID: "my-client",
	}
	assert.Equal(t, "my-client", tokenDetails.ClientID)
}

// =============================================================================
// TD - TokenDetails from JSON
// Tests that TokenDetails can be deserialized from JSON response.
// =============================================================================

func TestTokenTypes_TD_FromJSON(t *testing.T) {
	jsonData := `{
		"token": "deserialized-token",
		"expires": 1234567890000,
		"issued": 1234567800000,
		"capability": "{\"channel-1\":[\"publish\"]}",
		"clientId": "json-client",
		"keyName": "appId.keyId"
	}`

	var tokenDetails ably.TokenDetails
	err := json.Unmarshal([]byte(jsonData), &tokenDetails)
	assert.NoError(t, err)

	assert.Equal(t, "deserialized-token", tokenDetails.Token)
	assert.Equal(t, int64(1234567890000), tokenDetails.Expires)
	assert.Equal(t, int64(1234567800000), tokenDetails.Issued)
	assert.Equal(t, `{"channel-1":["publish"]}`, tokenDetails.Capability)
	assert.Equal(t, "json-client", tokenDetails.ClientID)
}

func TestTokenTypes_TD_AllAttributes(t *testing.T) {
	tokenDetails := ably.TokenDetails{
		Token:      "full-token",
		Expires:    1234567890000,
		Issued:     1234567800000,
		Capability: `{"*":["*"]}`,
		ClientID:   "full-client",
	}

	assert.Equal(t, "full-token", tokenDetails.Token)
	assert.Equal(t, int64(1234567890000), tokenDetails.Expires)
	assert.Equal(t, int64(1234567800000), tokenDetails.Issued)
	assert.Equal(t, `{"*":["*"]}`, tokenDetails.Capability)
	assert.Equal(t, "full-client", tokenDetails.ClientID)
}

// =============================================================================
// TK1-TK6 - TokenParams structure
// Tests that TokenParams has all required attributes.
// =============================================================================

func TestTokenTypes_TK1_TTLAttribute(t *testing.T) {
	params := ably.TokenParams{
		TTL: 3600000,
	}
	assert.Equal(t, int64(3600000), params.TTL)
}

func TestTokenTypes_TK2_CapabilityAttribute(t *testing.T) {
	params := ably.TokenParams{
		Capability: `{"*":["subscribe"]}`,
	}
	assert.Equal(t, `{"*":["subscribe"]}`, params.Capability)
}

func TestTokenTypes_TK3_ClientIdAttribute(t *testing.T) {
	params := ably.TokenParams{
		ClientID: "param-client",
	}
	assert.Equal(t, "param-client", params.ClientID)
}

func TestTokenTypes_TK4_TimestampAttribute(t *testing.T) {
	params := ably.TokenParams{
		Timestamp: 1234567890000,
	}
	assert.Equal(t, int64(1234567890000), params.Timestamp)
}

func TestTokenTypes_TK5_NonceAttribute(t *testing.T) {
	// ANOMALY: ably-go TokenParams does NOT have a Nonce field.
	// Nonce is only present in TokenRequest, not TokenParams.
	// Per the spec, TK5 says TokenParams should have nonce, but ably-go
	// moves it to TokenRequest only.
	t.Skip("TK5 - ably-go TokenParams does not have Nonce field; it's only in TokenRequest")
}

func TestTokenTypes_TK6_AllAttributes(t *testing.T) {
	// ANOMALY: ably-go TokenParams does NOT have Nonce (see TK5)
	params := ably.TokenParams{
		TTL:        7200000,
		Capability: `{"*":["*"]}`,
		ClientID:   "full-client",
		Timestamp:  1234567890000,
	}

	assert.Equal(t, int64(7200000), params.TTL)
	assert.Equal(t, `{"*":["*"]}`, params.Capability)
	assert.Equal(t, "full-client", params.ClientID)
	assert.Equal(t, int64(1234567890000), params.Timestamp)
}

// =============================================================================
// TK - TokenParams to query string
// Tests that TokenParams are correctly converted to query parameters.
// ANOMALY: ably-go TokenParams doesn't have a direct toQueryParams method.
// Conversion is handled internally during token requests.
// =============================================================================

func TestTokenTypes_TK_ToQueryParams(t *testing.T) {
	params := ably.TokenParams{
		TTL:        3600000,
		ClientID:   "query-client",
		Capability: `{"ch":["pub"]}`,
	}

	// ANOMALY: ably-go doesn't expose a direct toQueryParams method.
	// We verify the fields are set correctly for internal use.
	assert.Equal(t, int64(3600000), params.TTL)
	assert.Equal(t, "query-client", params.ClientID)
	assert.Equal(t, `{"ch":["pub"]}`, params.Capability)
}

// =============================================================================
// TE1-TE6 - TokenRequest structure
// Tests that TokenRequest has all required attributes.
// =============================================================================

func TestTokenTypes_TE1_KeyNameAttribute(t *testing.T) {
	request := ably.TokenRequest{
		TokenParams: ably.TokenParams{
			Timestamp: 1234567890000,
		},
		KeyName: "appId.keyId",
		Nonce:   "nonce-1",
	}
	assert.Equal(t, "appId.keyId", request.KeyName)
}

func TestTokenTypes_TE2_TTLAttribute(t *testing.T) {
	request := ably.TokenRequest{
		TokenParams: ably.TokenParams{
			TTL:       3600000,
			Timestamp: 1234567890000,
		},
		KeyName: "appId.keyId",
		Nonce:   "nonce-2",
	}
	assert.Equal(t, int64(3600000), request.TTL)
}

func TestTokenTypes_TE3_CapabilityAttribute(t *testing.T) {
	request := ably.TokenRequest{
		TokenParams: ably.TokenParams{
			Capability: `{"*":["*"]}`,
			Timestamp:  1234567890000,
		},
		KeyName: "appId.keyId",
		Nonce:   "nonce-3",
	}
	assert.Equal(t, `{"*":["*"]}`, request.Capability)
}

func TestTokenTypes_TE4_ClientIdAttribute(t *testing.T) {
	request := ably.TokenRequest{
		TokenParams: ably.TokenParams{
			ClientID:  "request-client",
			Timestamp: 1234567890000,
		},
		KeyName: "appId.keyId",
		Nonce:   "nonce-4",
	}
	assert.Equal(t, "request-client", request.ClientID)
}

func TestTokenTypes_TE5_TimestampAttribute(t *testing.T) {
	request := ably.TokenRequest{
		TokenParams: ably.TokenParams{
			Timestamp: 1234567890000,
		},
		KeyName: "appId.keyId",
		Nonce:   "nonce-5",
	}
	assert.Equal(t, int64(1234567890000), request.Timestamp)
}

func TestTokenTypes_TE6_NonceAttribute(t *testing.T) {
	request := ably.TokenRequest{
		TokenParams: ably.TokenParams{
			Timestamp: 1234567890000,
		},
		KeyName: "appId.keyId",
		Nonce:   "unique-nonce",
	}
	assert.Equal(t, "unique-nonce", request.Nonce)
}

// =============================================================================
// TE - TokenRequest with mac (signature)
// Tests that TokenRequest includes the mac signature.
// =============================================================================

func TestTokenTypes_TE_MacAttribute(t *testing.T) {
	request := ably.TokenRequest{
		TokenParams: ably.TokenParams{
			Timestamp: 1234567890000,
		},
		KeyName: "appId.keyId",
		Nonce:   "nonce-value",
		MAC:     "signature-base64",
	}
	assert.Equal(t, "signature-base64", request.MAC)
}

// =============================================================================
// TE - TokenRequest to JSON
// Tests that TokenRequest serializes correctly for transmission.
// =============================================================================

func TestTokenTypes_TE_ToJSON(t *testing.T) {
	request := ably.TokenRequest{
		TokenParams: ably.TokenParams{
			TTL:        3600000,
			Capability: `{"*":["*"]}`,
			ClientID:   "json-client",
			Timestamp:  1234567890000,
		},
		KeyName: "appId.keyId",
		Nonce:   "json-nonce",
		MAC:     "json-mac",
	}

	jsonBytes, err := json.Marshal(request)
	assert.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(jsonBytes, &result)
	assert.NoError(t, err)

	assert.Equal(t, "appId.keyId", result["keyName"])
	assert.Equal(t, float64(3600000), result["ttl"])
	assert.Equal(t, `{"*":["*"]}`, result["capability"])
	assert.Equal(t, "json-client", result["clientId"])
	assert.Equal(t, float64(1234567890000), result["timestamp"])
	assert.Equal(t, "json-nonce", result["nonce"])
	assert.Equal(t, "json-mac", result["mac"])
}

// =============================================================================
// TE - TokenRequest from JSON
// Tests that TokenRequest can be deserialized from JSON.
// =============================================================================

func TestTokenTypes_TE_FromJSON(t *testing.T) {
	jsonData := `{
		"keyName": "appId.keyId",
		"ttl": 7200000,
		"capability": "{\"ch\":[\"sub\"]}",
		"clientId": "from-json-client",
		"timestamp": 1234567899999,
		"nonce": "from-json-nonce",
		"mac": "from-json-mac"
	}`

	var request ably.TokenRequest
	err := json.Unmarshal([]byte(jsonData), &request)
	assert.NoError(t, err)

	assert.Equal(t, "appId.keyId", request.KeyName)
	assert.Equal(t, int64(7200000), request.TTL)
	assert.Equal(t, `{"ch":["sub"]}`, request.Capability)
	assert.Equal(t, "from-json-client", request.ClientID)
	assert.Equal(t, int64(1234567899999), request.Timestamp)
	assert.Equal(t, "from-json-nonce", request.Nonce)
	assert.Equal(t, "from-json-mac", request.MAC)
}

// =============================================================================
// Additional token type tests
// =============================================================================

func TestTokenTypes_TokenDetailsJSONRoundTrip(t *testing.T) {
	original := ably.TokenDetails{
		Token:      "roundtrip-token",
		Expires:    1234567890000,
		Issued:     1234567800000,
		Capability: `{"*":["*"]}`,
		ClientID:   "roundtrip-client",
	}

	jsonBytes, err := json.Marshal(original)
	assert.NoError(t, err)

	var restored ably.TokenDetails
	err = json.Unmarshal(jsonBytes, &restored)
	assert.NoError(t, err)

	assert.Equal(t, original.Token, restored.Token)
	assert.Equal(t, original.Expires, restored.Expires)
	assert.Equal(t, original.Issued, restored.Issued)
	assert.Equal(t, original.Capability, restored.Capability)
	assert.Equal(t, original.ClientID, restored.ClientID)
}

func TestTokenTypes_TokenRequestJSONRoundTrip(t *testing.T) {
	original := ably.TokenRequest{
		TokenParams: ably.TokenParams{
			TTL:        3600000,
			Capability: `{"*":["*"]}`,
			ClientID:   "roundtrip-client",
			Timestamp:  1234567890000,
		},
		KeyName: "appId.keyId",
		Nonce:   "roundtrip-nonce",
		MAC:     "roundtrip-mac",
	}

	jsonBytes, err := json.Marshal(original)
	assert.NoError(t, err)

	var restored ably.TokenRequest
	err = json.Unmarshal(jsonBytes, &restored)
	assert.NoError(t, err)

	assert.Equal(t, original.KeyName, restored.KeyName)
	assert.Equal(t, original.TTL, restored.TTL)
	assert.Equal(t, original.Capability, restored.Capability)
	assert.Equal(t, original.ClientID, restored.ClientID)
	assert.Equal(t, original.Timestamp, restored.Timestamp)
	assert.Equal(t, original.Nonce, restored.Nonce)
	assert.Equal(t, original.MAC, restored.MAC)
}

func TestTokenTypes_TokenParamsDefaults(t *testing.T) {
	// Empty TokenParams should have zero values
	params := ably.TokenParams{}

	assert.Equal(t, int64(0), params.TTL)
	assert.Empty(t, params.Capability)
	assert.Empty(t, params.ClientID)
	assert.Equal(t, int64(0), params.Timestamp)
	// ANOMALY: ably-go TokenParams does NOT have Nonce field
}

func TestTokenTypes_TokenStringImplementsTokener(t *testing.T) {
	// TokenString should implement Tokener interface
	var tokener ably.Tokener = ably.TokenString("test-token")
	assert.NotNil(t, tokener)
}

func TestTokenTypes_TokenDetailsImplementsTokener(t *testing.T) {
	// TokenDetails should implement Tokener interface
	tokenDetails := &ably.TokenDetails{
		Token:   "test-token",
		Expires: 1234567890000,
	}
	var tokener ably.Tokener = tokenDetails
	assert.NotNil(t, tokener)
}

func TestTokenTypes_TokenRequestImplementsTokener(t *testing.T) {
	// TokenRequest should implement Tokener interface
	tokenRequest := &ably.TokenRequest{
		TokenParams: ably.TokenParams{
			Timestamp: 1234567890000,
		},
		KeyName: "appId.keyId",
		Nonce:   "nonce",
		MAC:     "mac",
	}
	var tokener ably.Tokener = tokenRequest
	assert.NotNil(t, tokener)
}

func TestTokenTypes_CapabilityJSON(t *testing.T) {
	// Test various capability formats
	testCases := []struct {
		name       string
		capability string
	}{
		{"wildcard_all", `{"*":["*"]}`},
		{"channel_specific", `{"channel-1":["publish","subscribe"]}`},
		{"multiple_channels", `{"ch1":["publish"],"ch2":["subscribe"]}`},
		{"presence", `{"channel":["presence"]}`},
		{"history", `{"channel":["history"]}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := ably.TokenParams{
				Capability: tc.capability,
			}
			assert.Equal(t, tc.capability, params.Capability)

			// Verify it's valid JSON
			var parsed map[string]interface{}
			err := json.Unmarshal([]byte(tc.capability), &parsed)
			assert.NoError(t, err)
		})
	}
}
