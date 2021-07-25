package ably_test

import (
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"
)

func Test_RSC15_RestHostFallback(t *testing.T) {
	t.Parallel()

	t.Run("RSC15a: should get fallback hosts in random order", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		restHosts := ably.NewRestHosts(clientOptions)
		// All expected hosts supposed to be tried upon
		expectedHosts := []string{
			"rest.ably.io",
			"a.ably-realtime.com",
			"b.ably-realtime.com",
			"c.ably-realtime.com",
			"d.ably-realtime.com",
			"e.ably-realtime.com",
		}

		// Get first preferred restHost
		var actualHosts []string
		prefHost := restHosts.GetPreferredHost()
		assertEquals(t, "rest.ably.io", prefHost)

		actualHosts = append(actualHosts, prefHost)

		// Get all fallback hosts in random order
		actualHosts = append(actualHosts, restHosts.GetAllRemainingFallbackHosts()...)

		assertNotDeepEquals(t, expectedHosts, actualHosts)
		assertDeepEquals(t, ablyutil.Sort(expectedHosts), ablyutil.Sort(actualHosts))
	})

	t.Run("RSC15a: should get fallback hosts when host is cached", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		restHosts := ably.NewRestHosts(clientOptions)
		// All expected hosts supposed to be tried upon
		expectedHosts := []string{
			"rest.ably.io",
			"a.ably-realtime.com",
			"b.ably-realtime.com",
			"c.ably-realtime.com",
			"d.ably-realtime.com",
			"e.ably-realtime.com",
		}

		// cache the restHosts
		restHosts.CacheHost("b.ably-realtime.com")

		// Get first preferred restHost
		var actualHosts []string
		prefHost := restHosts.GetPreferredHost()
		assertEquals(t, "b.ably-realtime.com", prefHost)

		actualHosts = append(actualHosts, prefHost)

		// Get all fallback hosts in random order
		actualHosts = append(actualHosts, restHosts.GetAllRemainingFallbackHosts()...)

		assertNotDeepEquals(t, expectedHosts, actualHosts)
		assertDeepEquals(t, ablyutil.Sort(expectedHosts), ablyutil.Sort(actualHosts))
	})

	t.Run("RSC15a: should get all fallback hosts again, when visited hosts are cleared after reconnection", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		restHosts := ably.NewRestHosts(clientOptions)
		// All expected hosts supposed to be tried upon
		expectedHosts := []string{
			"rest.ably.io",
			"a.ably-realtime.com",
			"b.ably-realtime.com",
			"c.ably-realtime.com",
			"d.ably-realtime.com",
			"e.ably-realtime.com",
		}

		// Get first preferred restHost
		var actualHosts []string
		restHosts.GetPreferredHost()

		// Get some fallback hosts
		restHosts.NextFallbackHost()
		restHosts.NextFallbackHost()

		// Clear visited hosts, after reconnection
		restHosts.ResetVisitedFallbackHosts()

		// Get all fallback hosts in random order
		actualHosts = append(actualHosts, restHosts.GetAllRemainingFallbackHosts()...)

		assertNotDeepEquals(t, expectedHosts, actualHosts)
		assertDeepEquals(t, ablyutil.Sort(expectedHosts), ablyutil.Sort(actualHosts))
	})

	t.Run("RSC15a: should get all fallback hosts, including primary host when preferred host is not requested", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		restHosts := ably.NewRestHosts(clientOptions)
		// All expected hosts supposed to be tried upon
		expectedHosts := []string{
			"rest.ably.io",
			"a.ably-realtime.com",
			"b.ably-realtime.com",
			"c.ably-realtime.com",
			"d.ably-realtime.com",
			"e.ably-realtime.com",
		}

		var actualHosts []string

		// Get all fallback hosts in random order
		actualHosts = append(actualHosts, restHosts.GetAllRemainingFallbackHosts()...)

		assertNotDeepEquals(t, expectedHosts, actualHosts)
		assertDeepEquals(t, ablyutil.Sort(expectedHosts), ablyutil.Sort(actualHosts))
	})

	t.Run("RSC15e: should return primary host if not cached", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		restHosts := ably.NewRestHosts(clientOptions)
		prefHost := restHosts.GetPreferredHost()
		assertEquals(t, "rest.ably.io", prefHost)
	})

	t.Run("RSC15e, RSC15f: should return cached host when set", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		restHosts := ably.NewRestHosts(clientOptions)
		restHosts.CacheHost("custom-ably.rest")
		prefHost := restHosts.GetPreferredHost()
		assertEquals(t, "custom-ably.rest", prefHost)
	})

	t.Run("should get remaining fallback hosts count", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		restHosts := ably.NewRealtimeHosts(clientOptions)

		// Get first preferred restHost
		restHosts.GetPreferredHost()

		assertEquals(t, 5, restHosts.FallbackHostsRemaining())
		// Get some fallback hosts
		restHosts.NextFallbackHost()
		assertEquals(t, 4, restHosts.FallbackHostsRemaining())

		restHosts.NextFallbackHost()
		assertEquals(t, 3, restHosts.FallbackHostsRemaining())

		restHosts.NextFallbackHost()
		restHosts.NextFallbackHost()
		restHosts.NextFallbackHost()

		assertEquals(t, 0, restHosts.FallbackHostsRemaining())
	})
}

func Test_RTN17_RealtimeHostFallback(t *testing.T) {
	t.Parallel()
	t.Run("RTN17a: should always get primary host as pref. host", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		realtimeHosts := ably.NewRealtimeHosts(clientOptions)
		prefHost := realtimeHosts.GetPreferredHost()
		assertEquals(t, "realtime.ably.io", prefHost)
	})

	t.Run("RTN17a: should always get primary host as pref. host", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithRealtimeHost("custom-realtime.ably.io"))
		realtimeHosts := ably.NewRealtimeHosts(clientOptions)
		prefHost := realtimeHosts.GetPreferredHost()
		assertEquals(t, "custom-realtime.ably.io", prefHost)
	})

	t.Run("RTN17c, RSC15g: should get fallback hosts in random order", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		realtimeHosts := ably.NewRealtimeHosts(clientOptions)
		// All expected hosts supposed to be tried upon
		expectedHosts := []string{
			"realtime.ably.io",
			"a.ably-realtime.com",
			"b.ably-realtime.com",
			"c.ably-realtime.com",
			"d.ably-realtime.com",
			"e.ably-realtime.com",
		}

		// Get first preferred restHost
		var actualHosts []string
		prefHost := realtimeHosts.GetPreferredHost()
		actualHosts = append(actualHosts, prefHost)

		// Get all fallback hosts in random order
		actualHosts = append(actualHosts, realtimeHosts.GetAllRemainingFallbackHosts()...)

		assertNotDeepEquals(t, expectedHosts, actualHosts)
		assertDeepEquals(t, ablyutil.Sort(expectedHosts), ablyutil.Sort(actualHosts))
	})

	t.Run("RTN17c, RSC15g: should get all fallback hosts again, when visited hosts are cleared after reconnection", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		realtimeHosts := ably.NewRealtimeHosts(clientOptions)
		// All expected hosts supposed to be tried upon
		expectedHosts := []string{
			"realtime.ably.io",
			"a.ably-realtime.com",
			"b.ably-realtime.com",
			"c.ably-realtime.com",
			"d.ably-realtime.com",
			"e.ably-realtime.com",
		}

		// Get first preferred restHost
		var actualHosts []string
		realtimeHosts.GetPreferredHost()

		// Get some fallback hosts
		realtimeHosts.NextFallbackHost()
		realtimeHosts.NextFallbackHost()

		// Clear visited hosts, after reconnection
		realtimeHosts.ResetVisitedFallbackHosts()

		// Get all fallback hosts in random order
		actualHosts = append(actualHosts, realtimeHosts.GetAllRemainingFallbackHosts()...)

		assertNotDeepEquals(t, expectedHosts, actualHosts)
		assertDeepEquals(t, ablyutil.Sort(expectedHosts), ablyutil.Sort(actualHosts))
	})

	t.Run("RTN17c, RSC15g: should get all fallback hosts, including primary host when preferred host is not requested", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		realtimeHosts := ably.NewRealtimeHosts(clientOptions)
		// All expected hosts supposed to be tried upon
		expectedHosts := []string{
			"realtime.ably.io",
			"a.ably-realtime.com",
			"b.ably-realtime.com",
			"c.ably-realtime.com",
			"d.ably-realtime.com",
			"e.ably-realtime.com",
		}

		var actualHosts []string

		// Get all fallback hosts in random order
		actualHosts = append(actualHosts, realtimeHosts.GetAllRemainingFallbackHosts()...)

		assertNotDeepEquals(t, expectedHosts, actualHosts)
		assertDeepEquals(t, ablyutil.Sort(expectedHosts), ablyutil.Sort(actualHosts))
	})

	t.Run("RTN17e: rest host should use active realtime host as pref. host", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		restHosts := ably.NewRestHosts(clientOptions)
		restHosts.SetPrimaryFallbackHost("custom-ably.realtime") // set by realtime in accordance with active connection with given host
		prefHost := restHosts.GetPreferredHost()
		assertEquals(t, "custom-ably.realtime", prefHost)
	})

	t.Run("should get remaining fallback hosts count", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		realtimeHosts := ably.NewRealtimeHosts(clientOptions)

		// Get first preferred restHost
		realtimeHosts.GetPreferredHost()

		assertEquals(t, 5, realtimeHosts.FallbackHostsRemaining())
		// Get some fallback hosts
		realtimeHosts.NextFallbackHost()
		assertEquals(t, 4, realtimeHosts.FallbackHostsRemaining())

		realtimeHosts.NextFallbackHost()
		assertEquals(t, 3, realtimeHosts.FallbackHostsRemaining())

		realtimeHosts.NextFallbackHost()
		realtimeHosts.NextFallbackHost()
		realtimeHosts.NextFallbackHost()

		assertEquals(t, 0, realtimeHosts.FallbackHostsRemaining())
	})
}
