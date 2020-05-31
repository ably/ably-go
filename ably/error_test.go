package ably_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
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
	err, ok := e.(*ably.ErrorInfo)
	assert.True(t, ok, fmt.Sprintf("want e be *ably.Error; was %T", e))
	assert.Equal(t, 400, err.StatusCode, fmt.Sprintf("want StatusCode=400; got %d", err.StatusCode))
	assert.Equal(t, 40005, err.Code, fmt.Sprintf("want Code=40005; got %d", err.Code))
	assert.NotNil(t, err.Unwrap())
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

func TestErrorInfo(t *testing.T) {
	t.Run("without an error code", func(ts *testing.T) {
		e := &ably.ErrorInfo{
			StatusCode: 401,
		}
		h := "help.ably.io"
		if strings.Contains(e.Error(), h) {
			ts.Errorf("expected error message not to contain %s", h)
		}
	})
	t.Run("with an error code", func(ts *testing.T) {
		e := &ably.ErrorInfo{
			Code: 44444,
		}
		h := "https://help.ably.io/error/44444"
		if !strings.Contains(e.Error(), h) {
			ts.Errorf("expected error message %s  to contain %s", e.Error(), h)
		}
	})
	t.Run("with an error code and an href attribute", func(ts *testing.T) {
		href := "http://foo.bar.com/"
		e := &ably.ErrorInfo{
			Code: 44444,
			HRef: href,
		}
		h := "https://help.ably.io/error/44444"
		if strings.Contains(e.Error(), h) {
			ts.Errorf("expected error message %s not to contain %s", e.Error(), h)
		}
		if !strings.Contains(e.Error(), href) {
			ts.Errorf("expected error message %s  to contain %s", e.Error(), href)
		}
	})

	t.Run("with an error code and a message with the same error URL", func(ts *testing.T) {
		e := ably.NewErrorInfo(44444, errors.New("error https://help.ably.io/error/44444"))
		h := "https://help.ably.io/error/44444"
		if !strings.Contains(e.Error(), h) {
			ts.Errorf("expected error message %s  to contain %s", e.Error(), h)
		}
		n := strings.Count(e.Error(), h)
		if n != 1 {
			ts.Errorf("expected 1 occupance of %s got %d", h, n)
		}
	})
	t.Run("with an error code and a message with a different error URL", func(ts *testing.T) {
		e := ably.NewErrorInfo(44444, errors.New("error https://help.ably.io/error/123123"))
		h := "https://help.ably.io/error/44444"
		if !strings.Contains(e.Error(), h) {
			ts.Errorf("expected error message %s  to contain %s", e.Error(), h)
		}
		n := strings.Count(e.Error(), "help.ably.io")
		if n != 2 {
			ts.Errorf("expected 2 got %d", n)
		}
		n = strings.Count(e.Error(), "/123123")
		if n != 1 {
			ts.Errorf("expected 1 got %d", n)
		}
		n = strings.Count(e.Error(), "/44444")
		if n != 1 {
			ts.Errorf("expected 1 got %d", n)
		}
	})
}

func TestIssue_154(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "text/html")
		rw.WriteHeader(http.StatusMethodNotAllowed)
	}))
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
	et := err.(*ably.ErrorInfo)
	if et.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected %d got %d", http.StatusMethodNotAllowed, et.StatusCode)
	}
}
