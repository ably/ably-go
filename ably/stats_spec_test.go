//go:build !integration
// +build !integration

package ably_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// RSC6a - stats() returns paginated results
// Tests that stats() returns a PaginatedResult of Stats objects.
// =============================================================================

func TestStats_RSC6a_ReturnsPaginatedResults(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	statsResp := `[
		{
			"intervalId": "2024-01-01:00:00",
			"unit": "hour",
			"all": {
				"messages": {"count": 100, "data": 5000},
				"all": {"count": 100, "data": 5000}
			}
		},
		{
			"intervalId": "2024-01-01:01:00",
			"unit": "hour",
			"all": {
				"messages": {"count": 150, "data": 7500},
				"all": {"count": 150, "data": 7500}
			}
		}
	]`
	mock.queueResponse(200, []byte(statsResp), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	result, err := client.Stats().Pages(context.Background())
	assert.NoError(t, err)

	// Result should be a PaginatedResult with items
	assert.NotNil(t, result)

	// Must call Next() to load the first page
	hasNext := result.Next(context.Background())
	assert.True(t, hasNext, "expected first page to be available")

	items := result.Items()
	assert.Equal(t, 2, len(items))

	// First stats object
	assert.Equal(t, "2024-01-01:00:00", items[0].IntervalID)

	// Verify correct endpoint was called
	assert.GreaterOrEqual(t, len(mock.requests), 1)
	request := mock.requests[0]
	assert.Equal(t, "GET", request.Method)
	assert.Equal(t, "/stats", request.URL.Path)
}

// =============================================================================
// RSC6a - stats() requires authentication
// Tests that stats() requires authentication.
// =============================================================================

func TestStats_RSC6a_RequiresAuthentication(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	_, err = client.Stats().Pages(context.Background())
	assert.NoError(t, err)

	request := mock.requests[0]

	// Request should have Authorization header
	assert.NotEmpty(t, request.Header.Get("Authorization"))
}

// =============================================================================
// RSC6b1 - stats() with start parameter
// Tests that the start parameter filters stats by start time.
// =============================================================================

func TestStats_RSC6b1_WithStartParameter(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err = client.Stats(ably.StatsWithStart(startTime)).Pages(context.Background())
	assert.NoError(t, err)

	request := mock.requests[0]
	assert.NotEmpty(t, request.URL.Query().Get("start"))
}

// =============================================================================
// RSC6b1 - stats() with end parameter
// Tests that the end parameter filters stats by end time.
// =============================================================================

func TestStats_RSC6b1_WithEndParameter(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	endTime := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)
	_, err = client.Stats(ably.StatsWithEnd(endTime)).Pages(context.Background())
	assert.NoError(t, err)

	request := mock.requests[0]
	assert.NotEmpty(t, request.URL.Query().Get("end"))
}

// =============================================================================
// RSC6b2 - stats() with limit parameter
// Tests that the limit parameter restricts the number of results.
// =============================================================================

func TestStats_RSC6b2_WithLimitParameter(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	_, err = client.Stats(ably.StatsWithLimit(10)).Pages(context.Background())
	assert.NoError(t, err)

	request := mock.requests[0]
	assert.Equal(t, "10", request.URL.Query().Get("limit"))
}

// =============================================================================
// RSC6b3 - stats() with direction parameter
// Tests that the direction parameter controls result ordering.
// =============================================================================

func TestStats_RSC6b3_WithDirectionParameter(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	_, err = client.Stats(ably.StatsWithDirection(ably.Forwards)).Pages(context.Background())
	assert.NoError(t, err)

	request := mock.requests[0]
	assert.Equal(t, "forwards", request.URL.Query().Get("direction"))
}

// =============================================================================
// RSC6b4 - stats() with unit parameter
// Tests that the unit parameter specifies the stats granularity.
// =============================================================================

func TestStats_RSC6b4_WithUnitParameter(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	// Valid units: minute, hour, day, month
	_, err = client.Stats(ably.StatsWithUnit(ably.StatGranularityDay)).Pages(context.Background())
	assert.NoError(t, err)

	request := mock.requests[0]
	assert.Equal(t, "day", request.URL.Query().Get("unit"))
}

// =============================================================================
// RSC6a - stats() empty results
// Tests that stats() handles empty results correctly.
// =============================================================================

func TestStats_RSC6a_EmptyResults(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	result, err := client.Stats().Pages(context.Background())
	assert.NoError(t, err)

	// Load first page (even if empty)
	result.Next(context.Background())

	items := result.Items()
	assert.NotNil(t, items)
	assert.Equal(t, 0, len(items))
}

// =============================================================================
// RSC6a - stats() error handling
// Tests that errors from the stats endpoint are properly propagated.
// =============================================================================

func TestStats_RSC6a_ErrorHandling(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	errorResp := `{"error":{"message":"Unauthorized","code":40100,"statusCode":401}}`
	mock.queueResponse(401, []byte(errorResp), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	_, err = client.Stats().Pages(context.Background())
	assert.Error(t, err)

	if errInfo, ok := err.(*ably.ErrorInfo); ok {
		assert.Equal(t, 401, errInfo.StatusCode)
		assert.Equal(t, 40100, int(errInfo.Code))
	}
}
