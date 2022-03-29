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

func TestDefaultFallbacks_RSC15h(t *testing.T) {
	t.Run("with env should return environment fallback hosts", func(t *testing.T) {
		expectedFallBackHosts := []string{
			"a.ably-realtime.com",
			"b.ably-realtime.com",
			"c.ably-realtime.com",
			"d.ably-realtime.com",
			"e.ably-realtime.com",
		}
		hosts := ably.DefaultFallbackHosts()
		assert.Equal(t, expectedFallBackHosts, hosts)
	})
}

func TestEnvFallbackHosts_RSC15i(t *testing.T) {
	t.Run("with env should return environment fallback hosts", func(t *testing.T) {
		expectedFallBackHosts := []string{
			"sandbox-a-fallback.ably-realtime.com",
			"sandbox-b-fallback.ably-realtime.com",
			"sandbox-c-fallback.ably-realtime.com",
			"sandbox-d-fallback.ably-realtime.com",
			"sandbox-e-fallback.ably-realtime.com",
		}
		hosts := ably.GetEnvFallbackHosts("sandbox")
		assert.Equal(t, expectedFallBackHosts, hosts)
	})
}

func TestFallbackHosts_RSC15b(t *testing.T) {
	t.Run("RSC15e RSC15g3 with default options", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		assert.Equal(t, "realtime.ably.io", clientOptions.GetRealtimeHost())
		assert.Equal(t, "rest.ably.io", clientOptions.GetRestHost())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Equal(t, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("RSC15h with production environment", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithEnvironment("production"))
		assert.Equal(t, "realtime.ably.io", clientOptions.GetRealtimeHost())
		assert.Equal(t, "rest.ably.io", clientOptions.GetRestHost())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Equal(t, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("RSC15g2 RTC1e with custom environment", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithEnvironment("sandbox"))
		assert.Equal(t, "sandbox-realtime.ably.io", clientOptions.GetRealtimeHost())
		assert.Equal(t, "sandbox-rest.ably.io", clientOptions.GetRestHost())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Equal(t, ably.GetEnvFallbackHosts("sandbox"), fallbackHosts)
	})

	t.Run("RSC15g4 RTC1e with custom environment and fallbackHostUseDefault", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithEnvironment("sandbox"), ably.WithFallbackHostsUseDefault(true))
		assert.Equal(t, "sandbox-realtime.ably.io", clientOptions.GetRealtimeHost())
		assert.Equal(t, "sandbox-rest.ably.io", clientOptions.GetRestHost())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Equal(t, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("RSC11b RTN17b RTC1e with custom environment and non default ports", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(
			ably.WithEnvironment("local"),
			ably.WithPort(8080),
			ably.WithTLSPort(8081),
		)
		assert.Equal(t, "local-realtime.ably.io", clientOptions.GetRealtimeHost())
		assert.Equal(t, "local-rest.ably.io", clientOptions.GetRestHost())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 8081, port)
		assert.False(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Nil(t, fallbackHosts)
	})

	t.Run("RSC11 with custom rest host", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithRESTHost("test.org"))
		assert.Equal(t, "test.org", clientOptions.GetRealtimeHost())
		assert.Equal(t, "test.org", clientOptions.GetRestHost())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Nil(t, fallbackHosts)
	})

	t.Run("RSC11 with custom rest host and realtime host", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithRealtimeHost("ws.test.org"), ably.WithRESTHost("test.org"))
		assert.Equal(t, "ws.test.org", clientOptions.GetRealtimeHost())
		assert.Equal(t, "test.org", clientOptions.GetRestHost())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Nil(t, fallbackHosts)
	})

	t.Run("RSC15b with custom rest host and realtime host and fallbackHostsUseDefault", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(
			ably.WithRealtimeHost("ws.test.org"),
			ably.WithRESTHost("test.org"),
			ably.WithFallbackHostsUseDefault(true))
		assert.Equal(t, "ws.test.org", clientOptions.GetRealtimeHost())
		assert.Equal(t, "test.org", clientOptions.GetRestHost())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Equal(t, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("RSC15g1 with fallbackHosts", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithFallbackHosts([]string{"a.example.com", "b.example.com"}))
		assert.Equal(t, "realtime.ably.io", clientOptions.GetRealtimeHost())
		assert.Equal(t, "rest.ably.io", clientOptions.GetRestHost())
		assert.False(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(t, 443, port)
		assert.True(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Equal(t, []string{"a.example.com", "b.example.com"}, fallbackHosts)
	})

	t.Run("RSC15b with fallbackHosts and fallbackHostsUseDefault", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(
			ably.WithFallbackHosts([]string{"a.example.com", "b.example.com"}),
			ably.WithFallbackHostsUseDefault(true))
		_, err := clientOptions.GetFallbackHosts()
		assert.Equal(t, err.Error(),
			"fallbackHosts and fallbackHostsUseDefault cannot both be set")
	})

	t.Run("RSC15b with fallbackHostsUseDefault And custom port", func(t *testing.T) {
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
