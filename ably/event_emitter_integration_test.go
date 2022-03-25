//go:build !unit

package ably_test

import (
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"

	"github.com/stretchr/testify/assert"
)

func TestEventEmitterConcurrency(t *testing.T) {
	em := ably.NewEventEmitter(ably.NewInternalLogger(ablytest.DiscardLogger))

	type called struct {
		i    int
		goOn chan struct{}
	}
	calls := make(chan called)

	for i := 0; i < 2; i++ {
		i := i
		em.OnAll(func(ably.EmitterData) {
			c := called{i: i, goOn: make(chan struct{})}
			calls <- c
			<-c.goOn
		})
	}

	// Emit, and, since handlers are concurrent, expect a call per handler to
	// be initiated.

	em.Emit(ably.EmitterString("foo"), nil)

	var ongoingCalls []called

	for i := 0; i < 2; i++ {
		var call called
		ablytest.Instantly.Recv(t, &call, calls, t.Fatalf)
		ongoingCalls = append(ongoingCalls, call)
	}
	ablytest.Instantly.NoRecv(t, nil, calls, t.Fatalf)

	// While the last event is still being handled by each handler, a new event
	// should be enqueued.

	em.Emit(ably.EmitterString("foo"), nil)

	ablytest.Instantly.NoRecv(t, nil, calls, t.Fatalf)

	// Allow the first ongoing call to finish, which should then process the
	// enqueued event for that handler.

	close(ongoingCalls[0].goOn)
	var call called
	ablytest.Instantly.Recv(t, &call, calls, t.Fatalf)
	assert.Equal(t, ongoingCalls[0].i, call.i,
		"expected to unblock handler %d, got %d", ongoingCalls[0].i, call.i)

	close(call.goOn)

	// Unblock the other handler too.
	close(ongoingCalls[1].goOn)
	ablytest.Instantly.Recv(t, &call, calls, t.Fatalf)
	assert.Equal(t, ongoingCalls[1].i, call.i,
		"expected to unblock handler %d, got %d", ongoingCalls[1].i, call.i)
	close(call.goOn)

	// Make sure things still work after emptying the queues.

	em.Emit(ably.EmitterString("foo"), nil)
	for i := 0; i < 2; i++ {
		ablytest.Instantly.Recv(t, &call, calls, t.Fatalf)
		close(call.goOn)
	}

	ablytest.Instantly.NoRecv(t, nil, calls, t.Fatalf)
}
