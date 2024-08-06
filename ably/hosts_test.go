package ably_test

import (
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
)

func Test_RTN17_RealtimeHostFallback(t *testing.T) {
	t.Parallel()
	t.Run("RTN17a: should always get primary host as pref. host", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		realtimeHosts := ably.NewRealtimeHosts(clientOptions)
		prefHost := realtimeHosts.GetPreferredHost()
		assert.Equal(t, "realtime.ably.io", prefHost)
	})

	t.Run("RTN17a: should always get primary host as pref. host", func(t *testing.T) {
		clientOptions := ably.NewClientOptions(ably.WithRealtimeHost("custom-realtime.ably.io"))
		realtimeHosts := ably.NewRealtimeHosts(clientOptions)
		prefHost := realtimeHosts.GetPreferredHost()
		assert.Equal(t, "custom-realtime.ably.io", prefHost)
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

		assert.ElementsMatch(t, expectedHosts, actualHosts)
	})

	t.Run("RTN17c, RSC15g: should get all fallback hosts again, when visited hosts are cleared after reconnection", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		realtimeHosts := ably.NewRealtimeHosts(clientOptions)
		// All expected hosts supposed to be tried upon
		expectedHosts := []string{
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

		assert.ElementsMatch(t, expectedHosts, actualHosts)
	})

	t.Run("RTN17c, RSC15g: should get all fallback hosts when preferred host is not requested", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		realtimeHosts := ably.NewRealtimeHosts(clientOptions)
		// All expected hosts supposed to be tried upon
		expectedHosts := []string{
			"a.ably-realtime.com",
			"b.ably-realtime.com",
			"c.ably-realtime.com",
			"d.ably-realtime.com",
			"e.ably-realtime.com",
		}

		var actualHosts []string

		// Get all fallback hosts in random order
		actualHosts = append(actualHosts, realtimeHosts.GetAllRemainingFallbackHosts()...)

		assert.ElementsMatch(t, expectedHosts, actualHosts)
	})

	t.Run("should get remaining fallback hosts count", func(t *testing.T) {
		clientOptions := ably.NewClientOptions()
		realtimeHosts := ably.NewRealtimeHosts(clientOptions)

		// Get first preferred restHost
		realtimeHosts.GetPreferredHost()

		assert.Equal(t, 5, realtimeHosts.FallbackHostsRemaining())
		// Get some fallback hosts
		realtimeHosts.NextFallbackHost()
		assert.Equal(t, 4, realtimeHosts.FallbackHostsRemaining())

		realtimeHosts.NextFallbackHost()
		assert.Equal(t, 3, realtimeHosts.FallbackHostsRemaining())

		realtimeHosts.NextFallbackHost()
		realtimeHosts.NextFallbackHost()
		realtimeHosts.NextFallbackHost()

		assert.Equal(t, 0, realtimeHosts.FallbackHostsRemaining())
	})
}
