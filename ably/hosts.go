package ably

import (
	"context"
	"sync"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
)

// RSC15
type restHosts struct {
	primaryFallbackHost string
	opts                *clientOptions
	cache               *hostCache
	visitedHosts        ablyutil.HashSet
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
		cachedHost := restHosts.cache.get()
		if !ablyutil.Empty(cachedHost) && !visitedHosts.Has(cachedHost) {
			return cachedHost
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

func (restHosts *restHosts) setPrimaryFallbackHost(host string) {
	restHosts.Lock()
	defer restHosts.Unlock()
	restHosts.primaryFallbackHost = host
}

func (restHosts *restHosts) getPreferredHost() string {
	restHosts.Lock()
	defer restHosts.Unlock()
	host := restHosts.cache.get()
	if ablyutil.Empty(host) {
		if ablyutil.Empty(restHosts.primaryFallbackHost) {
			host = restHosts.getPrimaryHost()
		} else {
			host = restHosts.primaryFallbackHost
		}
	}
	restHosts.visitedHosts.Add(host)
	return host
}

func (restHosts *restHosts) cacheHost(host string) {
	select {
	case <-restHosts.cache.put(host):
		return
	case <-time.After(time.Second): // timeout of a second to cache the host
		return
	}
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
		primaryHost := realtimeHosts.getPrimaryHost()
		if !visitedHosts.Has(primaryHost) {
			return primaryHost
		}
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
	running  bool
	host     string
	duration time.Duration
	cancel   func()
	mu       sync.RWMutex
}

func (f *hostCache) put(host string) (isCached chan struct{}) {
	isCached = make(chan struct{}, 1)
	if f.get() != host {
		if f.isRunning() {
			f.stop()
		}
		go f.run(host, isCached)
		return
	}
	isCached <- struct{}{}
	return
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

func (f *hostCache) run(host string, isCached chan struct{}) {
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
	isCached <- struct{}{}
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
