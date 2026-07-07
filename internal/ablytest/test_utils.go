package ablytest

import (
	"sort"
	"testing"
)

func GetReconnectionTimersFrom(t *testing.T, afterCalls <-chan AfterCall) (reconnect AfterCall, suspend AfterCall) {
	var timers []AfterCall
	for i := 0; i < 2; i++ {
		var timer AfterCall
		Instantly.Recv(t, &timer, afterCalls, t.Fatalf)
		timers = append(timers, timer)
	}
	// Shortest timer is for reconnection.
	sort.Slice(timers, func(i, j int) bool {
		return timers[i].D < timers[j].D
	})
	reconnect, suspend = timers[0], timers[1]
	return
}
