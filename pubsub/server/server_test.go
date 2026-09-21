package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ably/ably-go/internal/ably"
	"github.com/ably/ably-go/pubsub/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAgentRecorder returns a test server which records the Ably-Agent header
// of each request it receives, along with options pointing a client at it.
func newAgentRecorder(t *testing.T) (agent *string, opts []ably.ClientOption) {
	t.Helper()
	agent = new(string)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*agent = r.Header.Get(ably.AblyAgentHeaderName)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	return agent, []ably.ClientOption{
		ably.WithEndpoint(u.Host),
		ably.WithTLS(false),
		ably.WithUseTokenAuth(true),
	}
}

func TestNewHTTPClient_DeclaresServerAgent(t *testing.T) {
	agent, opts := newAgentRecorder(t)

	client, err := server.NewHTTPClient(opts...)
	require.NoError(t, err)

	client.Time(context.Background())
	assert.Equal(t, ably.AgentIdentifier(map[string]string{"ably-pubsub-server": ""}), *agent)
}

func TestNewHTTPClient_MergesUserAgents(t *testing.T) {
	agent, opts := newAgentRecorder(t)

	client, err := server.NewHTTPClient(append(opts, ably.WithAgents(map[string]string{"foo": "1.2.3"}))...)
	require.NoError(t, err)

	client.Time(context.Background())
	// Agent map iteration order is unspecified, so assert each entry.
	assert.True(t, strings.Contains(*agent, " ably-pubsub-server"), "missing side agent: %q", *agent)
	assert.True(t, strings.Contains(*agent, " foo/1.2.3"), "missing user agent: %q", *agent)
}
