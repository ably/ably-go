//go:build !integration
// +build !integration

package ably_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// RSA4b4 - Token renewal on expiry rejection
// Tests that when a request is rejected with a token expiry error, the library
// obtains a new token and retries.
// =============================================================================

func TestTokenRenewal_RSA4b4_RenewalOnExpiryRejection(t *testing.T) {
	// SKIP: ably-go REST client does not automatically retry on token expiry (40142)
	// This behavior may be implemented differently or only available in realtime client
	t.Skip("RSA4b4 - ably-go REST client does not auto-retry on token expiry")

	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	callbackCount := 0
	tokens := []string{"first-token", "second-token"}

	authCallback := func(ctx context.Context, params ably.TokenParams) (ably.Tokener, error) {
		token := tokens[callbackCount]
		callbackCount++
		return &ably.TokenDetails{
			Token:   token,
			Expires: time.Now().Add(time.Hour).UnixMilli(),
		}, nil
	}

	client, err := ably.NewREST(
		ably.WithAuthCallback(authCallback),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	// First request fails with 40142 (token expired)
	errorResp := `{"error":{"code":40142,"statusCode":401,"message":"Token expired"}}`
	mock.queueResponse(401, []byte(errorResp), "application/json")
	// After renewal, second attempt succeeds
	mock.queueResponse(200, []byte(`[]`), "application/json")

	channel := client.Channels.Get("test")
	pages, err := channel.History().Pages(context.Background())
	assert.NoError(t, err)
	_ = pages

	// authCallback was called twice (initial + renewal)
	assert.Equal(t, 2, callbackCount, "authCallback should be called twice")

	// Two HTTP requests were made to the API
	apiRequests := filterRequestsByPath(mock.requests, "/channels/")
	assert.Equal(t, 2, len(apiRequests), "expected two API requests")

	// First request used first token
	firstAuth := apiRequests[0].Header.Get("Authorization")
	expectedFirst := "Bearer " + base64.StdEncoding.EncodeToString([]byte("first-token"))
	assert.Equal(t, expectedFirst, firstAuth)

	// Second request used renewed token
	secondAuth := apiRequests[1].Header.Get("Authorization")
	expectedSecond := "Bearer " + base64.StdEncoding.EncodeToString([]byte("second-token"))
	assert.Equal(t, expectedSecond, secondAuth)
}

// =============================================================================
// RSA4b4 - Token renewal on 40140 error
// Tests renewal is triggered for error code 40140 (token error).
// =============================================================================

func TestTokenRenewal_RSA4b4_RenewalOn40140Error(t *testing.T) {
	// SKIP: ably-go REST client does not automatically retry on token errors
	t.Skip("RSA4b4 - ably-go REST client does not auto-retry on token errors")

	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	callbackCount := 0

	authCallback := func(ctx context.Context, params ably.TokenParams) (ably.Tokener, error) {
		callbackCount++
		return &ably.TokenDetails{
			Token:   fmt.Sprintf("token-%d", callbackCount),
			Expires: time.Now().Add(time.Hour).UnixMilli(),
		}, nil
	}

	client, err := ably.NewREST(
		ably.WithAuthCallback(authCallback),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	// First attempt fails with 40140
	errorResp := `{"error":{"code":40140,"statusCode":401,"message":"Token error"}}`
	mock.queueResponse(401, []byte(errorResp), "application/json")
	// Retry succeeds
	mock.queueResponse(200, []byte(`[]`), "application/json")

	channel := client.Channels.Get("test")
	_, err = channel.History().Pages(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, 2, callbackCount, "authCallback should be called twice")
	apiRequests := filterRequestsByPath(mock.requests, "/channels/")
	assert.Equal(t, 2, len(apiRequests), "expected two API requests")
}

// =============================================================================
// RSA14 - Pre-emptive token renewal
// Tests that if a token is known to be expired before making a request,
// renewal happens without first making a failing request.
// =============================================================================

func TestTokenRenewal_RSA14_PreemptiveRenewal(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	callbackCount := 0

	authCallback := func(ctx context.Context, params ably.TokenParams) (ably.Tokener, error) {
		callbackCount++
		if callbackCount == 1 {
			// First token is already expired
			return &ably.TokenDetails{
				Token:   "expired-token",
				Expires: time.Now().Add(-time.Second).UnixMilli(), // Already expired
			}, nil
		}
		return &ably.TokenDetails{
			Token:   "fresh-token",
			Expires: time.Now().Add(time.Hour).UnixMilli(),
		}, nil
	}

	client, err := ably.NewREST(
		ably.WithAuthCallback(authCallback),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	// Force initial token acquisition
	_, err = client.Auth.Authorize(context.Background(), nil)
	assert.NoError(t, err)

	// Queue only success response (no 401 expected)
	mock.queueResponse(200, []byte(`[]`), "application/json")

	// This should detect expired token and renew before request
	channel := client.Channels.Get("test")
	_, err = channel.History().Pages(context.Background())
	assert.NoError(t, err)

	// Callback was called twice (initial + pre-emptive renewal)
	assert.Equal(t, 2, callbackCount, "authCallback should be called twice for pre-emptive renewal")

	// Only ONE HTTP request to the channels API (no failed request with expired token)
	apiRequests := filterRequestsByPath(mock.requests, "/channels/")
	assert.Equal(t, 1, len(apiRequests), "expected only one API request (pre-emptive renewal)")

	// The request used the fresh token
	authHeader := apiRequests[0].Header.Get("Authorization")
	expectedAuth := "Bearer " + base64.StdEncoding.EncodeToString([]byte("fresh-token"))
	assert.Equal(t, expectedAuth, authHeader)
}

// =============================================================================
// RSA4b4 - No renewal without authCallback
// Tests that token renewal is not attempted if no renewal mechanism is available.
// =============================================================================

func TestTokenRenewal_RSA4b4_NoRenewalWithoutCallback(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	// Client with explicit token but no authCallback
	client, err := ably.NewREST(
		ably.WithToken("static-token"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	// Token expired error
	errorResp := `{"error":{"code":40142,"statusCode":401,"message":"Token expired"}}`
	mock.queueResponse(401, []byte(errorResp), "application/json")

	channel := client.Channels.Get("test")
	_, err = channel.History().Pages(context.Background())

	// Should get an error
	assert.Error(t, err)

	// Verify it's a token error
	if errInfo, ok := err.(*ably.ErrorInfo); ok {
		assert.Equal(t, 40142, int(errInfo.Code))
	}

	// Only one request was made (no retry)
	apiRequests := filterRequestsByPath(mock.requests, "/channels/")
	assert.Equal(t, 1, len(apiRequests), "expected only one request without renewal capability")
}

// =============================================================================
// RSA4b4 - Renewal with authUrl
// Tests that token renewal works via authUrl.
// =============================================================================

func TestTokenRenewal_RSA4b4_RenewalWithAuthUrl(t *testing.T) {
	// SKIP: ably-go REST client does not automatically retry on token expiry
	t.Skip("RSA4b4 - ably-go REST client does not auto-retry on token expiry")

	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	client, err := ably.NewREST(
		ably.WithAuthURL("https://example.com/auth"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	// First token request from authUrl
	expires := time.Now().Add(time.Hour).UnixMilli()
	firstTokenResp, _ := json.Marshal(map[string]interface{}{
		"token":   "first-token",
		"expires": expires,
	})
	mock.queueResponse(200, firstTokenResp, "application/json")

	// First API request fails with token expired
	errorResp := `{"error":{"code":40142,"statusCode":401,"message":"Token expired"}}`
	mock.queueResponse(401, []byte(errorResp), "application/json")

	// Second token request (renewal) from authUrl
	secondTokenResp, _ := json.Marshal(map[string]interface{}{
		"token":   "second-token",
		"expires": expires,
	})
	mock.queueResponse(200, secondTokenResp, "application/json")

	// Retry succeeds
	mock.queueResponse(200, []byte(`[]`), "application/json")

	channel := client.Channels.Get("test")
	_, err = channel.History().Pages(context.Background())
	assert.NoError(t, err)

	// Count requests to authUrl
	authRequests := filterRequestsByHost(mock.requests, "example.com")
	assert.Equal(t, 2, len(authRequests), "expected two requests to authUrl")

	// Count requests to Ably API
	apiRequests := filterRequestsByPath(mock.requests, "/channels/")
	assert.Equal(t, 2, len(apiRequests), "expected two API requests")

	// Second API request used renewed token
	secondAuth := apiRequests[1].Header.Get("Authorization")
	expectedAuth := "Bearer " + base64.StdEncoding.EncodeToString([]byte("second-token"))
	assert.Equal(t, expectedAuth, secondAuth)
}

// =============================================================================
// RSA4b4 - Renewal limit
// Tests that token renewal doesn't loop infinitely if server keeps rejecting.
// =============================================================================

func TestTokenRenewal_RSA4b4_RenewalLimit(t *testing.T) {
	// SKIP: ably-go REST client does not automatically retry on token expiry
	t.Skip("RSA4b4 - ably-go REST client does not auto-retry on token expiry")

	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	callbackCount := 0

	authCallback := func(ctx context.Context, params ably.TokenParams) (ably.Tokener, error) {
		callbackCount++
		return &ably.TokenDetails{
			Token:   fmt.Sprintf("token-%d", callbackCount),
			Expires: time.Now().Add(time.Hour).UnixMilli(),
		}, nil
	}

	client, err := ably.NewREST(
		ably.WithAuthCallback(authCallback),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	// Always return token expired
	for i := 0; i < 10; i++ {
		errorResp := `{"error":{"code":40142,"statusCode":401,"message":"Token expired"}}`
		mock.queueResponse(401, []byte(errorResp), "application/json")
	}

	channel := client.Channels.Get("test")
	_, err = channel.History().Pages(context.Background())

	// Should eventually give up and return error
	assert.Error(t, err)

	// Should not retry indefinitely (implementation-specific limit)
	assert.LessOrEqual(t, callbackCount, 3, "should not retry more than a reasonable limit")
}

// Helper function to filter requests by path substring
func filterRequestsByPath(requests []*http.Request, pathContains string) []*http.Request {
	var filtered []*http.Request
	for _, req := range requests {
		if strings.Contains(req.URL.Path, pathContains) {
			filtered = append(filtered, req)
		}
	}
	return filtered
}

// Helper function to filter requests by host
func filterRequestsByHost(requests []*http.Request, host string) []*http.Request {
	var filtered []*http.Request
	for _, req := range requests {
		if req.URL.Host == host {
			filtered = append(filtered, req)
		}
	}
	return filtered
}
