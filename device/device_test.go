package device_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/ably/ably-pubsub-go/device"
	"github.com/ably/ably-pubsub-go/internal/ably"
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
	client, err := device.NewClient(
		ably.WithEndpoint(u.Host),
		ably.WithTLS(false),
		ably.WithUseTokenAuth(true),
		ably.WithAutoConnect(false),
	)
	require.NoError(t, err)

	client.Time(context.Background())
	assert.Equal(t, ably.AgentIdentifier(map[string]string{"ably-pubsub-device": ""}), agent)
}
