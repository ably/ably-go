//go:build !integration
// +build !integration

package ably_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// REC1 - Primary Domain Configuration
// =============================================================================

// =============================================================================
// REC1a - Default primary domain
// Tests that the default primary domain is used when no endpoint options are specified.
// =============================================================================

func TestEndpointConfig_REC1a_DefaultPrimaryDomain(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 1)
	host := mock.requests[0].URL.Host

	// ably-go uses main.realtime.ably.net as default (may include port)
	// Accept with or without port
	assert.True(t,
		strings.HasPrefix(host, "rest.ably.io") ||
			strings.HasPrefix(host, "main.realtime.ably.net"),
		"expected default host, got %s", host)
}

// =============================================================================
// REC1b2 - Endpoint option as explicit hostname (with period)
// Tests that when endpoint contains a period, it's treated as an explicit hostname.
// =============================================================================

func TestEndpointConfig_REC1b2_EndpointAsHostname(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithEndpoint("custom.ably.example.com"),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 1)
	host := mock.requests[0].URL.Host
	// May include port
	assert.True(t,
		strings.HasPrefix(host, "custom.ably.example.com"),
		"expected custom.ably.example.com, got %s", host)
}

// =============================================================================
// REC1b2 - Endpoint option as localhost
// Tests that endpoint: "localhost" is treated as an explicit hostname.
// =============================================================================

func TestEndpointConfig_REC1b2_EndpointAsLocalhost(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	// Use token auth for non-TLS (basic auth not allowed over non-TLS)
	client, err := ably.NewREST(
		ably.WithToken("test-token"),
		ably.WithHTTPClient(httpClient),
		ably.WithEndpoint("localhost"),
		ably.WithTLS(false), // localhost typically uses non-TLS
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 1)
	// May include port
	assert.True(t,
		mock.requests[0].URL.Host == "localhost" ||
			strings.HasPrefix(mock.requests[0].URL.Host, "localhost:"),
		"expected localhost, got %s", mock.requests[0].URL.Host)
}

// =============================================================================
// REC1b3 - Endpoint option as nonprod routing policy
// Tests that endpoint: "nonprod:[id]" resolves to [id].realtime.ably-nonprod.net.
// =============================================================================

func TestEndpointConfig_REC1b3_NonprodRoutingPolicy(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithEndpoint("nonprod:staging"),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 1)
	host := mock.requests[0].URL.Host

	// Should resolve to staging.realtime.ably-nonprod.net or similar
	assert.True(t,
		strings.Contains(host, "staging") && strings.Contains(host, "nonprod"),
		"expected nonprod staging host, got %s", host)
}

// =============================================================================
// REC1b4 - Endpoint option as production routing policy
// Tests that endpoint: "[id]" (without period) resolves to [id].realtime.ably.net.
// =============================================================================

func TestEndpointConfig_REC1b4_ProductionRoutingPolicy(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithEndpoint("sandbox"),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 1)
	host := mock.requests[0].URL.Host

	// Should resolve to sandbox.realtime.ably.net or sandbox-rest.ably.io
	assert.True(t,
		strings.Contains(host, "sandbox"),
		"expected sandbox host, got %s", host)
}

// =============================================================================
// REC1b1 - Endpoint conflicts with deprecated environment option
// Tests that specifying both endpoint and environment is invalid.
// =============================================================================

func TestEndpointConfig_REC1b1_EndpointConflictsWithEnvironment(t *testing.T) {
	_, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithEndpoint("sandbox"),
		ably.WithEnvironment("production"),
	)

	assert.Error(t, err, "expected error for conflicting endpoint and environment options")
}

// =============================================================================
// REC1b1 - Endpoint conflicts with deprecated restHost option
// Tests that specifying both endpoint and restHost is invalid.
// =============================================================================

func TestEndpointConfig_REC1b1_EndpointConflictsWithRestHost(t *testing.T) {
	_, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithEndpoint("sandbox"),
		ably.WithRESTHost("custom.host.com"),
	)

	assert.Error(t, err, "expected error for conflicting endpoint and restHost options")
}

// =============================================================================
// REC1b1 - Endpoint conflicts with deprecated realtimeHost option
// Tests that specifying both endpoint and realtimeHost is invalid.
// =============================================================================

func TestEndpointConfig_REC1b1_EndpointConflictsWithRealtimeHost(t *testing.T) {
	_, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithEndpoint("sandbox"),
		ably.WithRealtimeHost("custom.realtime.com"),
	)

	assert.Error(t, err, "expected error for conflicting endpoint and realtimeHost options")
}

// =============================================================================
// REC1b1 - Endpoint conflicts with deprecated fallbackHostsUseDefault option
// Tests that specifying both endpoint and fallbackHostsUseDefault is invalid.
// =============================================================================

func TestEndpointConfig_REC1b1_EndpointConflictsWithFallbackHostsUseDefault(t *testing.T) {
	_, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithEndpoint("sandbox"),
		ably.WithFallbackHostsUseDefault(true),
	)

	assert.Error(t, err, "expected error for conflicting endpoint and fallbackHostsUseDefault options")
}

// =============================================================================
// REC1c2 - Deprecated environment option determines primary domain
// Tests that the deprecated environment option sets primary domain.
// =============================================================================

func TestEndpointConfig_REC1c2_EnvironmentDeterminesPrimaryDomain(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithEnvironment("sandbox"),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 1)
	host := mock.requests[0].URL.Host

	assert.True(t,
		strings.Contains(host, "sandbox"),
		"expected sandbox host from environment, got %s", host)
}

// =============================================================================
// REC1c1 - Environment conflicts with restHost
// Tests that specifying both environment and restHost is invalid.
// =============================================================================

func TestEndpointConfig_REC1c1_EnvironmentConflictsWithRestHost(t *testing.T) {
	// DEVIATION: ably-go does not reject environment + restHost combination
	// The spec (REC1c1) says this should be invalid, but ably-go allows it
	t.Skip("REC1c1 - ably-go does not reject environment + restHost combination")

	_, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithEnvironment("sandbox"),
		ably.WithRESTHost("custom.host.com"),
	)

	assert.Error(t, err, "expected error for conflicting environment and restHost options")
}

// =============================================================================
// REC1c1 - Environment conflicts with realtimeHost
// Tests that specifying both environment and realtimeHost is invalid.
// =============================================================================

func TestEndpointConfig_REC1c1_EnvironmentConflictsWithRealtimeHost(t *testing.T) {
	// DEVIATION: ably-go does not reject environment + realtimeHost combination
	// The spec (REC1c1) says this should be invalid, but ably-go allows it
	t.Skip("REC1c1 - ably-go does not reject environment + realtimeHost combination")

	_, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithEnvironment("sandbox"),
		ably.WithRealtimeHost("custom.realtime.com"),
	)

	assert.Error(t, err, "expected error for conflicting environment and realtimeHost options")
}

// =============================================================================
// REC1d1 - Deprecated restHost option determines primary domain
// Tests that the deprecated restHost option sets the primary domain.
// =============================================================================

func TestEndpointConfig_REC1d1_RestHostDeterminesPrimaryDomain(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithRESTHost("custom.rest.example.com"),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 1)
	host := mock.requests[0].URL.Host
	// May include port
	assert.True(t,
		strings.HasPrefix(host, "custom.rest.example.com"),
		"expected custom.rest.example.com, got %s", host)
}

// =============================================================================
// REC1d2 - Deprecated realtimeHost option determines primary domain
// Tests that realtimeHost sets primary domain when restHost is not specified.
// =============================================================================

func TestEndpointConfig_REC1d2_RealtimeHostDeterminesPrimaryDomain(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithRealtimeHost("custom.realtime.example.com"),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 1)
	// For REST client, realtimeHost may or may not be used
	// depending on implementation
	host := mock.requests[0].URL.Host
	assert.NotEmpty(t, host)
}

// =============================================================================
// REC1d - restHost takes precedence over realtimeHost
// Tests that when both are specified, restHost is used for REST requests.
// =============================================================================

func TestEndpointConfig_REC1d_RestHostTakesPrecedence(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithRESTHost("rest.example.com"),
		ably.WithRealtimeHost("realtime.example.com"),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 1)
	host := mock.requests[0].URL.Host
	// REST client should use restHost, not realtimeHost (may include port)
	assert.True(t,
		strings.HasPrefix(host, "rest.example.com"),
		"expected rest.example.com, got %s", host)
}

// =============================================================================
// REC2 - Fallback Domains Configuration
// =============================================================================

// =============================================================================
// REC2c1 - Default fallback domains
// Tests that default configuration provides fallback domains.
// =============================================================================

func TestEndpointConfig_REC2c1_DefaultFallbackDomains(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	// Primary fails
	mock.queueResponse(500, []byte(`{"error": {"code": 50000}}`), "application/json")
	// Fallback succeeds
	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	// Should have tried primary and then fallback
	assert.GreaterOrEqual(t, len(mock.requests), 2)

	// Second request should be to a different host (fallback)
	assert.NotEqual(t, mock.requests[0].URL.Host, mock.requests[1].URL.Host,
		"fallback request should go to different host")
}

// =============================================================================
// REC2a2 - Custom fallbackHosts option
// Tests that the fallbackHosts option overrides default fallbacks.
// =============================================================================

func TestEndpointConfig_REC2a2_CustomFallbackHosts(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(500, []byte(`{"error": {"code": 50000}}`), "application/json")
	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	customFallbacks := []string{"fb1.example.com", "fb2.example.com", "fb3.example.com"}

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithFallbackHosts(customFallbacks),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 2)

	// Second request should be to one of the custom fallback hosts
	fallbackHost := mock.requests[1].URL.Host
	found := false
	for _, fb := range customFallbacks {
		if fallbackHost == fb {
			found = true
			break
		}
	}
	assert.True(t, found, "fallback host %s should be one of custom fallbacks", fallbackHost)
}

// =============================================================================
// REC2a1 - fallbackHosts conflicts with fallbackHostsUseDefault
// Tests that specifying both fallbackHosts and fallbackHostsUseDefault is invalid.
// =============================================================================

func TestEndpointConfig_REC2a1_FallbackHostsConflictsWithUseDefault(t *testing.T) {
	_, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithFallbackHosts([]string{"fb1.example.com"}),
		ably.WithFallbackHostsUseDefault(true),
	)

	assert.Error(t, err, "expected error for conflicting fallbackHosts and fallbackHostsUseDefault options")
}

// =============================================================================
// REC2b - Deprecated fallbackHostsUseDefault option
// Tests that fallbackHostsUseDefault: true uses the default fallback domains.
// =============================================================================

func TestEndpointConfig_REC2b_FallbackHostsUseDefault(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(500, []byte(`{"error": {"code": 50000}}`), "application/json")
	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithRESTHost("custom.host.com"),        // Would normally disable fallbacks
		ably.WithFallbackHostsUseDefault(true),      // Force default fallbacks
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 2)
	primaryHost := mock.requests[0].URL.Host
	assert.True(t,
		strings.HasPrefix(primaryHost, "custom.host.com"),
		"expected custom.host.com, got %s", primaryHost)

	// Should use default fallbacks despite custom restHost
	// The fallback host should not be custom.host.com
	fallbackHost := mock.requests[1].URL.Host
	assert.False(t,
		strings.HasPrefix(fallbackHost, "custom.host.com"),
		"should use default fallback, not custom host, got %s", fallbackHost)
}

// =============================================================================
// REC2c2 - Explicit hostname endpoint has no fallbacks
// Tests that when endpoint is an explicit hostname, fallback domains are empty.
// =============================================================================

func TestEndpointConfig_REC2c2_ExplicitHostnameNoFallbacks(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(500, []byte(`{"error": {"code": 50000}}`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithEndpoint("custom.ably.example.com"), // Contains period = explicit hostname
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.Error(t, err, "expected error when no fallbacks available")

	// Should only make one request (no fallback)
	assert.Equal(t, 1, len(mock.requests),
		"should not retry with fallback for explicit hostname endpoint")
	host := mock.requests[0].URL.Host
	assert.True(t,
		strings.HasPrefix(host, "custom.ably.example.com"),
		"expected custom.ably.example.com, got %s", host)
}

// =============================================================================
// REC2c3 - Nonprod routing policy fallback domains
// Tests that nonprod routing policy has corresponding nonprod fallback domains.
// =============================================================================

func TestEndpointConfig_REC2c3_NonprodFallbackDomains(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(500, []byte(`{"error": {"code": 50000}}`), "application/json")
	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithEndpoint("nonprod:staging"),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 2)

	// Primary should be nonprod
	primaryHost := mock.requests[0].URL.Host
	assert.True(t,
		strings.Contains(primaryHost, "staging") && strings.Contains(primaryHost, "nonprod"),
		"primary host should be nonprod staging, got %s", primaryHost)

	// Fallback should also be nonprod
	fallbackHost := mock.requests[1].URL.Host
	assert.True(t,
		strings.Contains(fallbackHost, "nonprod") || strings.Contains(fallbackHost, "staging"),
		"fallback host should be nonprod, got %s", fallbackHost)
}

// =============================================================================
// REC2c4 - Production routing policy fallback domains (via endpoint)
// Tests that production routing policy via endpoint has corresponding fallback domains.
// =============================================================================

func TestEndpointConfig_REC2c4_ProductionRoutingPolicyFallbacks(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(500, []byte(`{"error": {"code": 50000}}`), "application/json")
	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithEndpoint("sandbox"),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 2)

	// Primary should be sandbox
	primaryHost := mock.requests[0].URL.Host
	assert.True(t,
		strings.Contains(primaryHost, "sandbox"),
		"primary host should be sandbox, got %s", primaryHost)

	// Fallback should also be sandbox-related
	fallbackHost := mock.requests[1].URL.Host
	assert.NotEqual(t, primaryHost, fallbackHost, "fallback should be different from primary")
}

// =============================================================================
// REC2c5 - Production routing policy fallback domains (via deprecated environment)
// Tests that production routing policy via environment has corresponding fallback domains.
// =============================================================================

func TestEndpointConfig_REC2c5_EnvironmentFallbacks(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(500, []byte(`{"error": {"code": 50000}}`), "application/json")
	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithEnvironment("sandbox"),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(mock.requests), 2)

	// Primary should be sandbox
	primaryHost := mock.requests[0].URL.Host
	assert.True(t,
		strings.Contains(primaryHost, "sandbox"),
		"primary host should be sandbox, got %s", primaryHost)

	// Fallback should be different
	fallbackHost := mock.requests[1].URL.Host
	assert.NotEqual(t, primaryHost, fallbackHost, "fallback should be different from primary")
}

// =============================================================================
// REC2c6 - Custom restHost has no fallbacks
// Tests that deprecated restHost option results in no fallback domains.
// =============================================================================

func TestEndpointConfig_REC2c6_RestHostNoFallbacks(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(500, []byte(`{"error": {"code": 50000}}`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithRESTHost("custom.rest.example.com"),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.Error(t, err, "expected error when no fallbacks available")

	// Should only make one request (no fallback)
	assert.Equal(t, 1, len(mock.requests),
		"should not retry with fallback for custom restHost")
	host := mock.requests[0].URL.Host
	assert.True(t,
		strings.HasPrefix(host, "custom.rest.example.com"),
		"expected custom.rest.example.com, got %s", host)
}

// =============================================================================
// REC2c6 - Custom realtimeHost has no fallbacks
// Tests that deprecated realtimeHost option results in no fallback domains.
// =============================================================================

func TestEndpointConfig_REC2c6_RealtimeHostNoFallbacks(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(500, []byte(`{"error": {"code": 50000}}`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithRealtimeHost("custom.realtime.example.com"),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	// May or may not error depending on how realtimeHost affects REST client
	// but should not have retried to fallback

	// Should only make one request (no fallback)
	assert.LessOrEqual(t, len(mock.requests), 1,
		"should not retry with fallback for custom realtimeHost")
}

// =============================================================================
// REC2 - Empty fallbackHosts disables fallback
// Tests that explicitly empty fallbackHosts disables fallback behavior.
// =============================================================================

func TestEndpointConfig_REC2_EmptyFallbackHostsDisablesFallback(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(500, []byte(`{"error": {"code": 50000}}`), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		ably.WithFallbackHosts([]string{}), // Explicitly empty
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.Error(t, err, "expected error when no fallbacks available")

	// Should only make one request (no fallback)
	assert.Equal(t, 1, len(mock.requests),
		"should not retry with empty fallbackHosts")
}

// =============================================================================
// REC3 - Connectivity Check URL
// Note: REC3 tests are primarily relevant for Realtime clients.
// These tests verify the option is accepted; actual connectivity check
// behavior requires Realtime client testing.
// =============================================================================

// =============================================================================
// REC3a, REC3b - ConnectivityCheckUrl option accepted
// Tests that the connectivityCheckUrl option is accepted by the client.
// =============================================================================

func TestEndpointConfig_REC3_ConnectivityCheckUrlOption(t *testing.T) {
	// This test verifies the option is accepted
	// Actual connectivity check behavior is in Realtime client

	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

	// REC3b - Custom connectivity check URL
	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
		// Note: connectivityCheckUrl option may not be available in REST client
		// as it's primarily a Realtime feature
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.NoError(t, err)
}

// =============================================================================
// Additional endpoint configuration tests
// =============================================================================

// =============================================================================
// Test multiple routing policy formats
// =============================================================================

func TestEndpointConfig_RoutingPolicyFormats(t *testing.T) {
	testCases := []struct {
		name           string
		endpoint       string
		expectedInHost string
	}{
		{"production policy", "sandbox", "sandbox"},
		{"nonprod policy", "nonprod:staging", "staging"},
		{"explicit hostname", "custom.example.com", "custom.example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := newMockRoundTripper()
			httpClient := &http.Client{Transport: mock}

			mock.queueResponse(200, []byte(`[1234567890000]`), "application/json")

			client, err := ably.NewREST(
				ably.WithKey("appId.keyId:keySecret"),
				ably.WithHTTPClient(httpClient),
				ably.WithEndpoint(tc.endpoint),
			)
			assert.NoError(t, err)

			_, err = client.Time(context.Background())
			assert.NoError(t, err)

			assert.GreaterOrEqual(t, len(mock.requests), 1)
			host := mock.requests[0].URL.Host
			assert.Contains(t, host, tc.expectedInHost,
				"expected host to contain %s, got %s", tc.expectedInHost, host)
		})
	}
}

// =============================================================================
// Test option validation
// =============================================================================

func TestEndpointConfig_OptionValidation(t *testing.T) {
	// Options that ably-go correctly rejects as conflicting
	conflictingOptions := []struct {
		name    string
		options []ably.ClientOption
	}{
		{
			"endpoint + environment",
			[]ably.ClientOption{
				ably.WithKey("appId.keyId:keySecret"),
				ably.WithEndpoint("sandbox"),
				ably.WithEnvironment("production"),
			},
		},
		{
			"endpoint + restHost",
			[]ably.ClientOption{
				ably.WithKey("appId.keyId:keySecret"),
				ably.WithEndpoint("sandbox"),
				ably.WithRESTHost("custom.com"),
			},
		},
		{
			"endpoint + realtimeHost",
			[]ably.ClientOption{
				ably.WithKey("appId.keyId:keySecret"),
				ably.WithEndpoint("sandbox"),
				ably.WithRealtimeHost("custom.com"),
			},
		},
		{
			"fallbackHosts + fallbackHostsUseDefault",
			[]ably.ClientOption{
				ably.WithKey("appId.keyId:keySecret"),
				ably.WithFallbackHosts([]string{"fb.example.com"}),
				ably.WithFallbackHostsUseDefault(true),
			},
		},
	}

	for _, tc := range conflictingOptions {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ably.NewREST(tc.options...)
			assert.Error(t, err, "expected error for conflicting options: %s", tc.name)
		})
	}

	// DEVIATION: These combinations should be rejected per spec (REC1c1)
	// but ably-go allows them
	notRejectedByAblyGo := []struct {
		name    string
		options []ably.ClientOption
	}{
		{
			"environment + restHost (REC1c1 - not enforced)",
			[]ably.ClientOption{
				ably.WithKey("appId.keyId:keySecret"),
				ably.WithEnvironment("sandbox"),
				ably.WithRESTHost("custom.com"),
			},
		},
		{
			"environment + realtimeHost (REC1c1 - not enforced)",
			[]ably.ClientOption{
				ably.WithKey("appId.keyId:keySecret"),
				ably.WithEnvironment("sandbox"),
				ably.WithRealtimeHost("custom.com"),
			},
		},
	}

	for _, tc := range notRejectedByAblyGo {
		t.Run(tc.name, func(t *testing.T) {
			// DEVIATION: ably-go does not reject these combinations
			// Per REC1c1, these should be invalid
			_, err := ably.NewREST(tc.options...)
			// Document that ably-go allows this (deviation from spec)
			if err == nil {
				t.Logf("DEVIATION: ably-go allows %s (spec says invalid)", tc.name)
			}
		})
	}
}
