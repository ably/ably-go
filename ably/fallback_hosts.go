package ably

import (
	"context"
	"sync"
	"time"
)

// restFallbackCache this caches a successful fallback host for 10 minutes.
// Only used by REST client while making requests RSC15f
type restFallbackCache struct {
	running  bool
	host     string
	duration time.Duration
	cancel   func()
	mu       sync.RWMutex
}

func (f *restFallbackCache) put(host string) {
	if f.get() != host {
		if f.isRunning() {
			f.stop()
		}
		go f.run(host)
	}
}

func (f *restFallbackCache) get() string {
	if f.isRunning() {
		f.mu.RLock()
		h := f.host
		f.mu.RUnlock()
		return h
	}
	return ""
}

func (f *restFallbackCache) isRunning() bool {
	f.mu.RLock()
	v := f.running
	f.mu.RUnlock()
	return v
}

func (f *restFallbackCache) run(host string) {
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

func (f *restFallbackCache) stop() {
	f.cancel()
	// we make sure we have stopped
	for {
		if !f.isRunning() {
			return
		}
	}
}
