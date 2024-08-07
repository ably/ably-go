package ably

import (
	"sync"

	"github.com/ably/ably-go/ably/internal/ablyutil"
)

// realtimeHosts(RTN17) is used to retrieve realtime primaryHost/fallbackHosts
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
	return realtimeHosts.opts.getRealtimeHost()
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

// getPreferredHost - Used to retrieve primary realtime host
func (realtimeHosts *realtimeHosts) getPreferredHost() string {
	realtimeHosts.Lock()
	defer realtimeHosts.Unlock()
	host := realtimeHosts.getPrimaryHost() // primary host is always preferred host/ fallback host in realtime
	realtimeHosts.visitedHosts.Add(host)
	return host
}
