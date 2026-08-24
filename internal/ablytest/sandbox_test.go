//go:build !integration
// +build !integration

package ablytest

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestLoopbackEndpoint(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
		endpoint, err := loopbackEndpoint(host, 7100)
		require.NoError(t, err)
		assert.Equal(t, net.JoinHostPort(host, "7100"), endpoint)
	}

	_, err := loopbackEndpoint("example.com", 7100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not loopback")
	_, err = loopbackEndpoint("127.0.0.1", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port 0 is invalid")
}

func TestLocalPlaintextTransportRoutesAllowedHostsToLoopback(t *testing.T) {
	t.Setenv("ABLY_LOCAL_REST_HOSTS", "primary.example")
	original, err := http.NewRequest(http.MethodGet, "https://primary.example/path", nil)
	require.NoError(t, err)

	transport := localPlaintextTransport{
		endpoint: "127.0.0.1:7100",
		base: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			assert.Equal(t, "http://127.0.0.1:7100/path", request.URL.String())
			assert.Equal(t, "primary.example", request.Host)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Request:    request,
			}, nil
		}),
	}

	response, err := transport.RoundTrip(original)
	require.NoError(t, err)
	assert.Same(t, original, response.Request)
}

func TestLocalPlaintextTransportRejectsUnlistedHosts(t *testing.T) {
	t.Setenv("ABLY_LOCAL_REST_HOSTS", "primary.example")
	request, err := http.NewRequest(http.MethodGet, "https://external.example/path", nil)
	require.NoError(t, err)

	transport := localPlaintextTransport{base: http.DefaultTransport, endpoint: "127.0.0.1:7100"}
	_, err = transport.RoundTrip(request)
	var dnsError *net.DNSError
	assert.ErrorAs(t, err, &dnsError)
}

func TestRoundTripRecorderWrapsAnyTransport(t *testing.T) {
	recorder := &RoundTripRecorder{}
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})
	recorder.Hijack(transport)

	request, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)
	response, err := recorder.RoundTrip(request)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, response.StatusCode)
	assert.Equal(t, 1, recorder.Len())
}

func TestRoundTripRecorderHandlesTransportErrorsWithoutAResponse(t *testing.T) {
	recorder := &RoundTripRecorder{}
	recorder.Hijack(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, assert.AnError
	}))

	request, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)
	response, err := recorder.RoundTrip(request)
	assert.Nil(t, response)
	assert.Equal(t, assert.AnError, err)
	assert.Equal(t, 1, recorder.Len())
	assert.Nil(t, recorder.Response(0))
}
