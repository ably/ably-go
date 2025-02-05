//go:build !integration
// +build !integration

package ably_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"

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

	t.Run("legacy support", func(t *testing.T) {
		t.Run("REC1c with production environment", func(t *testing.T) {
			clientOptions := ably.NewClientOptions(ably.WithEnvironment("production"))
			assert.Equal(t, "main.realtime.ably.net", clientOptions.GetHostnameFromEndpoint())
			assert.False(t, clientOptions.NoTLS)
			port, isDefaultPort := clientOptions.ActivePort()
			assert.Equal(t, 443, port)
			assert.True(t, isDefaultPort)
			fallbackHosts, _ := clientOptions.GetFallbackHosts()
			assert.Equal(t, ably.DefaultFallbackHosts(), fallbackHosts)
		})

		t.Run("REC1c with custom environment", func(t *testing.T) {
			clientOptions := ably.NewClientOptions(ably.WithEnvironment("acme"))
			assert.Equal(t, "acme.realtime.ably.net", clientOptions.GetRestHost())
			assert.False(t, clientOptions.NoTLS)
			port, isDefaultPort := clientOptions.ActivePort()
			assert.Equal(t, 443, port)
			assert.True(t, isDefaultPort)
			fallbackHosts, _ := clientOptions.GetFallbackHosts()
			assert.Equal(t, ably.GetEndpointFallbackHosts("acme"), fallbackHosts)
		})

		t.Run("REC1c REC2a1 with custom environment and fallbackHostUseDefault", func(t *testing.T) {
			clientOptions := ably.NewClientOptions(ably.WithEnvironment("acme"), ably.WithFallbackHostsUseDefault(true))
			assert.Equal(t, "acme.realtime.ably.net", clientOptions.GetRestHost())
			assert.False(t, clientOptions.NoTLS)
			port, isDefaultPort := clientOptions.ActivePort()
			assert.Equal(t, 443, port)
			assert.True(t, isDefaultPort)
			fallbackHosts, _ := clientOptions.GetFallbackHosts()
			assert.Equal(t, ably.DefaultFallbackHosts(), fallbackHosts)
		})

		t.Run("REC1d1 with custom restHost", func(t *testing.T) {
			clientOptions := ably.NewClientOptions(ably.WithRESTHost("test.org"))
			assert.Equal(t, "test.org", clientOptions.GetRestHost())
			assert.False(t, clientOptions.NoTLS)
			port, isDefaultPort := clientOptions.ActivePort()
			assert.Equal(t, 443, port)
			assert.True(t, isDefaultPort)
			fallbackHosts, _ := clientOptions.GetFallbackHosts()
			assert.Nil(t, fallbackHosts)
		})

		t.Run("REC1d2 with custom realtimeHost", func(t *testing.T) {
			clientOptions := ably.NewClientOptions(ably.WithRealtimeHost("ws.test.org"))
			assert.Equal(t, "ws.test.org", clientOptions.GetRealtimeHost())
			assert.False(t, clientOptions.NoTLS)
			port, isDefaultPort := clientOptions.ActivePort()
			assert.Equal(t, 443, port)
			assert.True(t, isDefaultPort)
			fallbackHosts, _ := clientOptions.GetFallbackHosts()
			assert.Nil(t, fallbackHosts)
		})

		t.Run("REC1d with custom restHost and realtimeHost", func(t *testing.T) {
			clientOptions := ably.NewClientOptions(ably.WithRealtimeHost("ws.test.org"), ably.WithRESTHost("test.org"))
			assert.Equal(t, "test.org", clientOptions.GetRestHost())
			assert.False(t, clientOptions.NoTLS)
			port, isDefaultPort := clientOptions.ActivePort()
			assert.Equal(t, 443, port)
			assert.True(t, isDefaultPort)
			fallbackHosts, _ := clientOptions.GetFallbackHosts()
			assert.Nil(t, fallbackHosts)
		})

		t.Run("REC1d REC2b with custom restHost and realtimeHost and fallbackHostsUseDefault", func(t *testing.T) {
			clientOptions := ably.NewClientOptions(
				ably.WithRealtimeHost("ws.test.org"),
				ably.WithRESTHost("test.org"),
				ably.WithFallbackHostsUseDefault(true))
			assert.Equal(t, "test.org", clientOptions.GetRestHost())
			assert.False(t, clientOptions.NoTLS)
			port, isDefaultPort := clientOptions.ActivePort()
			assert.Equal(t, 443, port)
			assert.True(t, isDefaultPort)
			fallbackHosts, _ := clientOptions.GetFallbackHosts()
			assert.Equal(t, ably.DefaultFallbackHosts(), fallbackHosts)
		})
	})

	t.Run("If endpoint option is used with deprecated fallbackHostUseDefault, throw error", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(
			ably.WithFallbackHostsUseDefault(true),
			ably.WithEndpoint("custom"))
		err := clientOptions.Validate()
		assert.Equal(t, err.Error(),
			"invalid client option: cannot use endpoint with any of deprecated options environment, realtimeHost, restHost or FallbackHostsUseDefault")
	})

	t.Run("REC2a with fallbackHosts", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithFallbackHosts([]string{"a.example.com", "b.example.com"}))
		assert.Equal(t, "main.realtime.ably.net", clientOptions.GetRestHost())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Equal(t, []string{"a.example.com", "b.example.com"}, fallbackHosts)
	})

	t.Run("REC2a1 with fallbackHosts and fallbackHostsUseDefault", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(
			ably.WithFallbackHosts([]string{"a.example.com", "b.example.com"}),
			ably.WithFallbackHostsUseDefault(true))
		_, err := clientOptions.GetFallbackHosts()
		assert.Equal(t, err.Error(),
			"fallbackHosts and fallbackHostsUseDefault cannot both be set")
	})

	t.Run("REC2a1 with fallbackHostsUseDefault And custom port", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithTLSPort(8081), ably.WithFallbackHostsUseDefault(true))
		_, isDefaultPort := clientOptions.ActivePort()
		assert.False(t, isDefaultPort)
		_, err := clientOptions.GetFallbackHosts()
		assert.Equal(t, err.Error(),
			"fallbackHostsUseDefault cannot be set when port or tlsPort are set")

		clientOptions = ably.NewClientOptions(
			ably.WithTLS(false),
			ably.WithPort(8080),
			ably.WithFallbackHostsUseDefault(true))

		_, isDefaultPort = clientOptions.ActivePort()
		assert.False(t, isDefaultPort)
		_, err = clientOptions.GetFallbackHosts()
		assert.Equal(t, err.Error(),
			"fallbackHostsUseDefault cannot be set when port or tlsPort are set")
	})
}

func TestClientOptions(t *testing.T) {
	t.Run("must return error on invalid key", func(t *testing.T) {
		_, err := ably.NewREST([]ably.ClientOption{ably.WithKey("invalid")}...)
		assert.Error(t, err,
			"expected an error")
	})
	t.Run("must return error on empty options", func(t *testing.T) {
		_, err := ably.NewREST()
		assert.Error(t, err,
			"expected an error")
	})
	t.Run("must return error on nil value options", func(t *testing.T) {
		_, err := ably.NewREST(nil)
		assert.Error(t, err,
			"expected an error")
	})
	t.Run("must return error on invalid combinations", func(t *testing.T) {
		_, err := ably.NewREST([]ably.ClientOption{ably.WithEndpoint("acme"), ably.WithEnvironment("acme"), ably.WithRealtimeHost("foo.example.com"), ably.WithRESTHost("foo.example.com")}...)
		assert.Error(t, err,
			"expected an error")

		_, err = ably.NewREST([]ably.ClientOption{ably.WithEndpoint("acme"), ably.WithEnvironment("acme")}...)
		assert.Error(t, err,
			"expected an error")

		_, err = ably.NewREST([]ably.ClientOption{ably.WithEnvironment("acme"), ably.WithRealtimeHost("foo.example.com")}...)
		assert.Error(t, err,
			"expected an error")

		_, err = ably.NewREST([]ably.ClientOption{ably.WithEnvironment("acme"), ably.WithRESTHost("foo.example.com")}...)
		assert.Error(t, err,
			"expected an error")

		_, err = ably.NewREST([]ably.ClientOption{ably.WithEndpoint("acme"), ably.WithRealtimeHost("foo.example.com")}...)
		assert.Error(t, err,
			"expected an error")

		_, err = ably.NewREST([]ably.ClientOption{ably.WithEndpoint("acme"), ably.WithRESTHost("foo.example.com")}...)
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
		_, err := ably.NewREST(
			ably.WithKey("xxxxxx.yyyyyy:zzzzzz"),
			ably.WithTLS(false),
		)
		assert.Error(t, err)
		errInfo, ok := err.(*ably.ErrorInfo)
		assert.True(t, ok)
		assert.Equal(t, errInfo.Code, ably.ErrInvalidUseOfBasicAuthOverNonTLSTransport)
	})

	t.Run("allows basic auth with no TLS when InsecureAllowBasicAuthWithoutTLS is set", func(t *testing.T) {
		_, err := ably.NewREST(
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

func TestIsEndpointFQDN(t *testing.T) {
	assert.Equal(t, false, ably.IsEndpointFQDN("sandbox"))
	assert.Equal(t, true, ably.IsEndpointFQDN("sandbox.example.com"))
	assert.Equal(t, true, ably.IsEndpointFQDN("127.0.0.1"))
	assert.Equal(t, true, ably.IsEndpointFQDN("localhost"))
}
