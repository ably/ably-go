package ably_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
)

func Test_RTE3_EventEmitter_On(t *testing.T) {
	for _, tc := range []struct {
		event      string
		data       interface{}
		expectRecv []string
	}{{
		"qux", 42,
		[]string{"*"},
	}, {
		"foo", 42,
		[]string{"*", "foo"},
	}, {
		"bar", 42,
		[]string{"*", "bar", "bar"},
	}} {
		t.Run(fmt.Sprintf("event: %s, data: %v", tc.event, tc.data), func(t *testing.T) {
			t.Parallel()

			em := ably.NewEventEmitter(ablytest.DiscardLogger)

			received := map[string]chan interface{}{
				"*":   make(chan interface{}, 999),
				"foo": make(chan interface{}, 999),
				"bar": make(chan interface{}, 999),
			}

			em.On(nil, func(v interface{}) {
				received["*"] <- v
			})

			em.On("foo", func(v interface{}) {
				received["foo"] <- v
			})

			em.On("bar", func(v interface{}) {
				received["bar"] <- v
			})

			em.On("bar", func(v interface{}) {
				received["bar"] <- v
			})

			em.Emit(tc.event, tc.data)

			for _, name := range tc.expectRecv {
				fatalf := ablytest.FmtFunc.Wrap(t.Fatalf, t, "received to %q: %s", name)

				var recv interface{}
				ablytest.Instantly.Recv(t, &recv, received[name], fatalf)

				if !reflect.DeepEqual(tc.data, recv) {
					fatalf("expected %q, got %q")
				}
			}

			assertNoFurtherReceives(t, received)
		})
	}
}

func Test_RTE4_EventEmitter_Once(t *testing.T) {
	for _, tc := range []struct {
		event      string
		data       interface{}
		expectRecv []string
	}{{
		"qux", 42,
		[]string{"*"},
	}, {
		"foo", 42,
		[]string{"*", "foo"},
	}, {
		"bar", 42,
		[]string{"*", "bar", "bar"},
	}} {
		t.Run(fmt.Sprintf("event: %s, data: %v", tc.event, tc.data), func(t *testing.T) {
			t.Parallel()

			em := ably.NewEventEmitter(ablytest.DiscardLogger)

			received := map[string]chan interface{}{
				"*":   make(chan interface{}, 999),
				"foo": make(chan interface{}, 999),
				"bar": make(chan interface{}, 999),
			}

			em.Once(nil, func(v interface{}) {
				received["*"] <- v
			})

			em.Once("foo", func(v interface{}) {
				received["foo"] <- v
			})

			em.Once("bar", func(v interface{}) {
				received["bar"] <- v
			})

			em.Once("bar", func(v interface{}) {
				received["bar"] <- v
			})

			em.Emit(tc.event, tc.data)

			for _, name := range tc.expectRecv {
				fatalf := ablytest.FmtFunc.Wrap(t.Fatalf, t, "received to %q: %s", name)

				var recv interface{}
				ablytest.Instantly.Recv(t, &recv, received[name], fatalf)

				if !reflect.DeepEqual(tc.data, recv) {
					fatalf("expected %q, got %q")
				}
			}

			assertNoFurtherReceives(t, received)

			for name, ch := range received {
				fatalf := ablytest.FmtFunc.Wrap(t.Fatalf, t, "received to %q: %s", name)
				ablytest.Instantly.NoRecv(t, nil, ch, fatalf)
			}

			// Emit again; check that nothing is received because the listeners
			// got removed after the first trigger.

			em.Emit(tc.event, tc.data)

			assertNoFurtherReceives(t, received)
		})
	}
}

func Test_RTE5_EventEmitter_Off(t *testing.T) {
	setUp := func() (
		received map[string]chan interface{},
		em *ably.EventEmitter,
		allOff, fooOff, barOff1, barOff2 func(),
	) {
		received = map[string]chan interface{}{
			"*":   make(chan interface{}, 999),
			"foo": make(chan interface{}, 999),
			"bar": make(chan interface{}, 999),
		}

		em = ably.NewEventEmitter(ablytest.DiscardLogger)

		allOff = em.On(nil, func(v interface{}) {
			received["*"] <- v
		})

		fooOff = em.On("foo", func(v interface{}) {
			received["foo"] <- v
		})

		barOff1 = em.On("bar", func(v interface{}) {
			received["bar"] <- v
		})

		barOff2 = em.On("bar", func(v interface{}) {
			received["bar"] <- v
		})

		return
	}

	t.Run("specific listener", func(t *testing.T) {
		t.Parallel()

		received, em, _, fooOff, barOff1, _ := setUp()
		defer assertNoFurtherReceives(t, received)

		barOff1()
		em.Emit("bar", nil)

		// The other listeners work.
		ablytest.Instantly.Recv(t, nil, received["bar"], t.Fatalf)
		ablytest.Instantly.Recv(t, nil, received["*"], t.Fatalf)

		// Other specific listeners unaffected.
		em.Emit("foo", nil)
		ablytest.Instantly.Recv(t, nil, received["foo"], t.Fatalf)
		ablytest.Instantly.Recv(t, nil, received["*"], t.Fatalf)

		fooOff()
		em.Emit("foo", nil)

		// Only matching listener affected.
		ablytest.Instantly.NoRecv(t, nil, received["foo"], t.Fatalf)
		ablytest.Instantly.Recv(t, nil, received["*"], t.Fatalf)
	})

	t.Run("specific event", func(t *testing.T) {
		t.Parallel()

		received, em, _, _, _, _ := setUp()
		defer assertNoFurtherReceives(t, received)

		em.Off("bar")
		em.Emit("bar", nil)

		// Event-specific listeners affected.
		ablytest.Instantly.NoRecv(t, nil, received["bar"], t.Fatalf)

		// The other listeners work.
		ablytest.Instantly.Recv(t, nil, received["*"], t.Fatalf)

		// Other specific listeners unaffected.
		em.Emit("foo", nil)
		ablytest.Instantly.Recv(t, nil, received["foo"], t.Fatalf)
		ablytest.Instantly.Recv(t, nil, received["*"], t.Fatalf)

		em.Off("foo")
		em.Emit("foo", nil)

		// Only matching listener affected.
		ablytest.Instantly.NoRecv(t, nil, received["foo"], t.Fatalf)
		ablytest.Instantly.Recv(t, nil, received["*"], t.Fatalf)
	})

	t.Run("all", func(t *testing.T) {
		t.Parallel()

		received, em, _, _, _, _ := setUp()

		em.Off(nil)

		em.Emit("foo", nil)
		em.Emit("bar", nil)
		em.Emit("qux", nil)

		// Nothing gets delivered.
		assertNoFurtherReceives(t, received)
	})
}

func Test_RTE6_EventEmitter_EmitPanic(t *testing.T) {
	t.Parallel()

	logs := make(chan ablytest.LogMessage, 999)
	em := ably.NewEventEmitter(ablytest.NewLogger(logs))

	shouldPanic := true
	called := make(chan struct{}, 999)
	em.On(nil, func(interface{}) {
		if shouldPanic {
			panic("aaah")
		}
		called <- struct{}{}
	})

	em.Emit(nil, nil)

	ablytest.Instantly.Recv(t, nil, logs, t.Fatalf)
	ablytest.Instantly.NoRecv(t, nil, called, t.Fatalf)

	shouldPanic = false

	// Panic didn't break anything.

	em.Emit(nil, nil)

	ablytest.Instantly.NoRecv(t, nil, logs, t.Fatalf)
	ablytest.Instantly.Recv(t, nil, called, t.Fatalf)
}

func Test_RTE6a_EventEmitter_EmitToFixedListenersCollection(t *testing.T) {
	t.Parallel()

	// Give it a few tries; the order in which listeners are triggered isn't
	// deterministic.
	for i := 0; i < 10; i++ {
		em := ably.NewEventEmitter(ablytest.DiscardLogger)

		addedCalled := make(chan struct{}, 1)
		removedCalled := make(chan struct{}, 1)

		var off func()

		// Shuffle the order in which we register the listeners for extra
		// exhaustiveness.
		ons := []func(){func() {
			off = em.On(nil, func(interface{}) {
				close(removedCalled)
			})
		}, func() {
			em.On(nil, func(interface{}) {
				off()

				em.On(nil, func(interface{}) {
					close(addedCalled)
				})
			})
		}}
		rand.Shuffle(len(ons), func(i, j int) {
			ons[i], ons[j] = ons[j], ons[i]
		})
		for _, on := range ons {
			on()
		}

		em.Emit(nil, nil)

		ablytest.Instantly.NoRecv(t, nil, addedCalled, t.Errorf)
		ablytest.Instantly.Recv(t, nil, removedCalled, t.Errorf)
	}
}

func assertNoFurtherReceives(t *testing.T, received map[string]chan interface{}) {
	t.Helper()
	for name, ch := range received {
		fatalf := ablytest.FmtFunc.Wrap(t.Fatalf, t, "received to %q: %s", name)
		ablytest.Instantly.NoRecv(t, nil, ch, fatalf)
	}
}
