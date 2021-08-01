package ably

import (
	"sync"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
)

// RSC15
type restHosts struct {
	activeRealtimeFallbackHost string // RTN17e
	opts                       *clientOptions
	cache                      *hostCache
	visitedHosts               ablyutil.HashSet
	sync.Mutex
}

func newRestHosts(opts *clientOptions) *restHosts {
	return &restHosts{
		opts: opts,
		cache: &hostCache{
			duration: opts.fallbackRetryTimeout(),
		},
		visitedHosts: ablyutil.NewHashSet(),
	}
}

func (restHosts *restHosts) getPrimaryHost() string {
	return restHosts.opts.getPrimaryRestHost()
}

func (restHosts *restHosts) nextFallbackHost() string {
	restHosts.Lock()
	defer restHosts.Unlock()

	getNonVisitedHost := func() string {
		visitedHosts := restHosts.visitedHosts
		activeRealtimeHost := restHosts.activeRealtimeFallbackHost
		if !ablyutil.Empty(activeRealtimeHost) && !visitedHosts.Has(activeRealtimeHost) {
			return activeRealtimeHost
		}
		primaryHost := restHosts.getPrimaryHost()
		if !visitedHosts.Has(primaryHost) {
			return primaryHost
		}
		hosts, _ := restHosts.opts.getFallbackHosts()
		shuffledFallbackHosts := ablyutil.Shuffle(hosts)
		for _, host := range shuffledFallbackHosts {
			if !visitedHosts.Has(host) {
				return host
			}
		}
		return ""
	}

	nonVisitedHost := getNonVisitedHost()
	if !ablyutil.Empty(nonVisitedHost) {
		restHosts.visitedHosts.Add(nonVisitedHost)
	}
	return nonVisitedHost
}

func (restHosts *restHosts) resetVisitedFallbackHosts() {
	restHosts.Lock()
	defer restHosts.Unlock()
	restHosts.visitedHosts = ablyutil.NewHashSet()
}

func (restHosts *restHosts) fallbackHostsRemaining() int {
	restHosts.Lock()
	defer restHosts.Unlock()
	hosts, _ := restHosts.opts.getFallbackHosts()
	return len(hosts) + 1 - len(restHosts.visitedHosts)
}

func (restHosts *restHosts) setActiveRealtimeFallbackHost(host string) {
	restHosts.Lock()
	defer restHosts.Unlock()
	restHosts.activeRealtimeFallbackHost = host
}

func (restHosts *restHosts) getPreferredHost() string {
	restHosts.Lock()
	defer restHosts.Unlock()
	host := restHosts.cache.get()
	if ablyutil.Empty(host) {
		if ablyutil.Empty(restHosts.activeRealtimeFallbackHost) {
			host = restHosts.getPrimaryHost()
		} else {
			host = restHosts.activeRealtimeFallbackHost
		}
	}
	restHosts.visitedHosts.Add(host)
	return host
}

func (restHosts *restHosts) cacheHost(host string) {
	restHosts.cache.put(host)
}

// RTN17
type realtimeHosts struct {
	opts         *clientOptions
	visitedHosts ablyutil.HashSet
	sync.Mutex
}

func newRealtimeHosts(opts *clientOptions) *realtimeHosts {
	return &realtimeHosts{
		opts:         opts,
		visitedHosts: ablyutil.NewHashSet(),
	}
}

func (realtimeHosts *realtimeHosts) getPrimaryHost() string {
	return realtimeHosts.opts.getPrimaryRealtimeHost()
}

func (realtimeHosts *realtimeHosts) nextFallbackHost() string {
	realtimeHosts.Lock()
	defer realtimeHosts.Unlock()

	getNonVisitedHost := func() string {
		visitedHosts := realtimeHosts.visitedHosts
		hosts, _ := realtimeHosts.opts.getFallbackHosts()
		shuffledFallbackHosts := ablyutil.Shuffle(hosts)
		for _, host := range shuffledFallbackHosts {
			if !visitedHosts.Has(host) {
				return host
			}
		}
		return ""
	}

	nonVisitedHost := getNonVisitedHost()
	if !ablyutil.Empty(nonVisitedHost) {
		realtimeHosts.visitedHosts.Add(nonVisitedHost)
	}
	return nonVisitedHost
}

func (realtimeHosts *realtimeHosts) resetVisitedFallbackHosts() {
	realtimeHosts.Lock()
	defer realtimeHosts.Unlock()
	realtimeHosts.visitedHosts = ablyutil.NewHashSet()
}

func (realtimeHosts *realtimeHosts) fallbackHostsRemaining() int {
	realtimeHosts.Lock()
	defer realtimeHosts.Unlock()
	hosts, _ := realtimeHosts.opts.getFallbackHosts()
	return len(hosts) + 1 - len(realtimeHosts.visitedHosts)
}

func (realtimeHosts *realtimeHosts) getPreferredHost() string {
	realtimeHosts.Lock()
	defer realtimeHosts.Unlock()
	host := realtimeHosts.getPrimaryHost() // primary host is always preferred host/ fallback host in realtime
	realtimeHosts.visitedHosts.Add(host)
	return host
}

// hostCache this caches a successful fallback host for 10 minutes.
// Only used by REST client while making requests RSC15f
type hostCache struct {
	duration time.Duration

	sync.RWMutex
	deadline time.Time
	host     string
}

func (c *hostCache) put(host string) {
	c.Lock()
	defer c.Unlock()
	c.host = host
	c.deadline = time.Now().Add(c.duration)
}

func (c *hostCache) get() string {
	c.RLock()
	defer c.RUnlock()
	if ablyutil.Empty(c.host) || time.Until(c.deadline) <= 0 {
		return ""
	}
	return c.host
}
