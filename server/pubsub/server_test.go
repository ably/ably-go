package pubsub_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ably/ably-pubsub-go/internal/ably"
	"github.com/ably/ably-pubsub-go/server/pubsub"
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

	client, err := pubsub.NewHTTPClient(opts...)
	require.NoError(t, err)

	client.Time(context.Background())
	assert.Equal(t, ably.AgentIdentifier(map[string]string{"ably-pubsub-server": ""}), *agent)
	// Literal, not derived: the family and side identifiers are what Ably's
	// traffic classification keys on, so a change to either must fail here.
	assert.True(t, strings.HasPrefix(*agent, "ably-pubsub-go/"), "wrong SDK family: %q", *agent)
	assert.True(t, strings.HasSuffix(*agent, " ably-pubsub-server"), "missing side agent: %q", *agent)
}

func TestNewHTTPClient_MergesUserAgents(t *testing.T) {
	agent, opts := newAgentRecorder(t)

	client, err := pubsub.NewHTTPClient(append(opts, ably.WithAgents(map[string]string{"foo": "1.2.3"}))...)
	require.NoError(t, err)

	client.Time(context.Background())
	// Agents are sorted by product name, so the order is fixed.
	assert.True(t, strings.HasSuffix(*agent, " ably-pubsub-server foo/1.2.3"), "unexpected agents: %q", *agent)
}

func TestNewHTTPClient_UserCannotOverrideSideAgent(t *testing.T) {
	agent, opts := newAgentRecorder(t)

	client, err := pubsub.NewHTTPClient(append(opts, ably.WithAgents(map[string]string{"ably-pubsub-server": "9.9"}))...)
	require.NoError(t, err)

	client.Time(context.Background())
	// The package's own agent replaces any version the caller passes.
	assert.True(t, strings.HasSuffix(*agent, " ably-pubsub-server"), "side agent not bare: %q", *agent)
	assert.False(t, strings.Contains(*agent, "ably-pubsub-server/"), "side agent carries a version: %q", *agent)
}

func TestNewRealtimeClient_DeclaresServerAgent(t *testing.T) {
	// Record the Ably-Agent header of the first WebSocket handshake.
	agents := make(chan string, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case agents <- r.Header.Get(ably.AblyAgentHeaderName):
		default:
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)

	client, err := pubsub.NewRealtimeClient(
		ably.WithEndpoint(u.Host),
		ably.WithTLS(false),
		ably.WithToken("fake:token"),
	)
	require.NoError(t, err)
	t.Cleanup(client.Close)

	select {
	case agent := <-agents:
		assert.Equal(t, ably.AgentIdentifier(map[string]string{"ably-pubsub-server": ""}), agent)
		assert.True(t, strings.HasPrefix(agent, "ably-pubsub-go/"), "wrong SDK family: %q", agent)
		assert.True(t, strings.HasSuffix(agent, " ably-pubsub-server"), "missing side agent: %q", agent)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the realtime connection attempt")
	}
}
