package ably_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
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
		assertDeepEquals(t, expectedFallBackHosts, hosts)
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
		assertDeepEquals(t, expectedFallBackHosts, hosts)
	})
}

func TestFallbackHosts_RSC15b(t *testing.T) {
	t.Run("RSC15e RSC15g3 with default options", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		assertEquals(t, "realtime.ably.io", clientOptions.GetRealtimeHost())
		assertEquals(t, "rest.ably.io", clientOptions.GetRestHost())
		assertFalse(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(t, 443, port)
		assertTrue(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertDeepEquals(t, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("RSC15h with production environment", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithEnvironment("production"))
		assertEquals(t, "realtime.ably.io", clientOptions.GetRealtimeHost())
		assertEquals(t, "rest.ably.io", clientOptions.GetRestHost())
		assertFalse(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(t, 443, port)
		assertTrue(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertDeepEquals(t, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("RSC15g2 RTC1e with custom environment", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithEnvironment("sandbox"))
		assertEquals(t, "sandbox-realtime.ably.io", clientOptions.GetRealtimeHost())
		assertEquals(t, "sandbox-rest.ably.io", clientOptions.GetRestHost())
		assertFalse(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(t, 443, port)
		assertTrue(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertDeepEquals(t, ably.GetEnvFallbackHosts("sandbox"), fallbackHosts)
	})

	t.Run("RSC15g4 RTC1e with custom environment and fallbackHostUseDefault", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithEnvironment("sandbox"), ably.WithFallbackHostsUseDefault(true))
		assertEquals(t, "sandbox-realtime.ably.io", clientOptions.GetRealtimeHost())
		assertEquals(t, "sandbox-rest.ably.io", clientOptions.GetRestHost())
		assertFalse(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(t, 443, port)
		assertTrue(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertDeepEquals(t, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("RSC11b RTN17b RTC1e with custom environment and non default ports", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(
			ably.WithEnvironment("local"),
			ably.WithPort(8080),
			ably.WithTLSPort(8081),
		)
		assertEquals(t, "local-realtime.ably.io", clientOptions.GetRealtimeHost())
		assertEquals(t, "local-rest.ably.io", clientOptions.GetRestHost())
		assertFalse(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(t, 8081, port)
		assertFalse(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertNil(t, fallbackHosts)
	})

	t.Run("RSC11 with custom rest host", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithRESTHost("test.org"))
		assertEquals(t, "test.org", clientOptions.GetRealtimeHost())
		assertEquals(t, "test.org", clientOptions.GetRestHost())
		assertFalse(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(t, 443, port)
		assertTrue(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertNil(t, fallbackHosts)
	})

	t.Run("RSC11 with custom rest host and realtime host", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithRealtimeHost("ws.test.org"), ably.WithRESTHost("test.org"))
		assertEquals(t, "ws.test.org", clientOptions.GetRealtimeHost())
		assertEquals(t, "test.org", clientOptions.GetRestHost())
		assertFalse(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(t, 443, port)
		assertTrue(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertNil(t, fallbackHosts)
	})

	t.Run("RSC15b with custom rest host and realtime host and fallbackHostsUseDefault", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(
			ably.WithRealtimeHost("ws.test.org"),
			ably.WithRESTHost("test.org"),
			ably.WithFallbackHostsUseDefault(true))
		assertEquals(t, "ws.test.org", clientOptions.GetRealtimeHost())
		assertEquals(t, "test.org", clientOptions.GetRestHost())
		assertFalse(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(t, 443, port)
		assertTrue(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertDeepEquals(t, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("RSC15g1 with fallbackHosts", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithFallbackHosts([]string{"a.example.com", "b.example.com"}))
		assertEquals(t, "realtime.ably.io", clientOptions.GetRealtimeHost())
		assertEquals(t, "rest.ably.io", clientOptions.GetRestHost())
		assertFalse(t, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assertEquals(t, 443, port)
		assertTrue(t, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assertDeepEquals(t, []string{"a.example.com", "b.example.com"}, fallbackHosts)
	})

	t.Run("RSC15b with fallbackHosts and fallbackHostsUseDefault", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(
			ably.WithFallbackHosts([]string{"a.example.com", "b.example.com"}),
			ably.WithFallbackHostsUseDefault(true))
		_, err := clientOptions.GetFallbackHosts()
		assertEquals(t, err.Error(), "fallbackHosts and fallbackHostsUseDefault cannot both be set")
	})

	t.Run("RSC15b with fallbackHostsUseDefault And custom port", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithTLSPort(8081), ably.WithFallbackHostsUseDefault(true))
		_, isDefaultPort := clientOptions.ActivePort()
		assertFalse(t, isDefaultPort)
		_, err := clientOptions.GetFallbackHosts()
		assertEquals(t, err.Error(), "fallbackHostsUseDefault cannot be set when port or tlsPort are set")

		clientOptions = ably.NewClientOptions(
			ably.WithTLS(false),
			ably.WithPort(8080),
			ably.WithFallbackHostsUseDefault(true))

		_, isDefaultPort = clientOptions.ActivePort()
		assertFalse(t, isDefaultPort)
		_, err = clientOptions.GetFallbackHosts()
		assertEquals(t, err.Error(), "fallbackHostsUseDefault cannot be set when port or tlsPort are set")
	})
}

func TestClientOptions(t *testing.T) {
	t.Run("must return error on invalid key", func(t *testing.T) {
		_, err := ably.NewREST([]ably.ClientOption{ably.WithKey("invalid")}...)
		if err == nil {
			t.Error("expected an error")
		}
	})
}

func TestScopeParams(t *testing.T) {
	t.Run("must error when given invalid range", func(t *testing.T) {
		t.Parallel()

		params := ably.ScopeParams{
			Start: time.Unix(0, 0).Add(123 * time.Millisecond),
			End:   time.Unix(0, 0).Add(122 * time.Millisecond),
		}
		err := params.EncodeValues(nil)
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("must set url values", func(t *testing.T) {
		t.Parallel()

		params := ably.ScopeParams{
			Start: time.Unix(0, 0).Add(122 * time.Millisecond),
			End:   time.Unix(0, 0).Add(123 * time.Millisecond),
		}
		u := make(url.Values)
		err := params.EncodeValues(&u)
		if err != nil {
			t.Fatal(err)
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
	t.Run("returns nil with no values", func(t *testing.T) {
		t.Parallel()

		params := ably.PaginateParams{}
		values := make(url.Values)
		err := params.EncodeValues(&values)
		if err != nil {
			t.Fatal(err)
		}
		encode := values.Encode()
		if encode != "" {
			t.Errorf("expected empty string got %s", encode)
		}
	})

	t.Run("returns the full params encoded", func(t *testing.T) {
		t.Parallel()

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
			t.Fatal(err)
		}
		expect := "direction=backwards&end=124&limit=1&start=123&unit=hello"
		got := values.Encode()
		if got != expect {
			t.Errorf("expected %s got %s", expect, got)
		}
	})

	t.Run("with value", func(t *testing.T) {
		t.Parallel()

		params := ably.PaginateParams{
			Limit:     10,
			Direction: "backwards",
		}
		values := make(url.Values)
		err := params.EncodeValues(&values)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with a value for ScopeParams", func(t *testing.T) {
		t.Parallel()

		values := make(url.Values)
		params := ably.PaginateParams{}
		params.Start = time.Unix(0, 0).Add(123 * time.Millisecond)
		err := params.EncodeValues(&values)
		if err != nil {
			t.Fatal(err)
		}
		start := values.Get("start")
		if start != "123" {
			t.Errorf("expected 123 got %s", start)
		}
	})
	t.Run("with invalid value for direction", func(t *testing.T) {
		t.Parallel()

		values := make(url.Values)
		params := ably.PaginateParams{}
		params.Direction = "unknown"
		err := params.EncodeValues(&values)
		if err == nil {
			t.Fatal("expected an error")
		}
	})
	t.Run("with invalid value for limit", func(t *testing.T) {
		t.Parallel()

		values := make(url.Values)
		params := ably.PaginateParams{}
		params.Limit = -1
		err := params.EncodeValues(&values)
		if err != nil {
			t.Fatal(err)
		}
		limit := values.Get("limit")
		if limit != "100" {
			t.Errorf("expected 100 got %s", limit)
		}
	})
}
