package ably_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
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
	assert.Equal(t, 400, err.StatusCode, fmt.Sprintf("want StatusCode=400; got %d", err.StatusCode))
	assert.Equal(t, 40005, err.Code, fmt.Sprintf("want Code=40005; got %d", err.Code))
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

func TestIssue_154(t *testing.T) {
	var retry atomic.Value
	incrRetry := func() {
		if v := retry.Load(); v != nil {
			retry.Store(v.(int) + 1)
		} else {
			retry.Store(int(1))
		}
	}
	count := func() int {
		if v := retry.Load(); v != nil {
			return v.(int)
		}
		return 0
	}
	ref := "Mon Jan 2 15:04:05 MST 2006"
	errMsg := "This is an html error body"
	ts, err := time.Parse(time.UnixDate, ref)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		incrRetry()
		rw.Header().Set("Content-Type", "text/html")
		rw.WriteHeader(http.StatusBadGateway)
		rw.Write([]byte(fmt.Sprintf("<html><head></head><body>%s</body></html>", errMsg)))
	}))
	var d time.Duration
	d.Milliseconds()
	defer server.Close()
	defaultServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode([]int64{ts.UnixNano() / int64(time.Millisecond)})
	}))
	defer defaultServer.Close()
	fallbackHosts := []string{"fallback0", "fallback1", "fallback2"}
	opts := &ably.ClientOptions{
		Environment:   ablytest.Environment,
		NoTLS:         true,
		FallbackHosts: fallbackHosts,
		AuthOptions: ably.AuthOptions{
			UseTokenAuth: true,
		},
	}
	serverURL, _ := url.Parse(server.URL)
	defaultURL, _ := url.Parse(defaultServer.URL)
	proxy := func(r *http.Request) (*url.URL, error) {
		retryCount := count()
		if retryCount < 2 {
			return serverURL, nil
		} else {
			r.Host = defaultURL.Hostname()
			return defaultURL, nil
		}
	}

	opts.HTTPClient = &http.Client{
		Transport: &http.Transport{
			Proxy:        proxy,
			TLSNextProto: map[string]func(authority string, c *tls.Conn) http.RoundTripper{},
		},
	}
	client, err := ably.NewRestClient(opts)
	if err != nil {
		t.Fatal(err)
	}
	tym, err := client.Time()
	if err != nil {
		t.Fatalf("didn't expect error got %v ", err)
	}
	if !tym.Equal(ts) {
		t.Errorf("expected %v got %v", ts, tym)
	}
	c := count()
	if c != 2 {
		t.Errorf("expected 2 retries got %d", c)
	}
}
