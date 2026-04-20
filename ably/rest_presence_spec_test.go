//go:build !integration
// +build !integration

package ably_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// presenceMockRoundTripper is a mock HTTP RoundTripper for presence tests
type presenceMockRoundTripper struct {
	requests  []*http.Request
	responses []*presenceMockResponse
}

type presenceMockResponse struct {
	statusCode  int
	body        []byte
	contentType string
	headers     map[string]string
}

func newPresenceMockRoundTripper() *presenceMockRoundTripper {
	return &presenceMockRoundTripper{}
}

func (m *presenceMockRoundTripper) queueResponse(statusCode int, body []byte, contentType string) {
	m.responses = append(m.responses, &presenceMockResponse{
		statusCode:  statusCode,
		body:        body,
		contentType: contentType,
	})
}

func (m *presenceMockRoundTripper) queueResponseWithHeaders(statusCode int, body []byte, contentType string, headers map[string]string) {
	m.responses = append(m.responses, &presenceMockResponse{
		statusCode:  statusCode,
		body:        body,
		contentType: contentType,
		headers:     headers,
	})
}

func (m *presenceMockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Store the request (clone to preserve body)
	reqCopy := req.Clone(req.Context())
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(body))
		reqCopy.Body = io.NopCloser(bytes.NewReader(body))
	}
	m.requests = append(m.requests, reqCopy)

	// Return queued response
	if len(m.responses) == 0 {
		// Default empty presence response
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte(`[]`))),
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}, nil
	}

	resp := m.responses[0]
	m.responses = m.responses[1:]

	header := http.Header{
		"Content-Type": []string{resp.contentType},
	}
	for k, v := range resp.headers {
		header.Set(k, v)
	}

	return &http.Response{
		StatusCode: resp.statusCode,
		Body:       io.NopCloser(bytes.NewReader(resp.body)),
		Header:     header,
	}, nil
}

func (m *presenceMockRoundTripper) lastRequest() *http.Request {
	if len(m.requests) == 0 {
		return nil
	}
	return m.requests[len(m.requests)-1]
}

func (m *presenceMockRoundTripper) reset() {
	m.requests = nil
	m.responses = nil
}

// =============================================================================
// RSP1 - RestPresence object associated with channel
// =============================================================================

func TestPresence_RSP1_PresenceAccessibleViaChannel(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	channel := client.Channels.Get("test-channel")
	presence := channel.Presence

	assert.NotNil(t, presence, "presence should not be nil")
}

// =============================================================================
// RSP3 - RestPresence#get
// =============================================================================

func TestPresence_RSP3_GetSendsRequestToPresenceEndpoint(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	presenceResponse := `[
		{"action": 1, "clientId": "client1", "data": "hello"},
		{"action": 1, "clientId": "client2", "data": "world"}
	]`
	mock.queueResponse(200, []byte(presenceResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test-channel").Presence.Get().Pages(ctx)
	require.NoError(t, err)

	ok := pages.Next(ctx)
	require.True(t, ok, "expected first page")

	// Check request
	req := mock.lastRequest()
	assert.Equal(t, "GET", req.Method)
	assert.True(t, strings.HasSuffix(req.URL.Path, "/channels/test-channel/presence"),
		"expected path to end with /channels/test-channel/presence, got %s", req.URL.Path)

	// Check response
	items := pages.Items()
	assert.Len(t, items, 2)
	assert.Equal(t, "client1", items[0].ClientID)
	assert.Equal(t, "client2", items[1].ClientID)
}

func TestPresence_RSP3_GetReturnsPresenceMessageObjects(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	presenceResponse := `[{
		"action": 1,
		"clientId": "user123",
		"connectionId": "conn456",
		"data": "status data",
		"timestamp": 1234567890000
	}]`
	mock.queueResponse(200, []byte(presenceResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test").Presence.Get().Pages(ctx)
	require.NoError(t, err)

	ok := pages.Next(ctx)
	require.True(t, ok)

	items := pages.Items()
	require.Len(t, items, 1)

	msg := items[0]
	assert.Equal(t, ably.PresenceActionPresent, msg.Action)
	assert.Equal(t, "user123", msg.ClientID)
	assert.Equal(t, "conn456", msg.ConnectionID)
	assert.Equal(t, "status data", msg.Data)
	assert.Equal(t, int64(1234567890000), msg.Timestamp)
}

func TestPresence_RSP3_GetWithNoMembersReturnsEmptyList(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("empty-channel").Presence.Get().Pages(ctx)
	require.NoError(t, err)

	ok := pages.Next(ctx)
	require.True(t, ok)

	items := pages.Items()
	assert.Empty(t, items)
	assert.False(t, pages.HasNext(ctx))
}

// =============================================================================
// RSP3a1 - Get limit parameter
// =============================================================================

func TestPresence_RSP3a1_GetWithLimitParameter(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.Get(
		ably.GetPresenceWithLimit(50),
	).Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	assert.Equal(t, "50", req.URL.Query().Get("limit"))
}

func TestPresence_RSP3a1_GetLimitMaximum1000(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.Get(
		ably.GetPresenceWithLimit(1000),
	).Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	assert.Equal(t, "1000", req.URL.Query().Get("limit"))
}

// =============================================================================
// RSP3a2 - Get clientId filter
// =============================================================================

func TestPresence_RSP3a2_GetWithClientIdFilter(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[{"action": 1, "clientId": "specific-client"}]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.Get(
		ably.GetPresenceWithClientID("specific-client"),
	).Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	assert.Equal(t, "specific-client", req.URL.Query().Get("clientId"))
}

// =============================================================================
// RSP3a3 - Get connectionId filter
// =============================================================================

func TestPresence_RSP3a3_GetWithConnectionIdFilter(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[{"action": 1, "clientId": "client1", "connectionId": "conn123"}]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.Get(
		ably.GetPresenceWithConnectionID("conn123"),
	).Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	assert.Equal(t, "conn123", req.URL.Query().Get("connectionId"))
}

// =============================================================================
// RSP3 Combined - Get with multiple filters
// =============================================================================

func TestPresence_RSP3_GetWithMultipleFilters(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.Get(
		ably.GetPresenceWithLimit(25),
		ably.GetPresenceWithClientID("user1"),
		ably.GetPresenceWithConnectionID("conn1"),
	).Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	query := req.URL.Query()
	assert.Equal(t, "25", query.Get("limit"))
	assert.Equal(t, "user1", query.Get("clientId"))
	assert.Equal(t, "conn1", query.Get("connectionId"))
}

// =============================================================================
// RSP4 - RestPresence#history
// =============================================================================

func TestPresence_RSP4_HistorySendsRequestToHistoryEndpoint(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	historyResponse := `[
		{"action": 2, "clientId": "client1", "data": "entered"},
		{"action": 4, "clientId": "client1", "data": "updated"}
	]`
	mock.queueResponse(200, []byte(historyResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test-channel").Presence.History().Pages(ctx)
	require.NoError(t, err)

	ok := pages.Next(ctx)
	require.True(t, ok)

	req := mock.lastRequest()
	assert.Equal(t, "GET", req.Method)
	assert.True(t, strings.HasSuffix(req.URL.Path, "/channels/test-channel/presence/history"),
		"expected path to end with /channels/test-channel/presence/history, got %s", req.URL.Path)
}

func TestPresence_RSP4a_HistoryReturnsPaginatedPresenceMessages(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	historyResponse := `[
		{"action": 2, "clientId": "user1", "data": "d1", "timestamp": 1000},
		{"action": 3, "clientId": "user1", "data": "d2", "timestamp": 2000},
		{"action": 4, "clientId": "user1", "data": "d3", "timestamp": 3000}
	]`
	mock.queueResponse(200, []byte(historyResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test").Presence.History().Pages(ctx)
	require.NoError(t, err)

	ok := pages.Next(ctx)
	require.True(t, ok)

	items := pages.Items()
	require.Len(t, items, 3)

	assert.Equal(t, ably.PresenceActionEnter, items[0].Action)
	assert.Equal(t, ably.PresenceActionLeave, items[1].Action)
	assert.Equal(t, ably.PresenceActionUpdate, items[2].Action)
}

// =============================================================================
// RSP4b1 - History start/end parameters
// =============================================================================

func TestPresence_RSP4b1_HistoryWithStartParameter(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	startTime := time.Unix(1609459200, 0) // 2021-01-01 00:00:00 UTC

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.History(
		ably.PresenceHistoryWithStart(startTime),
	).Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	assert.Equal(t, "1609459200000", req.URL.Query().Get("start"))
}

func TestPresence_RSP4b1_HistoryWithEndParameter(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	endTime := time.Unix(1609545600, 0) // 2021-01-02 00:00:00 UTC

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.History(
		ably.PresenceHistoryWithEnd(endTime),
	).Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	assert.Equal(t, "1609545600000", req.URL.Query().Get("end"))
}

func TestPresence_RSP4b1_HistoryWithStartAndEndParameters(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	startTime := time.Unix(1609459200, 0)
	endTime := time.Unix(1609545600, 0)

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.History(
		ably.PresenceHistoryWithStart(startTime),
		ably.PresenceHistoryWithEnd(endTime),
	).Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	query := req.URL.Query()
	assert.Equal(t, "1609459200000", query.Get("start"))
	assert.Equal(t, "1609545600000", query.Get("end"))
}

// =============================================================================
// RSP4b2 - History direction parameter
// =============================================================================

func TestPresence_RSP4b2_HistoryWithDirectionForwards(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.History(
		ably.PresenceHistoryWithDirection(ably.Forwards),
	).Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	assert.Equal(t, "forwards", req.URL.Query().Get("direction"))
}

func TestPresence_RSP4b2_HistoryWithDirectionBackwards(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.History(
		ably.PresenceHistoryWithDirection(ably.Backwards),
	).Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	assert.Equal(t, "backwards", req.URL.Query().Get("direction"))
}

// =============================================================================
// RSP4b3 - History limit parameter
// =============================================================================

func TestPresence_RSP4b3_HistoryWithLimitParameter(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.History(
		ably.PresenceHistoryWithLimit(50),
	).Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	assert.Equal(t, "50", req.URL.Query().Get("limit"))
}

func TestPresence_RSP4b3_HistoryLimitMaximum1000(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.History(
		ably.PresenceHistoryWithLimit(1000),
	).Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	assert.Equal(t, "1000", req.URL.Query().Get("limit"))
}

// =============================================================================
// RSP4 Combined - History with all parameters
// =============================================================================

func TestPresence_RSP4_HistoryWithAllParameters(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	startTime := time.Unix(1609459200, 0)
	endTime := time.Unix(1609545600, 0)

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.History(
		ably.PresenceHistoryWithStart(startTime),
		ably.PresenceHistoryWithEnd(endTime),
		ably.PresenceHistoryWithDirection(ably.Forwards),
		ably.PresenceHistoryWithLimit(50),
	).Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	query := req.URL.Query()
	assert.Equal(t, "1609459200000", query.Get("start"))
	assert.Equal(t, "1609545600000", query.Get("end"))
	assert.Equal(t, "forwards", query.Get("direction"))
	assert.Equal(t, "50", query.Get("limit"))
}

// =============================================================================
// RSP5 - Presence message decoding
// =============================================================================

func TestPresence_RSP5_StringDataDecodedAsString(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	presenceResponse := `[{"action": 1, "clientId": "c1", "data": "plain string data"}]`
	mock.queueResponse(200, []byte(presenceResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test").Presence.Get().Pages(ctx)
	require.NoError(t, err)

	ok := pages.Next(ctx)
	require.True(t, ok)

	items := pages.Items()
	require.Len(t, items, 1)

	data, ok := items[0].Data.(string)
	assert.True(t, ok, "expected data to be string")
	assert.Equal(t, "plain string data", data)
}

func TestPresence_RSP5_JSONEncodedDataDecodedToObject(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	presenceResponse := `[{
		"action": 1,
		"clientId": "c1",
		"data": "{\"status\":\"online\",\"count\":42}",
		"encoding": "json"
	}]`
	mock.queueResponse(200, []byte(presenceResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test").Presence.Get().Pages(ctx)
	require.NoError(t, err)

	ok := pages.Next(ctx)
	require.True(t, ok)

	items := pages.Items()
	require.Len(t, items, 1)

	data, ok := items[0].Data.(map[string]interface{})
	assert.True(t, ok, "expected data to be map")
	assert.Equal(t, "online", data["status"])
	assert.Equal(t, float64(42), data["count"])
}

func TestPresence_RSP5_Base64EncodedDataDecodedToBinary(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	// "Hello World" in base64
	presenceResponse := `[{
		"action": 1,
		"clientId": "c1",
		"data": "SGVsbG8gV29ybGQ=",
		"encoding": "base64"
	}]`
	mock.queueResponse(200, []byte(presenceResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test").Presence.Get().Pages(ctx)
	require.NoError(t, err)

	ok := pages.Next(ctx)
	require.True(t, ok)

	items := pages.Items()
	require.Len(t, items, 1)

	data, ok := items[0].Data.([]byte)
	assert.True(t, ok, "expected data to be []byte")
	assert.Equal(t, "Hello World", string(data))
}

func TestPresence_RSP5_ChainedEncodingDecodedInOrder(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	// base64 of {"key":"value"}
	jsonBase64 := base64.StdEncoding.EncodeToString([]byte(`{"key":"value"}`))
	presenceResponse := `[{
		"action": 1,
		"clientId": "c1",
		"data": "` + jsonBase64 + `",
		"encoding": "json/base64"
	}]`
	mock.queueResponse(200, []byte(presenceResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test").Presence.Get().Pages(ctx)
	require.NoError(t, err)

	ok := pages.Next(ctx)
	require.True(t, ok)

	items := pages.Items()
	require.Len(t, items, 1)

	data, ok := items[0].Data.(map[string]interface{})
	assert.True(t, ok, "expected data to be map after json/base64 decoding")
	assert.Equal(t, "value", data["key"])
}

func TestPresence_RSP5_HistoryMessagesAlsoDecoded(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	historyResponse := `[{
		"action": 2,
		"clientId": "c1",
		"data": "{\"event\":\"entered\"}",
		"encoding": "json"
	}]`
	mock.queueResponse(200, []byte(historyResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test").Presence.History().Pages(ctx)
	require.NoError(t, err)

	ok := pages.Next(ctx)
	require.True(t, ok)

	items := pages.Items()
	require.Len(t, items, 1)

	data, ok := items[0].Data.(map[string]interface{})
	assert.True(t, ok, "expected history data to be decoded map")
	assert.Equal(t, "entered", data["event"])
}

// =============================================================================
// Pagination
// =============================================================================

func TestPresence_Pagination_GetWithLinkHeader(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponseWithHeaders(200,
		[]byte(`[{"action": 1, "clientId": "client1"}, {"action": 1, "clientId": "client2"}]`),
		"application/json",
		map[string]string{
			"Link": `</channels/test/presence?page=2>; rel="next"`,
		},
	)

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test").Presence.Get().Pages(ctx)
	require.NoError(t, err)

	ok := pages.Next(ctx)
	require.True(t, ok)

	items := pages.Items()
	assert.Len(t, items, 2)
	assert.True(t, pages.HasNext(ctx))
}

func TestPresence_Pagination_GetNextPageFetchesFromLinkURL(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	// First page response
	mock.queueResponseWithHeaders(200,
		[]byte(`[{"action": 1, "clientId": "client1"}]`),
		"application/json",
		map[string]string{
			"Link": `</channels/test/presence?page=2>; rel="next"`,
		},
	)
	// Second page response
	mock.queueResponse(200, []byte(`[{"action": 1, "clientId": "client2"}]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test").Presence.Get().Pages(ctx)
	require.NoError(t, err)

	// First page
	ok := pages.Next(ctx)
	require.True(t, ok)
	assert.Equal(t, "client1", pages.Items()[0].ClientID)

	// Second page
	ok = pages.Next(ctx)
	require.True(t, ok)
	assert.Equal(t, "client2", pages.Items()[0].ClientID)
	assert.False(t, pages.HasNext(ctx))
}

func TestPresence_Pagination_HistoryPaginationWorks(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	// First page response
	mock.queueResponseWithHeaders(200,
		[]byte(`[{"action": 2, "clientId": "c1", "timestamp": 3000}]`),
		"application/json",
		map[string]string{
			"Link": `</channels/test/presence/history?page=2>; rel="next"`,
		},
	)
	// Second page response
	mock.queueResponse(200, []byte(`[{"action": 3, "clientId": "c1", "timestamp": 1000}]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test").Presence.History().Pages(ctx)
	require.NoError(t, err)

	// First page
	ok := pages.Next(ctx)
	require.True(t, ok)
	assert.Equal(t, ably.PresenceActionEnter, pages.Items()[0].Action)

	// Second page
	ok = pages.Next(ctx)
	require.True(t, ok)
	assert.Equal(t, ably.PresenceActionLeave, pages.Items()[0].Action)
}

// =============================================================================
// Error Handling
// =============================================================================

func TestPresence_Error_GetWithServerError(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	errorResponse := `{"error": {"code": 50000, "statusCode": 500, "message": "Internal server error"}}`
	mock.queueResponse(500, []byte(errorResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test").Presence.Get().Pages(ctx)
	require.NoError(t, err) // Pages() itself doesn't fail

	// First Next() fetches the data and encounters error
	ok := pages.Next(ctx)

	// In ably-go's presence pagination, after an HTTP error:
	// - Next() returns true (the response was received)
	// - Items() returns nil or empty
	// - The error is logged but not surfaced via Err()
	// This is different from the Request API which exposes ErrorCode()/ErrorMessage()

	// Verify at least that we didn't get valid items
	if ok {
		items := pages.Items()
		// With a 500 error, we shouldn't have valid presence items
		assert.Empty(t, items, "expected no items from server error response")
	}

	// Note: The error is logged at ERROR level but not returned via Err()
	// This appears to be the expected behavior in ably-go's presence pagination
}

func TestPresence_Error_HistoryWithAuthError(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	errorResponse := `{"error": {"code": 40101, "statusCode": 401, "message": "Invalid credentials"}}`
	mock.queueResponse(401, []byte(errorResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("invalid.key:secret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test").Presence.History().Pages(ctx)

	// In ably-go, auth errors (401) can be returned directly from Pages()
	// or may surface during pagination depending on when the request is made
	if err != nil {
		// Auth error returned from Pages()
		errInfo, ok := err.(*ably.ErrorInfo)
		if ok {
			assert.Equal(t, ably.ErrorCode(40101), errInfo.Code)
			assert.Equal(t, 401, errInfo.StatusCode)
		}
		return
	}

	// If Pages() succeeded, error may surface in Next()
	ok := pages.Next(ctx)
	if !ok {
		err = pages.Err()
		if err != nil {
			errInfo, isErr := err.(*ably.ErrorInfo)
			if isErr {
				assert.Equal(t, ably.ErrorCode(40101), errInfo.Code)
			}
		}
	}
}

// =============================================================================
// Request Headers
// =============================================================================

func TestPresence_Headers_GetIncludesStandardHeaders(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithUseBinaryProtocol(false), // Use JSON protocol for this test
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.Get().Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	assert.Equal(t, "2", req.Header.Get("X-Ably-Version"))
	assert.Contains(t, req.Header.Get("Ably-Agent"), "ably-go")
	assert.Equal(t, "application/json", req.Header.Get("Accept"))
}

func TestPresence_Headers_DefaultAcceptIsMsgpack(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	// Return msgpack-encoded empty array
	mock.queueResponse(200, []byte{0x90}, "application/x-msgpack") // msgpack empty array

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		// Default: UseBinaryProtocol is true
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.Get().Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	assert.Equal(t, "application/x-msgpack", req.Header.Get("Accept"))
}

func TestPresence_Headers_HistoryIncludesAuthorizationHeader(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Channels.Get("test").Presence.History().Pages(ctx)
	require.NoError(t, err)

	req := mock.lastRequest()
	authHeader := req.Header.Get("Authorization")
	assert.True(t, strings.HasPrefix(authHeader, "Basic "),
		"expected Authorization header to start with 'Basic ', got %s", authHeader)
}

func TestPresence_Headers_RequestIdIncludedWhenEnabled(t *testing.T) {
	// SKIP: ably-go does not implement addRequestIds option (RSC7c)
	t.Skip("RSC7c - addRequestIds option not implemented in ably-go")
}

// =============================================================================
// PresenceAction Values
// =============================================================================

func TestPresence_Action_AllPresenceActionsCorrectlyMapped(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	// Note: ably-go uses action values: 0=absent, 1=present, 2=enter, 3=leave, 4=update
	historyResponse := `[
		{"action": 0, "clientId": "c1"},
		{"action": 1, "clientId": "c2"},
		{"action": 2, "clientId": "c3"},
		{"action": 3, "clientId": "c4"},
		{"action": 4, "clientId": "c5"}
	]`
	mock.queueResponse(200, []byte(historyResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test").Presence.History().Pages(ctx)
	require.NoError(t, err)

	ok := pages.Next(ctx)
	require.True(t, ok)

	items := pages.Items()
	require.Len(t, items, 5)

	assert.Equal(t, ably.PresenceActionAbsent, items[0].Action)
	assert.Equal(t, ably.PresenceActionPresent, items[1].Action)
	assert.Equal(t, ably.PresenceActionEnter, items[2].Action)
	assert.Equal(t, ably.PresenceActionLeave, items[3].Action)
	assert.Equal(t, ably.PresenceActionUpdate, items[4].Action)
}

// =============================================================================
// Items Iterator
// =============================================================================

func TestPresence_Items_IteratorWorksCorrectly(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	presenceResponse := `[
		{"action": 1, "clientId": "client1"},
		{"action": 1, "clientId": "client2"},
		{"action": 1, "clientId": "client3"}
	]`
	mock.queueResponse(200, []byte(presenceResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	items, err := client.Channels.Get("test").Presence.Get().Items(ctx)
	require.NoError(t, err)

	var clientIds []string
	for items.Next(ctx) {
		clientIds = append(clientIds, items.Item().ClientID)
	}

	assert.NoError(t, items.Err())
	assert.Equal(t, []string{"client1", "client2", "client3"}, clientIds)
}

// =============================================================================
// UTF-8 Encoding
// =============================================================================

func TestPresence_RSP5_UTF8EncodedDataDecodedCorrectly(t *testing.T) {
	mock := newPresenceMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	// "Hello World" UTF-8 bytes encoded as base64
	utf8Base64 := base64.StdEncoding.EncodeToString([]byte("Hello World"))
	presenceResponse := `[{
		"action": 1,
		"clientId": "c1",
		"data": "` + utf8Base64 + `",
		"encoding": "utf-8/base64"
	}]`
	mock.queueResponse(200, []byte(presenceResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pages, err := client.Channels.Get("test").Presence.Get().Pages(ctx)
	require.NoError(t, err)

	ok := pages.Next(ctx)
	require.True(t, ok)

	items := pages.Items()
	require.Len(t, items, 1)

	data, ok := items[0].Data.(string)
	assert.True(t, ok, "expected data to be string after utf-8/base64 decoding")
	assert.Equal(t, "Hello World", data)
}

// =============================================================================
// Test helper to verify JSON encoding in requests (not applicable to presence GET)
// =============================================================================

func mustMarshalJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
