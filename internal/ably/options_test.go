//go:build !integration
// +build !integration

package ably_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ably/ably-pubsub-go/internal/ably"

	"github.com/stretchr/testify/assert"
)

func TestDefaultFallbacks_REC2c(t *testing.T) {
	expectedFallBackHosts := []string{
		"main.a.fallback.ably-realtime.com",
		"main.b.fallback.ably-realtime.com",
		"main.c.fallback.ably-realtime.com",
		"main.d.fallback.ably-realtime.com",
		"main.e.fallback.ably-realtime.com",
	}
	hosts := ably.DefaultFallbackHosts()
	assert.Equal(t, expectedFallBackHosts, hosts)
}

func TestEndpointFallbacks_REC2c(t *testing.T) {
	t.Run("standard endpoint", func(t *testing.T) {
		expectedFallBackHosts := []string{
			"acme.a.fallback.ably-realtime.com",
			"acme.b.fallback.ably-realtime.com",
			"acme.c.fallback.ably-realtime.com",
			"acme.d.fallback.ably-realtime.com",
			"acme.e.fallback.ably-realtime.com",
		}
		hosts := ably.GetEndpointFallbackHosts("acme")
		assert.Equal(t, expectedFallBackHosts, hosts)
	})

	t.Run("sandbox endpoint", func(t *testing.T) {
		expectedFallBackHosts := []string{
			"sandbox.a.fallback.ably-realtime-nonprod.com",
			"sandbox.b.fallback.ably-realtime-nonprod.com",
			"sandbox.c.fallback.ably-realtime-nonprod.com",
			"sandbox.d.fallback.ably-realtime-nonprod.com",
			"sandbox.e.fallback.ably-realtime-nonprod.com",
		}
		hosts := ably.GetEndpointFallbackHosts("nonprod:sandbox")
		assert.Equal(t, expectedFallBackHosts, hosts)
	})

	t.Run("nonprod endpoint", func(t *testing.T) {
		expectedFallBackHosts := []string{
			"acme.a.fallback.ably-realtime-nonprod.com",
			"acme.b.fallback.ably-realtime-nonprod.com",
			"acme.c.fallback.ably-realtime-nonprod.com",
			"acme.d.fallback.ably-realtime-nonprod.com",
			"acme.e.fallback.ably-realtime-nonprod.com",
		}
		hosts := ably.GetEndpointFallbackHosts("nonprod:acme")
		assert.Equal(t, expectedFallBackHosts, hosts)
	})
}

func TestInternetConnectionCheck_RTN17c(t *testing.T) {
	clientOptions := ably.NewClientOptions()
	assert.True(t, clientOptions.HasActiveInternetConnection())
}

func TestHosts_REC1(t *testing.T) {
	t.Run("REC1a with default options", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		assert.Equal(t, "main.realtime.ably.net", clientOptions.GetHostnameFromEndpoint())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Equal(t, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("REC1b with endpoint as a custom routing policy name", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithEndpoint("acme"))
		assert.Equal(t, "acme.realtime.ably.net", clientOptions.GetHostnameFromEndpoint())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Equal(t, ably.GetEndpointFallbackHosts("acme"), fallbackHosts)
	})

	t.Run("REC1b3 with endpoint as a nonprod routing policy name", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithEndpoint("nonprod:acme"))
		assert.Equal(t, "acme.realtime.ably-nonprod.net", clientOptions.GetHostnameFromEndpoint())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Equal(t, ably.GetEndpointFallbackHosts("nonprod:acme"), fallbackHosts)
	})

	t.Run("REC1b2 with endpoint as a fqdn with no fallbackHosts specified", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithEndpoint("foo.example.com"))
		assert.Equal(t, "foo.example.com", clientOptions.GetHostnameFromEndpoint())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, err := clientOptions.GetFallbackHosts()
		assert.NoError(t, err)
		assert.Nil(t, fallbackHosts)
	})

	t.Run("REC1b2 REC2a2 with endpoint as a fqdn with fallbackHosts specified", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithEndpoint("foo.example.com"), ably.WithFallbackHosts([]string{"fallback.foo.example.com"}))
		assert.Equal(t, "foo.example.com", clientOptions.GetHostnameFromEndpoint())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, err := clientOptions.GetFallbackHosts()
		assert.NoError(t, err)
		assert.Equal(t, []string{"fallback.foo.example.com"}, fallbackHosts)
	})

	t.Run("REC2a with fallbackHosts", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithFallbackHosts([]string{"a.example.com", "b.example.com"}))
		assert.Equal(t, "main.realtime.ably.net", clientOptions.GetHTTPHost())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Equal(t, []string{"a.example.com", "b.example.com"}, fallbackHosts)
	})
}

func TestEndpointResolution_REC1_REC2(t *testing.T) {
	defaultFallbacks := ably.DefaultFallbackHosts()
	custom := []string{"a.example.com", "b.example.com"}

	tests := []struct {
		name          string
		opts          []ably.ClientOption
		host          string
		fallbackHosts []string
	}{
		{"unset endpoint", nil, "main.realtime.ably.net", defaultFallbacks},
		{"routing policy name", []ably.ClientOption{ably.WithEndpoint("acme")}, "acme.realtime.ably.net", ably.GetEndpointFallbackHosts("acme")},
		{"nonprod routing policy name", []ably.ClientOption{ably.WithEndpoint("nonprod:sandbox")}, "sandbox.realtime.ably-nonprod.net", ably.GetEndpointFallbackHosts("nonprod:sandbox")},
		{"hostname", []ably.ClientOption{ably.WithEndpoint("foo.example.com")}, "foo.example.com", nil},
		{"localhost", []ably.ClientOption{ably.WithEndpoint("localhost")}, "localhost", nil},
		{"ipv6 hostname", []ably.ClientOption{ably.WithEndpoint("::1")}, "::1", nil},
		{"explicit fallbackHosts replace default fallbacks", []ably.ClientOption{ably.WithFallbackHosts(custom)}, "main.realtime.ably.net", custom},
		{"explicit fallbackHosts replace routing policy fallbacks", []ably.ClientOption{ably.WithEndpoint("acme"), ably.WithFallbackHosts(custom)}, "acme.realtime.ably.net", custom},
		{"explicit fallbackHosts with hostname endpoint", []ably.ClientOption{ably.WithEndpoint("foo.example.com"), ably.WithFallbackHosts(custom)}, "foo.example.com", custom},
		{"custom port keeps default fallbacks", []ably.ClientOption{ably.WithTLS(false), ably.WithPort(8080)}, "main.realtime.ably.net", defaultFallbacks},
		{"custom TLS port keeps default fallbacks", []ably.ClientOption{ably.WithTLSPort(8081)}, "main.realtime.ably.net", defaultFallbacks},
		{"custom port keeps routing policy fallbacks", []ably.ClientOption{ably.WithEndpoint("acme"), ably.WithTLSPort(8081)}, "acme.realtime.ably.net", ably.GetEndpointFallbackHosts("acme")},
		{"custom port with hostname endpoint has no fallbacks", []ably.ClientOption{ably.WithEndpoint("localhost"), ably.WithTLS(false), ably.WithPort(8080)}, "localhost", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clientOptions := ably.NewClientOptions(tc.opts...)
			assert.NoError(t, clientOptions.Validate())
			assert.Equal(t, tc.host, clientOptions.GetHTTPHost())
			assert.Equal(t, tc.host, clientOptions.GetRealtimeHost())
			fallbackHosts, err := clientOptions.GetFallbackHosts()
			assert.NoError(t, err)
			assert.Equal(t, tc.fallbackHosts, fallbackHosts)
		})
	}
}

func TestClientOptions(t *testing.T) {
	t.Run("must return error on invalid key", func(t *testing.T) {
		_, err := ably.NewHTTPClient([]ably.ClientOption{ably.WithKey("invalid")}...)
		assert.Error(t, err,
			"expected an error")
	})
	t.Run("must return error on empty options", func(t *testing.T) {
		_, err := ably.NewHTTPClient()
		assert.Error(t, err,
			"expected an error")
	})
	t.Run("must return error on nil value options", func(t *testing.T) {
		_, err := ably.NewHTTPClient(nil)
		assert.Error(t, err,
			"expected an error")
	})
}

func TestScopeParams(t *testing.T) {
	t.Run("must error when given invalid range", func(t *testing.T) {

		params := ably.ScopeParams{
			Start: time.Unix(0, 0).Add(123 * time.Millisecond),
			End:   time.Unix(0, 0).Add(122 * time.Millisecond),
		}
		err := params.EncodeValues(nil)
		assert.Error(t, err,
			"expected an error")
	})

	t.Run("must set url values", func(t *testing.T) {

		params := ably.ScopeParams{
			Start: time.Unix(0, 0).Add(122 * time.Millisecond),
			End:   time.Unix(0, 0).Add(123 * time.Millisecond),
		}
		u := make(url.Values)
		err := params.EncodeValues(&u)
		assert.NoError(t, err)
		start := u.Get("start")
		end := u.Get("end")
		assert.Equal(t, "122", start,
			"expected 122 got %s", start)
		assert.Equal(t, "123", end,
			"expected 123 got %s", end)
	})
}

func TestOption_NoTLS(t *testing.T) {
	t.Run("does not allow basic auth with no TLS", func(t *testing.T) {
		_, err := ably.NewHTTPClient(
			ably.WithKey("xxxxxx.yyyyyy:zzzzzz"),
			ably.WithTLS(false),
		)
		assert.Error(t, err)
		errInfo, ok := err.(*ably.ErrorInfo)
		assert.True(t, ok)
		assert.Equal(t, errInfo.Code, ably.ErrInvalidUseOfBasicAuthOverNonTLSTransport)
	})

	t.Run("allows basic auth with no TLS when InsecureAllowBasicAuthWithoutTLS is set", func(t *testing.T) {
		_, err := ably.NewHTTPClient(
			ably.WithKey("xxxxxx.yyyyyy:zzzzzz"),
			ably.WithTLS(false),
			ably.WithInsecureAllowBasicAuthWithoutTLS(),
		)
		assert.NoError(t, err)
	})
}

func TestPaginateParams(t *testing.T) {
	t.Run("returns nil with no values", func(t *testing.T) {

		params := ably.PaginateParams{}
		values := make(url.Values)
		err := params.EncodeValues(&values)
		assert.NoError(t, err)
		encode := values.Encode()
		assert.Equal(t, "", encode,
			"expected empty string got %s", encode)
	})

	t.Run("returns the full params encoded", func(t *testing.T) {

		params := ably.PaginateParams{
			Limit:     1,
			Direction: "backwards",
			ScopeParams: ably.ScopeParams{
				Start: time.Unix(0, 0).Add(123 * time.Millisecond),
				End:   time.Unix(0, 0).Add(124 * time.Millisecond),
				Unit:  "hello",
			},
		}
		values := make(url.Values)
		err := params.EncodeValues(&values)
		assert.NoError(t, err)
		assert.Equal(t, "direction=backwards&end=124&limit=1&start=123&unit=hello", values.Encode(),
			"expected \"direction=backwards&end=124&limit=1&start=123&unit=hello\" got %s", values.Encode())
	})

	t.Run("with value", func(t *testing.T) {

		params := ably.PaginateParams{
			Limit:     10,
			Direction: "backwards",
		}
		values := make(url.Values)
		err := params.EncodeValues(&values)
		assert.NoError(t, err)
	})

	t.Run("with a value for ScopeParams", func(t *testing.T) {

		values := make(url.Values)
		params := ably.PaginateParams{}
		params.Start = time.Unix(0, 0).Add(123 * time.Millisecond)
		err := params.EncodeValues(&values)
		assert.NoError(t, err)
		assert.Equal(t, "123", values.Get("start"),
			"expected 123 got %s", values.Get("start"))
	})
	t.Run("with invalid value for direction", func(t *testing.T) {

		values := make(url.Values)
		params := ably.PaginateParams{}
		params.Direction = "unknown"
		err := params.EncodeValues(&values)
		assert.Error(t, err)
	})
	t.Run("with invalid value for limit", func(t *testing.T) {

		values := make(url.Values)
		params := ably.PaginateParams{}
		params.Limit = -1
		err := params.EncodeValues(&values)
		assert.NoError(t, err)
		assert.Equal(t, "100", values.Get("limit"),
			"expected 100 got %s", values.Get("limit"))
	})
}

func TestIsEndpointHostname(t *testing.T) {
	assert.Equal(t, false, ably.IsEndpointHostname("sandbox"))
	assert.Equal(t, true, ably.IsEndpointHostname("sandbox.example.com"))
	assert.Equal(t, true, ably.IsEndpointHostname("127.0.0.1"))
	assert.Equal(t, true, ably.IsEndpointHostname("::1"))
	assert.Equal(t, true, ably.IsEndpointHostname("localhost"))
}

func TestWithAgents_FromNil(t *testing.T) {
	opts := ably.ApplyOptionsWithDefaults(ably.WithAgents(map[string]string{"foo": "1.0.0"}))
	assert.Equal(t, map[string]string{"foo": "1.0.0"}, opts.Agents)
}

func TestWithAgents_AccumulatesAcrossCalls(t *testing.T) {
	opts := ably.ApplyOptionsWithDefaults(
		ably.WithAgents(map[string]string{"foo": "1.0.0"}),
		ably.WithAgents(map[string]string{"bar": ""}),
	)
	assert.Equal(t, map[string]string{"foo": "1.0.0", "bar": ""}, opts.Agents)
}

func TestWithAgents_SameKeyOverrides(t *testing.T) {
	opts := ably.ApplyOptionsWithDefaults(
		ably.WithAgents(map[string]string{"foo": "1.0.0"}),
		ably.WithAgents(map[string]string{"foo": "2.0.0"}),
	)
	assert.Equal(t, map[string]string{"foo": "2.0.0"}, opts.Agents)
}

func TestWithAgents_DoesNotAliasCallersMap(t *testing.T) {
	agents := map[string]string{"foo": "1.0.0"}
	opts := ably.ApplyOptionsWithDefaults(ably.WithAgents(agents))

	agents["bar"] = "2.0.0"
	assert.Equal(t, map[string]string{"foo": "1.0.0"}, opts.Agents, "a later change to the caller's map leaked into the options")

	opts.Agents["baz"] = "3.0.0"
	assert.Equal(t, map[string]string{"foo": "1.0.0", "bar": "2.0.0"}, agents, "a change to the options leaked into the caller's map")
}

func TestAblyAgentIdentifier_SortedAndBareFlag(t *testing.T) {
	identifier := ably.AgentIdentifier(map[string]string{"zeta": "1.0.0", "alpha": "", "mid": "2.0.0"})
	// Products come after the SDK, runtime and OS entries, sorted alphabetically.
	// An empty version gives a bare name (RSC7d).
	assert.True(t, strings.HasSuffix(identifier, " alpha mid/2.0.0 zeta/1.0.0"), "unexpected identifier: %q", identifier)
}
