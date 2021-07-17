package ably_test

import (
	"context"
	"errors"
	"fmt"
	"net"
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
	opts := []ably.ClientOption{ably.WithKey(":")}
	_, e := ably.NewREST(opts...)
	if e == nil {
		t.Fatal("NewREST(): expected err != nil")
	}
	err, ok := e.(*ably.ErrorInfo)
	assert.True(t, ok, fmt.Sprintf("want e be *ably.Error; was %T", e))
	assert.Equal(t, 400, err.StatusCode, fmt.Sprintf("want StatusCode=400; got %d", err.StatusCode))
	assert.Equal(t, ably.ErrInvalidCredential, err.Code, fmt.Sprintf("want Code=	; got %d", err.Code))
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
	opts := []ably.ClientOption{
		ably.WithKey("xxxxxxx.yyyyyyy:zzzzzzz"),
		ably.WithTLS(false),
		ably.WithUseTokenAuth(true),
		ably.WithRESTHost(endpointURL.Hostname()),
	}
	port, _ := strconv.ParseInt(endpointURL.Port(), 10, 0)
	opts = append(opts, ably.WithPort(int(port)))
	client, e := ably.NewREST(opts...)
	assert.Nil(t, e)

	_, err = client.Time(context.Background())
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), errMsg)
}

func TestErrorInfo(t *testing.T) {
	t.Run("without an error code", func(t *testing.T) {
		t.Parallel()

		e := &ably.ErrorInfo{
			StatusCode: 401,
		}
		h := "help.ably.io"
		if strings.Contains(e.Error(), h) {
			t.Errorf("expected error message not to contain %s", h)
		}
	})
	t.Run("with an error code", func(t *testing.T) {
		t.Parallel()

		e := &ably.ErrorInfo{
			Code: 44444,
		}
		h := "https://help.ably.io/error/44444"
		if !strings.Contains(e.Error(), h) {
			t.Errorf("expected error message %s  to contain %s", e.Error(), h)
		}
	})
	t.Run("with an error code and an href attribute", func(t *testing.T) {
		t.Parallel()

		href := "http://foo.bar.com/"
		e := &ably.ErrorInfo{
			Code: 44444,
			HRef: href,
		}
		h := "https://help.ably.io/error/44444"
		if strings.Contains(e.Error(), h) {
			t.Errorf("expected error message %s not to contain %s", e.Error(), h)
		}
		if !strings.Contains(e.Error(), href) {
			t.Errorf("expected error message %s  to contain %s", e.Error(), href)
		}
	})

	t.Run("with an error code and a message with the same error URL", func(t *testing.T) {
		t.Parallel()

		e := ably.NewErrorInfo(44444, errors.New("error https://help.ably.io/error/44444"))
		h := "https://help.ably.io/error/44444"
		if !strings.Contains(e.Error(), h) {
			t.Errorf("expected error message %s  to contain %s", e.Error(), h)
		}
		n := strings.Count(e.Error(), h)
		if n != 1 {
			t.Errorf("expected 1 occupance of %s got %d", h, n)
		}
	})
	t.Run("with an error code and a message with a different error URL", func(t *testing.T) {
		t.Parallel()

		e := ably.NewErrorInfo(44444, errors.New("error https://help.ably.io/error/123123"))
		h := "https://help.ably.io/error/44444"
		if !strings.Contains(e.Error(), h) {
			t.Errorf("expected error message %s  to contain %s", e.Error(), h)
		}
		n := strings.Count(e.Error(), "help.ably.io")
		if n != 2 {
			t.Errorf("expected 2 got %d", n)
		}
		n = strings.Count(e.Error(), "/123123")
		if n != 1 {
			t.Errorf("expected 1 got %d", n)
		}
		n = strings.Count(e.Error(), "/44444")
		if n != 1 {
			t.Errorf("expected 1 got %d", n)
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
	opts := []ably.ClientOption{
		ably.WithKey("xxxxxxx.yyyyyyy:zzzzzzz"),
		ably.WithTLS(false),
		ably.WithUseTokenAuth(true),
		ably.WithRESTHost(endpointURL.Hostname()),
	}
	port, _ := strconv.ParseInt(endpointURL.Port(), 10, 0)
	opts = append(opts, ably.WithPort(int(port)))
	client, e := ably.NewREST(opts...)
	assert.Nil(t, e)

	_, err = client.Time(context.Background())
	assert.NotNil(t, err)
	et := err.(*ably.ErrorInfo)
	if et.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected %d got %d: %v", http.StatusMethodNotAllowed, et.StatusCode, err)
	}
}

func Test_DNSOrTimeoutErr(t *testing.T) {
	dnsErr := net.DNSError{
		Err:         "Can't resolve host",
		Name:        "Host unresolvable",
		Server:      "rest.ably.com",
		IsTimeout:   false,
		IsTemporary: false,
		IsNotFound:  false,
	}

	WrappedDNSErr := fmt.Errorf("custom error occured %w", &dnsErr)
	if !ably.IsTimeoutOrDnsErr(WrappedDNSErr) {
		t.Fatalf("%v is a DNS error", WrappedDNSErr)
	}

	urlErr := url.Error{
		URL: "rest.ably.io",
		Err: errors.New("URL error occured"),
		Op:  "IO read OP",
	}

	if ably.IsTimeoutOrDnsErr(&urlErr) {
		t.Fatalf("%v is not a DNS or timeout error", WrappedDNSErr)
	}

	urlErr.Err = &dnsErr

	if !ably.IsTimeoutOrDnsErr(WrappedDNSErr) {
		t.Fatalf("%v is a DNS error", WrappedDNSErr)
	}

	dnsErr.IsTimeout = true

	if !ably.IsTimeoutOrDnsErr(WrappedDNSErr) {
		t.Fatalf("%v is a timeout error", WrappedDNSErr)
	}
}
