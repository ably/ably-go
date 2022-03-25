//go:build !integration

package ably_test

import (
	"context"
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
	opts := []ably.ClientOption{ably.WithKey(":")}
	_, e := ably.NewREST(opts...)
	assert.Error(t, e,
		"NewREST(): expected err != nil")
	err, ok := e.(*ably.ErrorInfo)
	assert.True(t, ok,
		"want e be *ably.Error; was %T", e)
	assert.Equal(t, 400, err.StatusCode,
		"want StatusCode=400; got %d", err.StatusCode)
	assert.Equal(t, ably.ErrInvalidCredential, err.Code,
		"want Code=40005; got %d", err.Code)
	assert.NotNil(t, err.Unwrap())
}

func TestIssue127ErrorResponse(t *testing.T) {
	// Start a local HTTP server
	errMsg := "This is an html error body"
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Test request parameters
		assert.Equal(t, "/time", req.URL.String())
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
	client, err := ably.NewREST(opts...)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errMsg)
}

func TestErrorInfo(t *testing.T) {
	t.Run("without an error code", func(t *testing.T) {

		e := &ably.ErrorInfo{
			StatusCode: 401,
		}
		assert.NotContains(t, e.Error(), "help.ably.io",
			"expected error message not to contain \"help.ably.io\"")
	})
	t.Run("with an error code", func(t *testing.T) {

		e := &ably.ErrorInfo{
			Code: 44444,
		}
		assert.Contains(t, e.Error(), "https://help.ably.io/error/44444",
			"expected error message %s to contain \"https://help.ably.io/error/44444\"", e.Error())
	})
	t.Run("with an error code and an href attribute", func(t *testing.T) {

		e := &ably.ErrorInfo{
			Code: 44444,
			HRef: "http://foo.bar.com/",
		}
		assert.NotContains(t, e.Error(), "https://help.ably.io/error/44444",
			"expected error message %s not to contain \"https://help.ably.io/error/44444\"", e.Error())
		assert.Contains(t, e.Error(), "http://foo.bar.com/",
			"expected error message %s to contain \"http://foo.bar.com/\"", e.Error())
	})

	t.Run("with an error code and a message with the same error URL", func(t *testing.T) {

		e := ably.NewErrorInfo(44444, errors.New("error https://help.ably.io/error/44444"))
		assert.Contains(t, e.Error(), "https://help.ably.io/error/44444",
			"expected error message %s to contain \"https://help.ably.io/error/44444\"", e.Error())

		count := strings.Count(e.Error(), "https://help.ably.io/error/44444")
		assert.Equal(t, 1, count, "expected 1 occurrence of \"https://help.ably.io/error/44444\" got %d", count)

	})
	t.Run("with an error code and a message with a different error URL", func(t *testing.T) {

		e := ably.NewErrorInfo(44444, errors.New("error https://help.ably.io/error/123123"))
		assert.Contains(t, e.Error(), "https://help.ably.io/error/44444",
			"expected error message %s to contain \"https://help.ably.io/error/44444\"", e.Error())
		count := strings.Count(e.Error(), "help.ably.io")
		assert.Equal(t, 2, count,
			"expected 2 occurrences of \"help.ably.io\" got %d", count)
		count = strings.Count(e.Error(), "/123123")
		assert.Equal(t, 1, count,
			"expected 1 occurence of \"/123123\" got %d", count)
		count = strings.Count(e.Error(), "/44444")
		assert.Equal(t, 1, count, "expected 1 occurence of \"/44444\" got %d", count)
	})
}

func TestIssue_154(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "text/html")
		rw.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	endpointURL, err := url.Parse(server.URL)
	assert.NoError(t, err)
	opts := []ably.ClientOption{
		ably.WithKey("xxxxxxx.yyyyyyy:zzzzzzz"),
		ably.WithTLS(false),
		ably.WithUseTokenAuth(true),
		ably.WithRESTHost(endpointURL.Hostname()),
	}
	port, _ := strconv.ParseInt(endpointURL.Port(), 10, 0)
	opts = append(opts, ably.WithPort(int(port)))
	client, e := ably.NewREST(opts...)
	assert.NoError(t, e)

	_, err = client.Time(context.Background())
	assert.Error(t, err)
	et := err.(*ably.ErrorInfo)
	assert.Equal(t, http.StatusMethodNotAllowed, et.StatusCode,
		"expected %d got %d: %v", http.StatusMethodNotAllowed, et.StatusCode, err)
}
