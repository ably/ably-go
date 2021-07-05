package ably

import (
	"context"
	"sync"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
)

//type hosts interface {
//	getPrimaryHost() string
//	getFallbackHost() string
//	resetVisitedFallbackHosts()
//	fallbackHostsRemaining() int
//	// Cached host in case of rest Fallback hosts
//	setPrimaryFallbackHost(host string)
//	getPreferredHost() string
//	cacheHost(host string)
//}

// RSC15
type restHosts struct {
	primaryFallbackHost string
	opts                *clientOptions
	cache               *hostCache
	visitedHosts        []string
}

func newRestHosts(opts *clientOptions) *restHosts {
	return &restHosts{
		opts: opts,
		cache: &hostCache{
			duration: opts.fallbackRetryTimeout(),
		},
	}
}

func (fallbackHosts *restHosts) getPrimaryHost() string {
	return fallbackHosts.opts.getPrimaryRestHost()
}

func (fallbackHosts *restHosts) getFallbackHost() string {
	hosts, _ := fallbackHosts.opts.getFallbackHosts()
	shuffledFallbackHosts := ablyutil.Shuffle(hosts)
	getNonVisitedHost := func() string {
		visitedHosts := fallbackHosts.visitedHosts
		if !ablyutil.Contains(visitedHosts, fallbackHosts.getPrimaryHost()) {
			return fallbackHosts.getPrimaryHost()
		}
		for _, host := range shuffledFallbackHosts {
			if !ablyutil.Contains(visitedHosts, host) {
				return host
			}
		}
		return ""
	}
	nonVisitedHost := getNonVisitedHost()
	if !ablyutil.Empty(nonVisitedHost) {
		fallbackHosts.visitedHosts = append(fallbackHosts.visitedHosts, nonVisitedHost)
	}
	return nonVisitedHost
}

func (fallbackHosts *restHosts) resetVisitedFallbackHosts() {
	fallbackHosts.visitedHosts = nil
}

func (fallbackHosts *restHosts) fallbackHostsRemaining() int {
	hosts, _ := fallbackHosts.opts.getFallbackHosts()
	return len(hosts) + 1 - len(fallbackHosts.visitedHosts)
}

func (fallbackHosts *restHosts) setPrimaryFallbackHost(host string) {
	fallbackHosts.primaryFallbackHost = host
}

func (fallbackHosts *restHosts) getPreferredHost() string {
	host := fallbackHosts.cache.get()
	if ablyutil.Empty(host) {
		if ablyutil.Empty(fallbackHosts.primaryFallbackHost) {
			host = fallbackHosts.getPrimaryHost()
		} else {
			host = fallbackHosts.primaryFallbackHost
		}
	}
	if !ablyutil.Contains(fallbackHosts.visitedHosts, host) {
		fallbackHosts.visitedHosts = append(fallbackHosts.visitedHosts, host)
	}
	return host
}

func (fallbackHosts *restHosts) cacheHost(host string) {
	fallbackHosts.cache.put(host)
}

// RTN17
type realtimeHosts struct {
	opts         *clientOptions
	visitedHosts []string
}

func newRealtimeHosts(opts *clientOptions) *realtimeHosts {
	return &realtimeHosts{
		opts: opts,
	}
}

func (fallbackHosts *realtimeHosts) getPrimaryHost() string {
	return fallbackHosts.opts.getPrimaryRealtimeHost()
}

func (fallbackHosts *realtimeHosts) getFallbackHost() string {
	hosts, _ := fallbackHosts.opts.getFallbackHosts()
	shuffledFallbackHosts := ablyutil.Shuffle(hosts)
	getNonVisitedHost := func() string {
		visitedHosts := fallbackHosts.visitedHosts
		if !ablyutil.Contains(visitedHosts, fallbackHosts.getPrimaryHost()) {
			return fallbackHosts.getPrimaryHost()
		}
		for _, host := range shuffledFallbackHosts {
			if !ablyutil.Contains(visitedHosts, host) {
				return host
			}
		}
		return ""
	}
	nonVisitedHost := getNonVisitedHost()
	if !ablyutil.Empty(nonVisitedHost) {
		fallbackHosts.visitedHosts = append(fallbackHosts.visitedHosts, nonVisitedHost)
	}
	return nonVisitedHost
}

func (fallbackHosts *realtimeHosts) resetVisitedFallbackHosts() {
	fallbackHosts.visitedHosts = nil
}

func (fallbackHosts *realtimeHosts) fallbackHostsRemaining() int {
	hosts, _ := fallbackHosts.opts.getFallbackHosts()
	return len(hosts) + 1 - len(fallbackHosts.visitedHosts)
}

func (fallbackHosts *realtimeHosts) getPreferredHost() string {
	host := fallbackHosts.getPrimaryHost() // primary host is always preferred host/ fallback host in realtime
	if !ablyutil.Contains(fallbackHosts.visitedHosts, host) {
		fallbackHosts.visitedHosts = append(fallbackHosts.visitedHosts, host)
	}
	return host
}

// hostCache this caches a successful fallback host for 10 minutes.
// Only used by REST client while making requests RSC15f
type hostCache struct {
	running  bool
	host     string
	duration time.Duration
	cancel   func()
	mu       sync.RWMutex
}

func (f *hostCache) put(host string) {
	if f.get() != host {
		if f.isRunning() {
			f.stop()
		}
		go f.run(host)
	}
}

func (f *hostCache) get() string {
	if f.isRunning() {
		f.mu.RLock()
		h := f.host
		f.mu.RUnlock()
		return h
	}
	return ""
}

func (f *hostCache) isRunning() bool {
	f.mu.RLock()
	v := f.running
	f.mu.RUnlock()
	return v
}

func (f *hostCache) run(host string) {
	f.mu.Lock()
	now := time.Now()
	duration := defaultOptions.FallbackRetryTimeout // spec RSC15f
	if f.duration != 0 {
		duration = f.duration
	}
	ctx, cancel := context.WithDeadline(context.Background(), now.Add(duration))
	f.running = true
	f.host = host
	f.cancel = cancel
	f.mu.Unlock()
	<-ctx.Done()
	f.mu.Lock()
	f.running = false
	f.mu.Unlock()
}

func (f *hostCache) stop() {
	f.cancel()
	// we make sure we have stopped
	for {
		if !f.isRunning() {
			return
		}
	}
}
