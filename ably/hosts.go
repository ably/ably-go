package ably

import (
	"context"
	"sync"
	"time"
)

type hosts interface {
	getPrimaryHost() string
	getFallbackHost() string
	//resetVisitedFallbackHosts()
	//fallbackHostsRemaining() int
	// Cached host in case of rest Fallback hosts
	//getPreferredFallbackHost() string
	setPreferredFallbackHost(host string)
}

// RSC15
type restHosts struct {
	opts         *clientOptions
	cache        *hostCache
	visitedHosts []string
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
	return fallbackHosts.opts.getRestHost()
}

func (fallbackHosts *restHosts) getFallbackHost() string {
	return fallbackHosts.cache.get()
}

func (fallbackHosts *restHosts) setPreferredFallbackHost(host string) {
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
	return fallbackHosts.opts.getRealtimeHost()
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
