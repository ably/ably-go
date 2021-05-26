package ably_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
)

func TestDefaultFallbacks_RSC15h(t *testing.T) {
	t.Parallel()
	t.Run("with env should return environment fallback hosts", func(ts *testing.T) {
		expectedFallBackHosts := []string{
			"a.ably-realtime.com",
			"b.ably-realtime.com",
			"c.ably-realtime.com",
			"d.ably-realtime.com",
			"e.ably-realtime.com",
		}
		hosts := ably.DefaultFallbackHosts()
		assertDeepEquals(ts, expectedFallBackHosts, hosts)
	})
}

func TestEnvFallbackHosts_RSC15i(t *testing.T) {
	t.Parallel()
	t.Run("with env should return environment fallback hosts", func(ts *testing.T) {
		expectedFallBackHosts := []string{
			"sandbox-a-fallback.ably-realtime.com",
			"sandbox-b-fallback.ably-realtime.com",
			"sandbox-c-fallback.ably-realtime.com",
			"sandbox-d-fallback.ably-realtime.com",
			"sandbox-e-fallback.ably-realtime.com",
		}
		hosts := ably.GetEnvFallbackHosts("sandbox")
		assertDeepEquals(ts, expectedFallBackHosts, hosts)
	})
}

func TestFallbackHosts_RSC15b(t *testing.T) {
	t.Parallel()
	t.Run("RSC15e RSC15g3 with default options", func(ts *testing.T) {
		clientOptions := ably.NewClientOptions("")
		assertEquals(ts, "realtime.ably.io", clientOptions.GetRealtimeHost())
		assertEquals(ts, "rest.ably.io", clientOptions.GetRestHost())
		assertFalse(ts, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(ts, 443, port)
		assertTrue(ts, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertDeepEquals(ts, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("RSC15h with production environment", func(ts *testing.T) {
		clientOptions := ably.NewClientOptions("")
		clientOptions.Environment = "production"
		clientOptions.Environment = "production"
		assertEquals(ts, "realtime.ably.io", clientOptions.GetRealtimeHost())
		assertEquals(ts, "rest.ably.io", clientOptions.GetRestHost())
		assertFalse(ts, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(ts, 443, port)
		assertTrue(ts, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertDeepEquals(ts, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("RSC15g2 RTC1e with custom environment", func(ts *testing.T) {
		clientOptions := ably.NewClientOptions("")
		clientOptions.Environment = "sandbox"
		assertEquals(ts, "sandbox-realtime.ably.io", clientOptions.GetRealtimeHost())
		assertEquals(ts, "sandbox-rest.ably.io", clientOptions.GetRestHost())
		assertFalse(ts, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(ts, 443, port)
		assertTrue(ts, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertDeepEquals(ts, ably.GetEnvFallbackHosts("sandbox"), fallbackHosts)
	})

	t.Run("RSC15g4 RTC1e with custom environment and fallbackHostUseDefault", func(ts *testing.T) {
		clientOptions := ably.NewClientOptions("")
		clientOptions.Environment = "sandbox"
		clientOptions.FallbackHostsUseDefault = true
		assertEquals(ts, "sandbox-realtime.ably.io", clientOptions.GetRealtimeHost())
		assertEquals(ts, "sandbox-rest.ably.io", clientOptions.GetRestHost())
		assertFalse(ts, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(ts, 443, port)
		assertTrue(ts, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertDeepEquals(ts, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("RSC11b RTN17b RTC1e with custom environment and non default ports", func(ts *testing.T) {
		clientOptions := ably.NewClientOptions("")
		clientOptions.Environment = "local"
		clientOptions.Port = 8080
		clientOptions.TLSPort = 8081
		assertEquals(ts, "local-realtime.ably.io", clientOptions.GetRealtimeHost())
		assertEquals(ts, "local-rest.ably.io", clientOptions.GetRestHost())
		assertFalse(ts, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(ts, 8081, port)
		assertFalse(ts, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertNil(ts, fallbackHosts)
	})

	t.Run("RSC11 with custom rest host", func(ts *testing.T) {
		clientOptions := ably.NewClientOptions("")
		clientOptions.RestHost = "test.org"
		assertEquals(ts, "test.org", clientOptions.GetRealtimeHost())
		assertEquals(ts, "test.org", clientOptions.GetRestHost())
		assertFalse(ts, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(ts, 443, port)
		assertTrue(ts, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertNil(ts, fallbackHosts)
	})

	t.Run("RSC11 with custom rest host and realtime host", func(ts *testing.T) {
		clientOptions := ably.NewClientOptions("")
		clientOptions.RealtimeHost = "ws.test.org"
		clientOptions.RestHost = "test.org"
		assertEquals(ts, "ws.test.org", clientOptions.GetRealtimeHost())
		assertEquals(ts, "test.org", clientOptions.GetRestHost())
		assertFalse(ts, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(ts, 443, port)
		assertTrue(ts, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertNil(ts, fallbackHosts)
	})

	t.Run("RSC15b with custom rest host and realtime host and fallbackHostsUseDefault", func(ts *testing.T) {
		clientOptions := ably.NewClientOptions("")
		clientOptions.RealtimeHost = "ws.test.org"
		clientOptions.RestHost = "test.org"
		clientOptions.FallbackHostsUseDefault = true
		assertEquals(ts, "ws.test.org", clientOptions.GetRealtimeHost())
		assertEquals(ts, "test.org", clientOptions.GetRestHost())
		assertFalse(ts, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(ts, 443, port)
		assertTrue(ts, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertDeepEquals(ts, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("RSC15g1 with fallbackHosts", func(ts *testing.T) {
		clientOptions := ably.NewClientOptions("")
		clientOptions.FallbackHosts = []string{"a.example.com", "b.example.com"}
		assertEquals(ts, "realtime.ably.io", clientOptions.GetRealtimeHost())
		assertEquals(ts, "rest.ably.io", clientOptions.GetRestHost())
		assertFalse(ts, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(ts, 443, port)
		assertTrue(ts, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertDeepEquals(ts, []string{"a.example.com", "b.example.com"}, fallbackHosts)
	})

	t.Run("RSC15b with fallbackHosts and fallbackHostsUseDefault", func(ts *testing.T) {
		clientOptions := ably.NewClientOptions("")
		clientOptions.FallbackHosts = []string{"a.example.com", "b.example.com"}
		clientOptions.FallbackHostsUseDefault = true
		_, err := clientOptions.GetFallbackHosts()
		assertEquals(ts, err.Error(), "fallbackHosts and fallbackHostsUseDefault cannot both be set")
	})

	t.Run("RSC15b with fallbackHostsUseDefault And custom port", func(ts *testing.T) {
		clientOptions := ably.NewClientOptions("")
		clientOptions.TLSPort = 8081
		clientOptions.FallbackHostsUseDefault = true
		_, isDefaultPort := clientOptions.ActivePort()
		assertFalse(ts, isDefaultPort)
		_, err := clientOptions.GetFallbackHosts()
		assertEquals(ts, err.Error(), "fallbackHostsUseDefault cannot be set when port or tlsPort are set")

		clientOptions.NoTLS = true
		clientOptions.Port = 8080
		clientOptions.FallbackHostsUseDefault = true
		_, isDefaultPort = clientOptions.ActivePort()
		assertFalse(ts, isDefaultPort)
		_, err = clientOptions.GetFallbackHosts()
		assertEquals(ts, err.Error(), "fallbackHostsUseDefault cannot be set when port or tlsPort are set")
	})
}

func TestClientOptions(t *testing.T) {
	t.Parallel()
	t.Run("must return error on invalid key", func(ts *testing.T) {
		_, err := ably.NewREST([]ably.ClientOption{ably.WithKey("invalid")}...)
		if err == nil {
			ts.Error("expected an error")
		}
	})
}

func TestScopeParams(t *testing.T) {
	t.Parallel()
	t.Run("must error when given invalid range", func(ts *testing.T) {
		params := ably.ScopeParams{
			Start: time.Unix(0, 0).Add(123 * time.Millisecond),
			End:   time.Unix(0, 0).Add(122 * time.Millisecond),
		}
		err := params.EncodeValues(nil)
		if err == nil {
			ts.Fatal("expected an error")
		}
	})

	t.Run("must set url values", func(ts *testing.T) {
		params := ably.ScopeParams{
			Start: time.Unix(0, 0).Add(122 * time.Millisecond),
			End:   time.Unix(0, 0).Add(123 * time.Millisecond),
		}
		u := make(url.Values)
		err := params.EncodeValues(&u)
		if err != nil {
			ts.Fatal(err)
		}
		start := u.Get("start")
		end := u.Get("end")
		if start != "122" {
			t.Errorf("expected 122 got %s", start)
		}
		if end != "123" {
			t.Errorf("expected 123 got %s", end)
		}
	})
}

func TestPaginateParams(t *testing.T) {
	t.Parallel()
	t.Run("returns nil with no values", func(ts *testing.T) {
		params := ably.PaginateParams{}
		values := make(url.Values)
		err := params.EncodeValues(&values)
		if err != nil {
			ts.Fatal(err)
		}
		encode := values.Encode()
		if encode != "" {
			t.Errorf("expected empty string got %s", encode)
		}
	})

	t.Run("returns the full params encoded", func(ts *testing.T) {
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
		if err != nil {
			ts.Fatal(err)
		}
		expect := "direction=backwards&end=124&limit=1&start=123&unit=hello"
		got := values.Encode()
		if got != expect {
			t.Errorf("expected %s got %s", expect, got)
		}
	})

	t.Run("with value", func(ts *testing.T) {
		params := ably.PaginateParams{
			Limit:     10,
			Direction: "backwards",
		}
		values := make(url.Values)
		err := params.EncodeValues(&values)
		if err != nil {
			ts.Fatal(err)
		}
	})

	t.Run("with a value for ScopeParams", func(ts *testing.T) {
		values := make(url.Values)
		params := ably.PaginateParams{}
		params.Start = time.Unix(0, 0).Add(123 * time.Millisecond)
		err := params.EncodeValues(&values)
		if err != nil {
			ts.Fatal(err)
		}
		start := values.Get("start")
		if start != "123" {
			ts.Errorf("expected 123 got %s", start)
		}
	})
	t.Run("with invalid value for direction", func(ts *testing.T) {
		values := make(url.Values)
		params := ably.PaginateParams{}
		params.Direction = "unknown"
		err := params.EncodeValues(&values)
		if err == nil {
			ts.Fatal("expected an error")
		}
	})
	t.Run("with invalid value for limit", func(ts *testing.T) {
		values := make(url.Values)
		params := ably.PaginateParams{}
		params.Limit = -1
		err := params.EncodeValues(&values)
		if err != nil {
			ts.Fatal(err)
		}
		limit := values.Get("limit")
		if limit != "100" {
			t.Errorf("expected 100 got %s", limit)
		}
	})
}
