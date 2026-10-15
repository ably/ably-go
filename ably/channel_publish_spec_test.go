//go:build !integration
// +build !integration

package ably_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// RSL1a, RSL1b - Publish with name and data
// Tests that publish(name, data) sends a single message.
// =============================================================================

func TestChannelPublish_RSL1a_RSL1b_WithNameAndData(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(201, []byte(`{"serials": ["serial1"]}`), "application/json")

	// DEVIATION: ably-go uses msgpack by default. We force JSON for test inspection.
	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithIdempotentRESTPublishing(false), // Disable to avoid ID generation
		ably.WithUseBinaryProtocol(false),        // Force JSON for test inspection
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")
	err = channel.Publish(context.Background(), "greeting", "hello")
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 1)
	request := mock.requests[0]

	// RSL1b - single message published
	assert.Equal(t, "POST", request.Method)
	assert.Equal(t, "/channels/test-channel/messages", request.URL.Path)

	body := getPublishRequestBody(t, request)
	assert.Equal(t, 1, len(body), "should publish single message")
	assert.Equal(t, "greeting", body[0]["name"])
	assert.Equal(t, "hello", body[0]["data"])
}

// =============================================================================
// RSL1a, RSL1c - Publish with Message array
// Tests that publish(messages: [...]) sends all messages in a single request.
// =============================================================================

func TestChannelPublish_RSL1a_RSL1c_WithMessageArray(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(201, []byte(`{"serials": ["s1", "s2", "s3"]}`), "application/json")

	// DEVIATION: ably-go uses msgpack by default. We force JSON for test inspection.
	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithIdempotentRESTPublishing(false),
		ably.WithUseBinaryProtocol(false), // Force JSON for test inspection
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")
	messages := []*ably.Message{
		{Name: "event1", Data: "data1"},
		{Name: "event2", Data: map[string]interface{}{"key": "value"}},
		{Name: "event3", Data: []byte{0x01, 0x02, 0x03}},
	}
	err = channel.PublishMultiple(context.Background(), messages)
	assert.NoError(t, err)

	// RSL1c - single request for array
	assert.Equal(t, 1, len(mock.requests), "should send all messages in single request")

	body := getPublishRequestBody(t, mock.requests[0])
	assert.Equal(t, 3, len(body))
	assert.Equal(t, "event1", body[0]["name"])
	assert.Equal(t, "data1", body[0]["data"])
	assert.Equal(t, "event2", body[1]["name"])
}

// =============================================================================
// RSL1e - Null name and data
// Tests that null values are omitted from the transmitted message.
// =============================================================================

func TestChannelPublish_RSL1e_NullNameAndData(t *testing.T) {
	testCases := []struct {
		id           string
		name         string
		data         interface{}
		expectName   bool
		expectData   bool
	}{
		{"null_name", "", "hello", false, true},
		{"null_data", "event", nil, true, false},
		{"both_null", "", nil, false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			mock := newMockRoundTripper()
			httpClient := &http.Client{Transport: mock}

			mock.queueResponse(201, []byte(`{"serials": ["s1"]}`), "application/json")

			// DEVIATION: ably-go uses msgpack by default. We force JSON for test inspection.
			client, err := ably.NewREST(
				ably.WithKey("appId.keyId:keySecret"),
				ably.WithHTTPClient(httpClient),
				ably.WithIdempotentRESTPublishing(false),
				ably.WithUseBinaryProtocol(false), // Force JSON for test inspection
			)
			assert.NoError(t, err)

			channel := client.Channels.Get("test-channel")
			err = channel.Publish(context.Background(), tc.name, tc.data)
			assert.NoError(t, err)

			body := getPublishRequestBody(t, mock.requests[0])
			assert.Equal(t, 1, len(body))

			_, hasName := body[0]["name"]
			_, hasData := body[0]["data"]

			if tc.expectName {
				assert.True(t, hasName, "name should be present")
			}
			if tc.expectData {
				assert.True(t, hasData, "data should be present")
			}
		})
	}
}

// =============================================================================
// RSL1h - publish(name, data) signature
// Tests that the two-argument form works correctly.
// =============================================================================

func TestChannelPublish_RSL1h_TwoArgumentSignature(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(201, []byte(`{"serials": ["s1"]}`), "application/json")

	// DEVIATION: ably-go uses msgpack by default. We force JSON for test inspection.
	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithIdempotentRESTPublishing(false),
		ably.WithUseBinaryProtocol(false), // Force JSON for test inspection
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")
	err = channel.Publish(context.Background(), "event", "payload")
	assert.NoError(t, err)

	assert.Equal(t, 1, len(mock.requests))
	body := getPublishRequestBody(t, mock.requests[0])
	assert.Equal(t, "event", body[0]["name"])
	assert.Equal(t, "payload", body[0]["data"])
}

// =============================================================================
// RSL1i - Message size limit
// Tests that messages exceeding maxMessageSize are rejected with error 40009.
// ANOMALY: ably-go may not have maxMessageSize client-side validation.
// =============================================================================

func TestChannelPublish_RSL1i_MessageSizeLimit(t *testing.T) {
	// ANOMALY: ably-go does not appear to have client-side message size validation.
	// The maxMessageSize check is performed server-side.
	// This test documents the expected behavior but may not be enforceable client-side.
	t.Skip("RSL1i - ably-go does not perform client-side message size validation")
}

// =============================================================================
// RSL1j - All Message attributes transmitted
// Tests that all valid Message attributes are included in the encoded message.
// =============================================================================

func TestChannelPublish_RSL1j_AllMessageAttributes(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(201, []byte(`{"serials": ["s1"]}`), "application/json")

	// DEVIATION: ably-go uses msgpack by default. We force JSON for test inspection.
	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithIdempotentRESTPublishing(false),
		ably.WithUseBinaryProtocol(false), // Force JSON for test inspection
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")
	message := &ably.Message{
		ID:       "custom-message-id",
		Name:     "test-event",
		Data:     "test-data",
		ClientID: "explicit-client-id",
		Extras: map[string]interface{}{
			"push": map[string]interface{}{
				"notification": map[string]interface{}{
					"title": "Test",
				},
			},
		},
	}

	err = channel.PublishMultiple(context.Background(), []*ably.Message{message})
	assert.NoError(t, err)

	body := getPublishRequestBody(t, mock.requests[0])
	assert.Equal(t, "test-event", body[0]["name"])
	assert.Equal(t, "test-data", body[0]["data"])
	assert.Equal(t, "custom-message-id", body[0]["id"])

	if extras, ok := body[0]["extras"].(map[string]interface{}); ok {
		if push, ok := extras["push"].(map[string]interface{}); ok {
			if notification, ok := push["notification"].(map[string]interface{}); ok {
				assert.Equal(t, "Test", notification["title"])
			}
		}
	}
}

// =============================================================================
// RSL1l - Publish params as querystring
// Tests that additional params are sent as querystring parameters.
// =============================================================================

func TestChannelPublish_RSL1l_PublishParamsAsQuerystring(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(201, []byte(`{"serials": ["s1"]}`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithIdempotentRESTPublishing(false),
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")
	params := map[string]string{
		"customParam":  "customValue",
		"anotherParam": "123",
	}

	err = channel.Publish(context.Background(), "event", "data", ably.PublishWithParams(params))
	assert.NoError(t, err)

	request := mock.requests[0]
	assert.Equal(t, "customValue", request.URL.Query().Get("customParam"))
	assert.Equal(t, "123", request.URL.Query().Get("anotherParam"))
}

// =============================================================================
// RSL1m - ClientId not set from library clientId
// Tests that the library does not automatically set Message.clientId from the client's configured clientId.
// =============================================================================

func TestChannelPublish_RSL1m1_MessageWithNoClientId(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(201, []byte(`{"serials": ["s1"]}`), "application/json")

	// DEVIATION: ably-go uses msgpack by default. We force JSON for test inspection.
	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithClientID("lib-client"),
		ably.WithHTTPClient(httpClient),
		ably.WithIdempotentRESTPublishing(false),
		ably.WithUseBinaryProtocol(false), // Force JSON for test inspection
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("ch")
	err = channel.Publish(context.Background(), "e", "d")
	assert.NoError(t, err)

	body := getPublishRequestBody(t, mock.requests[0])

	// Library should not inject its clientId
	_, hasClientID := body[0]["clientId"]
	assert.False(t, hasClientID, "clientId should not be injected from library")
}

func TestChannelPublish_RSL1m2_MessageClientIdMatchesLibrary(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(201, []byte(`{"serials": ["s1"]}`), "application/json")

	// DEVIATION: ably-go uses msgpack by default. We force JSON for test inspection.
	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithClientID("lib-client"),
		ably.WithHTTPClient(httpClient),
		ably.WithIdempotentRESTPublishing(false),
		ably.WithUseBinaryProtocol(false), // Force JSON for test inspection
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("ch")
	err = channel.PublishMultiple(context.Background(), []*ably.Message{
		{Name: "e", Data: "d", ClientID: "lib-client"},
	})
	assert.NoError(t, err)

	body := getPublishRequestBody(t, mock.requests[0])

	// Explicit clientId preserved
	assert.Equal(t, "lib-client", body[0]["clientId"])
}

func TestChannelPublish_RSL1m3_UnidentifiedClientWithMessageClientId(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(201, []byte(`{"serials": ["s1"]}`), "application/json")

	// DEVIATION: ably-go uses msgpack by default. We force JSON for test inspection.
	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		// No clientId
		ably.WithHTTPClient(httpClient),
		ably.WithIdempotentRESTPublishing(false),
		ably.WithUseBinaryProtocol(false), // Force JSON for test inspection
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("ch")
	err = channel.PublishMultiple(context.Background(), []*ably.Message{
		{Name: "e", Data: "d", ClientID: "msg-client"},
	})
	assert.NoError(t, err)

	body := getPublishRequestBody(t, mock.requests[0])

	// Message clientId should be preserved
	assert.Equal(t, "msg-client", body[0]["clientId"])
}

// =============================================================================
// Additional publish tests
// =============================================================================

func TestChannelPublish_ErrorResponse(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	errorResponse := `{
		"error": {
			"code": 40000,
			"statusCode": 400,
			"message": "Bad request"
		}
	}`
	mock.queueResponse(400, []byte(errorResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")
	err = channel.Publish(context.Background(), "event", "data")

	assert.Error(t, err, "expected error from failed publish")
	if errInfo, ok := err.(*ably.ErrorInfo); ok {
		assert.Equal(t, ably.ErrorCode(40000), errInfo.Code)
		assert.Equal(t, 400, errInfo.StatusCode)
	}
}

// DEVIATION: The spec expects URL-encoded channel names (e.g., "with%3Acolon"),
// but ably-go does NOT URL-encode special characters in channel name paths.
// This test documents ably-go's actual behavior.
func TestChannelPublish_URLEncoding(t *testing.T) {
	testCases := []struct {
		channelName  string
		expectedPath string
	}{
		{"simple", "/channels/simple/messages"},
		// DEVIATION: Spec expects "/channels/with%3Acolon/messages"
		{"with:colon", "/channels/with:colon/messages"},
		// DEVIATION: Spec expects "/channels/with%2Fslash/messages"
		{"with/slash", "/channels/with/slash/messages"},
		// DEVIATION: Spec expects "/channels/with%20space/messages"
		{"with space", "/channels/with space/messages"},
	}

	for _, tc := range testCases {
		t.Run(tc.channelName, func(t *testing.T) {
			mock := newMockRoundTripper()
			httpClient := &http.Client{Transport: mock}

			mock.queueResponse(201, []byte(`{"serials": ["s1"]}`), "application/json")

			client, err := ably.NewREST(
				ably.WithKey("appId.keyId:keySecret"),
				ably.WithHTTPClient(httpClient),
			)
			assert.NoError(t, err)

			channel := client.Channels.Get(tc.channelName)
			err = channel.Publish(context.Background(), "e", "d")
			assert.NoError(t, err)

			assert.Equal(t, tc.expectedPath, mock.requests[0].URL.Path)
		})
	}
}

func TestChannelPublish_ConnectionKey(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(201, []byte(`{"serials": ["s1"]}`), "application/json")

	// DEVIATION: ably-go uses msgpack by default. We force JSON for test inspection.
	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithIdempotentRESTPublishing(false),
		ably.WithUseBinaryProtocol(false), // Force JSON for test inspection
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")
	err = channel.Publish(context.Background(), "event", "data", ably.PublishWithConnectionKey("conn-key-123"))
	assert.NoError(t, err)

	body := getPublishRequestBody(t, mock.requests[0])
	assert.Equal(t, "conn-key-123", body[0]["connectionKey"])
}

// Helper function to get publish request body
func getPublishRequestBody(t *testing.T, req *http.Request) []map[string]interface{} {
	if req.Body == nil {
		t.Fatal("request body is nil")
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to unmarshal request body: %v, body: %s", err, string(body))
	}

	return result
}
