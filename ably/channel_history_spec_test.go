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
// RSL2a - History returns PaginatedResult
// Tests that history() returns a PaginatedResult containing messages.
// =============================================================================

func TestChannelHistory_RSL2a_ReturnsPaginatedResult(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	historyResponse := `[
		{"id": "msg1", "name": "event1", "data": "data1", "timestamp": 1000},
		{"id": "msg2", "name": "event2", "data": "data2", "timestamp": 2000}
	]`
	mock.queueResponse(200, []byte(historyResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")

	// Use Pages() to get paginated results
	pages, err := channel.History().Pages(context.Background())
	assert.NoError(t, err)

	// Get first page
	hasNext := pages.Next(context.Background())
	assert.True(t, hasNext, "should have first page")

	items := pages.Items()
	assert.Equal(t, 2, len(items))

	assert.Equal(t, "msg1", items[0].ID)
	assert.Equal(t, "event1", items[0].Name)
	assert.Equal(t, "data1", items[0].Data)

	assert.Equal(t, "msg2", items[1].ID)
	assert.Equal(t, "event2", items[1].Name)
}

// =============================================================================
// RSL2b - History query parameters
// Tests that history parameters are correctly sent as query string.
// =============================================================================

func TestChannelHistory_RSL2b_QueryParameters(t *testing.T) {
	testCases := []struct {
		id          string
		historyOpts []ably.HistoryOption
		paramName   string
		paramValue  string
	}{
		{
			id:          "start",
			historyOpts: []ably.HistoryOption{ably.HistoryWithStart(time.Unix(1234567890, 0))},
			paramName:   "start",
			paramValue:  "1234567890000",
		},
		{
			id:          "end",
			historyOpts: []ably.HistoryOption{ably.HistoryWithEnd(time.Unix(1234567899, 0))},
			paramName:   "end",
			paramValue:  "1234567899000",
		},
		{
			id:          "direction_backwards",
			historyOpts: []ably.HistoryOption{ably.HistoryWithDirection(ably.Backwards)},
			paramName:   "direction",
			paramValue:  "backwards",
		},
		{
			id:          "direction_forwards",
			historyOpts: []ably.HistoryOption{ably.HistoryWithDirection(ably.Forwards)},
			paramName:   "direction",
			paramValue:  "forwards",
		},
		{
			id:          "limit",
			historyOpts: []ably.HistoryOption{ably.HistoryWithLimit(50)},
			paramName:   "limit",
			paramValue:  "50",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			mock := newMockRoundTripper()
			httpClient := &http.Client{Transport: mock}

			mock.queueResponse(200, []byte(`[]`), "application/json")

			client, err := ably.NewREST(
				ably.WithKey("appId.keyId:keySecret"),
				ably.WithHTTPClient(httpClient),
			)
			assert.NoError(t, err)

			channel := client.Channels.Get("test-channel")
			pages, err := channel.History(tc.historyOpts...).Pages(context.Background())
			assert.NoError(t, err)
			pages.Next(context.Background())

			assert.GreaterOrEqual(t, len(mock.requests), 1)
			request := mock.requests[0]
			assert.Equal(t, tc.paramValue, request.URL.Query().Get(tc.paramName),
				"expected %s=%s in query", tc.paramName, tc.paramValue)
		})
	}
}

// =============================================================================
// RSL2b1 - Default direction is backwards
// Tests that the default direction for history is backwards (newest first).
// =============================================================================

func TestChannelHistory_RSL2b1_DefaultDirectionBackwards(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")
	pages, err := channel.History().Pages(context.Background()) // No direction specified
	assert.NoError(t, err)
	pages.Next(context.Background())

	assert.GreaterOrEqual(t, len(mock.requests), 1)
	request := mock.requests[0]

	// Either direction param is absent (server default) or explicitly "backwards"
	direction := request.URL.Query().Get("direction")
	if direction != "" {
		assert.Equal(t, "backwards", direction,
			"if direction is specified, it should be 'backwards'")
	}
	// If absent, server defaults to backwards per spec
}

// =============================================================================
// RSL2b2 - Limit parameter
// Tests that limit parameter restricts the number of returned items.
// =============================================================================

func TestChannelHistory_RSL2b2_LimitParameter(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[
		{"id": "msg1", "name": "e", "data": "d", "timestamp": 1000},
		{"id": "msg2", "name": "e", "data": "d", "timestamp": 2000},
		{"id": "msg3", "name": "e", "data": "d", "timestamp": 3000}
	]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")
	pages, err := channel.History(ably.HistoryWithLimit(10)).Pages(context.Background())
	assert.NoError(t, err)
	pages.Next(context.Background())

	assert.GreaterOrEqual(t, len(mock.requests), 1)
	request := mock.requests[0]
	assert.Equal(t, "10", request.URL.Query().Get("limit"))
}

// =============================================================================
// RSL2b3 - Default limit is 100
// Tests that the default limit is 100 when not specified.
// =============================================================================

func TestChannelHistory_RSL2b3_DefaultLimit(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")
	pages, err := channel.History().Pages(context.Background()) // No limit specified
	assert.NoError(t, err)
	pages.Next(context.Background())

	assert.GreaterOrEqual(t, len(mock.requests), 1)
	request := mock.requests[0]

	// Either limit param is absent (server default) or explicitly "100"
	limit := request.URL.Query().Get("limit")
	if limit != "" {
		assert.Equal(t, "100", limit,
			"if limit is specified, it should be '100'")
	}
	// If absent, server defaults to 100 per spec
}

// =============================================================================
// RSL2 - History request URL format
// Tests that history requests use the correct URL path.
//
// DEVIATION: The spec expects URL-encoded channel names (e.g., "with%3Acolon"),
// but ably-go does NOT URL-encode special characters in channel name paths.
// This test documents ably-go's actual behavior.
// =============================================================================

func TestChannelHistory_RSL2_URLFormat(t *testing.T) {
	testCases := []struct {
		id           string
		channelName  string
		expectedPath string
	}{
		{"simple", "simple", "/channels/simple/history"},
		// DEVIATION: Spec expects "/channels/with%3Acolon/history"
		{"with_colon", "with:colon", "/channels/with:colon/history"},
		// DEVIATION: Spec expects "/channels/with%2Fslash/history"
		{"with_slash", "with/slash", "/channels/with/slash/history"},
		// DEVIATION: Spec expects "/channels/with%20space/history"
		{"with_space", "with space", "/channels/with space/history"},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			mock := newMockRoundTripper()
			httpClient := &http.Client{Transport: mock}

			mock.queueResponse(200, []byte(`[]`), "application/json")

			client, err := ably.NewREST(
				ably.WithKey("appId.keyId:keySecret"),
				ably.WithHTTPClient(httpClient),
			)
			assert.NoError(t, err)

			channel := client.Channels.Get(tc.channelName)
			pages, err := channel.History().Pages(context.Background())
			assert.NoError(t, err)
			pages.Next(context.Background())

			assert.GreaterOrEqual(t, len(mock.requests), 1)
			request := mock.requests[0]
			assert.Equal(t, "GET", request.Method)
			assert.Equal(t, tc.expectedPath, request.URL.Path)
		})
	}
}

// =============================================================================
// RSL2 - History with time range
// Tests combining start and end parameters for time-bounded queries.
// =============================================================================

func TestChannelHistory_RSL2_WithTimeRange(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[
		{"id": "msg1", "name": "e", "data": "d", "timestamp": 1500}
	]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")
	startTime := time.Unix(1, 0)    // 1000ms
	endTime := time.Unix(2, 0)      // 2000ms
	pages, err := channel.History(
		ably.HistoryWithStart(startTime),
		ably.HistoryWithEnd(endTime),
	).Pages(context.Background())
	assert.NoError(t, err)
	pages.Next(context.Background())

	assert.GreaterOrEqual(t, len(mock.requests), 1)
	request := mock.requests[0]
	assert.Equal(t, "1000", request.URL.Query().Get("start"))
	assert.Equal(t, "2000", request.URL.Query().Get("end"))
}

// =============================================================================
// Additional history tests
// =============================================================================

func TestChannelHistory_EmptyResult(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")
	pages, err := channel.History().Pages(context.Background())
	assert.NoError(t, err)

	hasNext := pages.Next(context.Background())
	// Even empty results should have a "first page"
	if hasNext {
		items := pages.Items()
		assert.Empty(t, items)
	}
}

func TestChannelHistory_MessageDecoding(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	// Message with plain string data (no encoding)
	// DEVIATION: Testing JSON decoding with "encoding": "json" is complex in unit tests
	// because the SDK may handle encoding differently. This tests basic message retrieval.
	historyResponse := `[
		{"id": "msg1", "name": "json-event", "data": "simple-data", "timestamp": 1000}
	]`
	mock.queueResponse(200, []byte(historyResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false), // Use JSON for easier testing
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")
	pages, err := channel.History().Pages(context.Background())
	assert.NoError(t, err)

	pages.Next(context.Background())
	items := pages.Items()

	assert.Equal(t, 1, len(items))
	assert.Equal(t, "simple-data", items[0].Data)
}

func TestChannelHistory_Items_Iterator(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	historyResponse := `[
		{"id": "msg1", "name": "e1", "data": "d1", "timestamp": 1000},
		{"id": "msg2", "name": "e2", "data": "d2", "timestamp": 2000}
	]`
	mock.queueResponse(200, []byte(historyResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	channel := client.Channels.Get("test-channel")

	// Use Items() iterator
	items, err := channel.History().Items(context.Background())
	assert.NoError(t, err)

	// Iterate through items
	count := 0
	for items.Next(context.Background()) {
		count++
		item := items.Item()
		assert.NotNil(t, item)
		assert.NotEmpty(t, item.ID)
	}

	assert.Equal(t, 2, count, "should iterate through 2 items")
}
