package ably_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
)

func TestErrorResponseWithInvalidKey(t *testing.T) {
	opts := ably.NewClientOptions(":")
	_, e := ably.NewRestClient(opts)
	if e == nil {
		t.Fatal("NewRestClient(): expected err != nil")
	}
	err, ok := e.(*ably.Error)
	assert.True(t, ok, fmt.Sprintf("want e be *ably.Error; was %T", e))
	assert.NotEqual(t, 400, err.StatusCode, fmt.Sprintf("want StatusCode=400; got %d", err.StatusCode))
	assert.NotEqual(t, 40005, err.Code, fmt.Sprintf("want Code=40005; got %d", err.Code))
	assert.NotNil(t, err.Err)
}

func TestIssue127ErrorResponse(t *testing.T) {
	// Start a local HTTP server
	errMsg := "This is an html error body"
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Test request parameters
		assert.Equal(t, req.URL.String(), "/time")
		// Send response to be tested
		rw.Header().Set("Content-Type", "text/html")
		rw.WriteHeader(400)
		rw.Write([]byte(fmt.Sprintf("<html><head></head><body>%s</body></html>", errMsg)))
	}))
	// Close the server when test finishes
	defer server.Close()

	endpointURL, err := url.Parse(server.URL)
	assert.Nil(t, err)
	opts := ably.NewClientOptions("xxxxxxx.yyyyyyy:zzzzzzz")
	opts.NoTLS = true
	opts.UseTokenAuth = true
	opts.RestHost = endpointURL.Hostname()
	port, _ := strconv.ParseInt(endpointURL.Port(), 10, 0)
	opts.Port = int(port)
	client, e := ably.NewRestClient(opts)
	assert.Nil(t, e)

	_, err = client.Time()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), errMsg)
}
