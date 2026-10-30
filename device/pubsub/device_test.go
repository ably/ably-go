package pubsub_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ably/ably-pubsub-go/v2/device/pubsub"
	"github.com/ably/ably-pubsub-go/v2/internal/ably"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_DeclaresDeviceAgent(t *testing.T) {
	var agent string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent = r.Header.Get(ably.AblyAgentHeaderName)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)

	// The realtime client's HTTP requests carry the same agents as its
	// connection, so assert the header via Time without connecting.
	client, err := pubsub.NewClient(
		ably.WithEndpoint(u.Host),
		ably.WithTLS(false),
		ably.WithUseTokenAuth(true),
		ably.WithAutoConnect(false),
	)
	require.NoError(t, err)

	client.Time(context.Background())
	assert.Equal(t, ably.AgentIdentifier(map[string]string{"ably-pubsub-device": ""}), agent)
	// Literal, not derived: the family and side identifiers are what Ably's
	// traffic classification keys on, so a change to either must fail here.
	// Change the version here when clientLibraryVersion changes.
	assert.True(t, strings.HasPrefix(agent, "ably-pubsub-go/2.0.0 "), "wrong SDK family or version: %q", agent)
	assert.True(t, strings.HasSuffix(agent, " ably-pubsub-device"), "missing side agent: %q", agent)
}

func TestNewClient_MergesUserAgents(t *testing.T) {
	var agent string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent = r.Header.Get(ably.AblyAgentHeaderName)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)

	client, err := pubsub.NewClient(
		ably.WithEndpoint(u.Host),
		ably.WithTLS(false),
		ably.WithUseTokenAuth(true),
		ably.WithAutoConnect(false),
		ably.WithAgents(map[string]string{"foo": "1.2.3"}),
	)
	require.NoError(t, err)

	client.Time(context.Background())
	// Agents are sorted by product name, so the order is fixed.
	assert.True(t, strings.HasSuffix(agent, " ably-pubsub-device foo/1.2.3"), "unexpected agents: %q", agent)
}
