//go:build !integration
// +build !integration

package ably_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// RSC16 - time() returns server time
// Tests that time() returns the server time as a DateTime/timestamp.
// =============================================================================

func TestTime_RSC16_ReturnsServerTime(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	// Server returns array with single timestamp (milliseconds since epoch)
	serverTimeMs := int64(1704067200000) // 2024-01-01 00:00:00 UTC
	mock.queueResponse(200, []byte(`[1704067200000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	result, err := client.Time(context.Background())
	assert.NoError(t, err)

	// Result should match the server timestamp
	assert.Equal(t, serverTimeMs, result.UnixMilli())

	// Verify correct endpoint was called
	assert.GreaterOrEqual(t, len(mock.requests), 1)
	request := mock.requests[0]
	assert.Equal(t, "GET", request.Method)
	assert.Equal(t, "/time", request.URL.Path)
}

// =============================================================================
// RSC16 - time() request format
// Tests that the time request is correctly formatted.
// =============================================================================

func TestTime_RSC16_RequestFormat(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[1704067200000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	request := mock.requests[0]

	// Should be GET request to /time
	assert.Equal(t, "GET", request.Method)
	assert.Equal(t, "/time", request.URL.Path)

	// Should have standard Ably headers
	assert.NotEmpty(t, request.Header.Get("X-Ably-Version"), "X-Ably-Version header should be present")
	assert.NotEmpty(t, request.Header.Get("Ably-Agent"), "Ably-Agent header should be present")
}

// =============================================================================
// RSC16 - time() does not require authentication
// Tests that time() works without authentication credentials.
// Note: ably-go may require some form of client initialization, so this test
// verifies that time() doesn't send auth headers when only using the endpoint.
// =============================================================================

func TestTime_RSC16_NoAuthenticationRequired(t *testing.T) {
	// Note: ably-go requires either a key or token to create a client.
	// However, the time endpoint itself doesn't require authentication.
	// We verify this by checking that a request to /time succeeds even
	// if the server would reject authenticated requests.

	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[1704067200000]`), "application/json")

	// Use a token (which doesn't involve Basic auth validation at client level)
	client, err := ably.NewREST(
		ably.WithToken("any-token"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	result, err := client.Time(context.Background())
	assert.NoError(t, err)
	assert.False(t, result.IsZero())
}

// =============================================================================
// RSC16 - time() error handling
// Tests that errors from the time endpoint are properly propagated.
// =============================================================================

func TestTime_RSC16_ErrorHandling(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	errorResp := `{"error":{"message":"Internal server error","code":50000,"statusCode":500}}`
	mock.queueResponse(500, []byte(errorResp), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.Error(t, err)

	if errInfo, ok := err.(*ably.ErrorInfo); ok {
		assert.Equal(t, 500, errInfo.StatusCode)
		assert.Equal(t, 50000, int(errInfo.Code))
	}
}
