//go:build !integration
// +build !integration

package ably_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// RSC19f - Method signature supports required HTTP methods
// Tests that the request() method supports GET, POST, PUT, PATCH, and DELETE.
// =============================================================================

func TestRequest_RSC19f_SupportedHTTPMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			mock := newMockRoundTripper()
			httpClient := &http.Client{Transport: mock}

			mock.queueResponse(200, []byte(`[]`), "application/json")

			client, err := ably.NewREST(
				ably.WithKey("appId.keyId:keySecret"),
				ably.WithHTTPClient(httpClient),
				ably.WithUseBinaryProtocol(false),
			)
			assert.NoError(t, err)

			_, err = client.Request(method, "/test").Pages(context.Background())
			assert.NoError(t, err)

			assert.GreaterOrEqual(t, len(mock.requests), 1)
			request := mock.requests[0]
			assert.Equal(t, method, request.Method)
			assert.Equal(t, "/test", request.URL.Path)
		})
	}
}

// =============================================================================
// RSC19f - Query parameters passed correctly
// Tests that the params argument adds URL query parameters.
// =============================================================================

func TestRequest_RSC19f_QueryParameters(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
	)
	assert.NoError(t, err)

	_, err = client.Request(
		"GET",
		"/channels/test/messages",
		ably.RequestWithParams(url.Values{
			"limit":     []string{"10"},
			"direction": []string{"backwards"},
		}),
	).Pages(context.Background())
	assert.NoError(t, err)

	request := mock.requests[0]
	assert.Equal(t, "10", request.URL.Query().Get("limit"))
	assert.Equal(t, "backwards", request.URL.Query().Get("direction"))
}

// =============================================================================
// RSC19f - Custom headers passed correctly
// Tests that the headers argument adds custom HTTP headers.
// =============================================================================

func TestRequest_RSC19f_CustomHeaders(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
	)
	assert.NoError(t, err)

	_, err = client.Request(
		"GET",
		"/test",
		ably.RequestWithHeaders(http.Header{
			"X-Custom-Header": []string{"custom-value"},
			"X-Another":       []string{"another-value"},
		}),
	).Pages(context.Background())
	assert.NoError(t, err)

	request := mock.requests[0]
	assert.Equal(t, "custom-value", request.Header.Get("X-Custom-Header"))
	assert.Equal(t, "another-value", request.Header.Get("X-Another"))
}

// =============================================================================
// RSC19f - Request body sent correctly
// Tests that the body argument is included in the request.
// =============================================================================

func TestRequest_RSC19f_RequestBody(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(201, []byte(`{"id":"123"}`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
	)
	assert.NoError(t, err)

	_, err = client.Request(
		"POST",
		"/channels/test/messages",
		ably.RequestWithBody(map[string]interface{}{
			"name": "event",
			"data": "payload",
		}),
	).Pages(context.Background())
	assert.NoError(t, err)

	request := mock.requests[0]
	assert.Equal(t, "POST", request.Method)
	// Body should be sent (verified by mock capturing it)
	assert.NotNil(t, request.Body)
}

// =============================================================================
// RSC19b - Uses configured authentication (Basic)
// Tests that request() uses the REST client's configured Basic authentication.
// =============================================================================

func TestRequest_RSC19b_BasicAuthentication(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
	)
	assert.NoError(t, err)

	_, err = client.Request("GET", "/test").Pages(context.Background())
	assert.NoError(t, err)

	request := mock.requests[0]
	authHeader := request.Header.Get("Authorization")
	assert.NotEmpty(t, authHeader)
	assert.Contains(t, authHeader, "Basic ")

	// Verify the base64 encoded credentials
	encodedCreds := authHeader[6:] // Skip "Basic "
	decodedCreds, err := base64.StdEncoding.DecodeString(encodedCreds)
	assert.NoError(t, err)
	assert.Equal(t, "appId.keyId:keySecret", string(decodedCreds))
}

// =============================================================================
// RSC19b - Uses configured authentication (Token)
// Tests that request() uses the REST client's configured Token authentication.
// =============================================================================

func TestRequest_RSC19b_TokenAuthentication(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithToken("my-token-string"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
	)
	assert.NoError(t, err)

	_, err = client.Request("GET", "/test").Pages(context.Background())
	assert.NoError(t, err)

	request := mock.requests[0]
	authHeader := request.Header.Get("Authorization")
	assert.NotEmpty(t, authHeader)
	assert.Contains(t, authHeader, "Bearer ")
}

// =============================================================================
// RSC19c - Protocol headers set correctly (JSON)
// Tests that Accept and Content-Type headers reflect JSON protocol.
// =============================================================================

func TestRequest_RSC19c_JSONProtocolHeaders(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
	)
	assert.NoError(t, err)

	_, err = client.Request(
		"POST",
		"/test",
		ably.RequestWithBody(map[string]string{"data": "test"}),
	).Pages(context.Background())
	assert.NoError(t, err)

	request := mock.requests[0]
	assert.Equal(t, "application/json", request.Header.Get("Accept"))
	assert.Equal(t, "application/json", request.Header.Get("Content-Type"))
}

// =============================================================================
// RSC19d, HP4 - HttpPaginatedResponse provides status code
// Tests that the response object provides access to the HTTP status code.
// =============================================================================

func TestRequest_RSC19d_HP4_StatusCode(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		body       string
	}{
		{"200 OK", 200, `[]`},
		{"201 Created", 201, `{"id":"123"}`},
		{"400 Bad Request", 400, `{"error":{"code":40000,"message":"Bad request"}}`},
		{"404 Not Found", 404, `{"error":{"code":40400,"message":"Not found"}}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := newMockRoundTripper()
			httpClient := &http.Client{Transport: mock}

			mock.queueResponse(tc.statusCode, []byte(tc.body), "application/json")

			client, err := ably.NewREST(
				ably.WithKey("appId.keyId:keySecret"),
				ably.WithHTTPClient(httpClient),
				ably.WithUseBinaryProtocol(false),
			)
			assert.NoError(t, err)

			response, err := client.Request("GET", "/test").Pages(context.Background())
			assert.NoError(t, err)

			assert.Equal(t, tc.statusCode, response.StatusCode())
		})
	}
}

// =============================================================================
// RSC19d, HP5 - HttpPaginatedResponse provides success indicator
// Tests that the success property correctly reflects 2xx status codes.
// =============================================================================

func TestRequest_RSC19d_HP5_SuccessIndicator(t *testing.T) {
	testCases := []struct {
		name            string
		statusCode      int
		body            string
		expectedSuccess bool
	}{
		{"200 OK", 200, `[]`, true},
		{"201 Created", 201, `{"id":"123"}`, true},
		{"204 No Content", 204, ``, true},
		{"400 Bad Request", 400, `{"error":{"code":40000,"message":"Bad"}}`, false},
		// Note: 500 errors may behave differently - skipped for now
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := newMockRoundTripper()
			httpClient := &http.Client{Transport: mock}

			mock.queueResponse(tc.statusCode, []byte(tc.body), "application/json")

			client, err := ably.NewREST(
				ably.WithKey("appId.keyId:keySecret"),
				ably.WithHTTPClient(httpClient),
				ably.WithUseBinaryProtocol(false),
			)
			assert.NoError(t, err)

			response, err := client.Request("GET", "/test").Pages(context.Background())
			assert.NoError(t, err)

			assert.Equal(t, tc.expectedSuccess, response.Success())
		})
	}
}

// =============================================================================
// RSC19d, HP6 - HttpPaginatedResponse provides error code from header
// Tests that the errorCode property extracts the value from X-Ably-Errorcode.
// =============================================================================

func TestRequest_RSC19d_HP6_ErrorCode(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponseWithHeaders(401,
		[]byte(`{"error":{"code":40101,"message":"Unauthorized"}}`),
		"application/json",
		map[string]string{"X-Ably-Errorcode": "40101"},
	)

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
	)
	assert.NoError(t, err)

	response, err := client.Request("GET", "/test").Pages(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, ably.ErrorCode(40101), response.ErrorCode())
}

// =============================================================================
// RSC19d, HP7 - HttpPaginatedResponse provides error message from header
// Tests that errorMessage extracts the value from X-Ably-Errormessage.
// =============================================================================

func TestRequest_RSC19d_HP7_ErrorMessage(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponseWithHeaders(401,
		[]byte(`{"error":{"code":40101,"message":"Unauthorized"}}`),
		"application/json",
		map[string]string{
			"X-Ably-Errorcode":    "40101",
			"X-Ably-Errormessage": "Token expired",
		},
	)

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
	)
	assert.NoError(t, err)

	response, err := client.Request("GET", "/test").Pages(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, "Token expired", response.ErrorMessage())
}

// =============================================================================
// RSC19d, HP8 - HttpPaginatedResponse provides all response headers
// Tests that all response headers are accessible.
// =============================================================================

func TestRequest_RSC19d_HP8_ResponseHeaders(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponseWithHeaders(200,
		[]byte(`[]`),
		"application/json",
		map[string]string{
			"X-Request-Id":    "req-123",
			"X-Custom-Header": "custom-value",
		},
	)

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
	)
	assert.NoError(t, err)

	response, err := client.Request("GET", "/test").Pages(context.Background())
	assert.NoError(t, err)

	headers := response.Headers()
	assert.Equal(t, "application/json", headers.Get("Content-Type"))
	assert.Equal(t, "req-123", headers.Get("X-Request-Id"))
	assert.Equal(t, "custom-value", headers.Get("X-Custom-Header"))
}

// =============================================================================
// RSC19d, HP3 - HttpPaginatedResponse provides response items
// Tests that the items() method returns the decoded response body.
// =============================================================================

func TestRequest_RSC19d_HP3_ResponseItems(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200,
		[]byte(`[{"id":"msg1","name":"event1"},{"id":"msg2","name":"event2"}]`),
		"application/json",
	)

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
	)
	assert.NoError(t, err)

	response, err := client.Request("GET", "/channels/test/messages").Pages(context.Background())
	assert.NoError(t, err)

	// Load first page
	hasNext := response.Next(context.Background())
	assert.True(t, hasNext)

	var items []map[string]interface{}
	err = response.Items(&items)
	assert.NoError(t, err)

	assert.Equal(t, 2, len(items))
	assert.Equal(t, "msg1", items[0]["id"])
	assert.Equal(t, "msg2", items[1]["id"])
}

// =============================================================================
// RSC19d, HP1 - HttpPaginatedResponse pagination support
// Tests that multi-page responses can be navigated using next().
// =============================================================================

func TestRequest_RSC19d_HP1_Pagination(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	// First page with Link header pointing to next page
	mock.queueResponseWithHeaders(200,
		[]byte(`[{"id":"1"},{"id":"2"}]`),
		"application/json",
		map[string]string{
			"Link": `</channels/test/messages?page=2>; rel="next"`,
		},
	)

	// Second page (last)
	mock.queueResponse(200, []byte(`[{"id":"3"}]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
	)
	assert.NoError(t, err)

	response, err := client.Request("GET", "/channels/test/messages").Pages(context.Background())
	assert.NoError(t, err)

	// First page
	hasNext := response.Next(context.Background())
	assert.True(t, hasNext)

	var items1 []map[string]interface{}
	err = response.Items(&items1)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(items1))

	// Check if there's a next page
	hasMore := response.HasNext(context.Background())
	assert.True(t, hasMore)

	// Navigate to second page
	hasNext = response.Next(context.Background())
	assert.True(t, hasNext)

	var items2 []map[string]interface{}
	err = response.Items(&items2)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(items2))
	assert.Equal(t, "3", items2[0]["id"])

	// No more pages
	hasMore = response.HasNext(context.Background())
	assert.False(t, hasMore)
}

// =============================================================================
// RSC19e - HTTP error status does not trigger fallback
// Tests that HTTP error responses are returned directly without fallback retry.
// =============================================================================

func TestRequest_RSC19e_NoFallbackOnHTTPError(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponseWithHeaders(400,
		[]byte(`{"error":{"code":40000,"message":"Bad request"}}`),
		"application/json",
		map[string]string{"X-Ably-Errorcode": "40000"},
	)

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
		ably.WithFallbackHosts([]string{"a.ably-realtime.com", "b.ably-realtime.com"}),
	)
	assert.NoError(t, err)

	response, err := client.Request("GET", "/test").Pages(context.Background())
	assert.NoError(t, err)

	// Should return the error response, not retry to fallback
	assert.Equal(t, 400, response.StatusCode())
	assert.False(t, response.Success())
	assert.Equal(t, ably.ErrorCode(40000), response.ErrorCode())

	// Only one request should have been made (no fallback)
	assert.Equal(t, 1, len(mock.requests))
}

// =============================================================================
// RSC19d - Empty response handling
// Tests that empty responses (204 No Content) are handled correctly.
// =============================================================================

func TestRequest_RSC19d_EmptyResponse(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(204, []byte(``), "")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
	)
	assert.NoError(t, err)

	response, err := client.Request("DELETE", "/channels/test/messages/123").Pages(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, 204, response.StatusCode())
	assert.True(t, response.Success())
}
