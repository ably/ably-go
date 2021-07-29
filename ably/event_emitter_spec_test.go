package ably_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"
)

func Test_RTE3_EventEmitter_On(t *testing.T) {
	for _, tc := range []struct {
		event      ably.EmitterString
		data       ably.EmitterData
		expectRecv []ably.EmitterString
	}{{
		"qux", ably.EmitterString("42"),
		[]ably.EmitterString{"*"},
	}, {
		"foo", ably.EmitterString("42"),
		[]ably.EmitterString{"*", "foo"},
	}, {
		"bar", ably.EmitterString("42"),
		[]ably.EmitterString{"*", "bar", "bar"},
	}} {
		t.Run(fmt.Sprintf("event: %s, data: %v", tc.event, tc.data), func(t *testing.T) {
			t.Parallel()

			em := ably.NewEventEmitter(ably.NewInternalLogger(ablytest.DiscardLogger))

			received := map[ably.EmitterString]chan ably.EmitterData{
				"*":   make(chan ably.EmitterData, 999),
				"foo": make(chan ably.EmitterData, 999),
				"bar": make(chan ably.EmitterData, 999),
			}

			em.OnAll(func(v ably.EmitterData) {
				received["*"] <- v
			})

			em.On(ably.EmitterString("foo"), func(v ably.EmitterData) {
				received["foo"] <- v
			})

			em.On(ably.EmitterString("bar"), func(v ably.EmitterData) {
				received["bar"] <- v
			})

			em.On(ably.EmitterString("bar"), func(v ably.EmitterData) {
				received["bar"] <- v
			})

			em.Emit(tc.event, tc.data)

			for _, name := range tc.expectRecv {
				fatalf := ablytest.FmtFunc.Wrap(t.Fatalf, t, "received to %q: %s", name)

				var recv ably.EmitterData
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
		event      ably.EmitterString
		data       ably.EmitterData
		expectRecv []ably.EmitterString
	}{{
		"qux", ably.EmitterString("42"),
		[]ably.EmitterString{"*"},
	}, {
		"foo", ably.EmitterString("42"),
		[]ably.EmitterString{"*", "foo"},
	}, {
		"bar", ably.EmitterString("42"),
		[]ably.EmitterString{"*", "bar", "bar"},
	}} {
		t.Run(fmt.Sprintf("event: %s, data: %v", tc.event, tc.data), func(t *testing.T) {
			t.Parallel()

			em := ably.NewEventEmitter(ably.NewInternalLogger(ablytest.DiscardLogger))

			received := map[ably.EmitterString]chan ably.EmitterData{
				"*":   make(chan ably.EmitterData, 999),
				"foo": make(chan ably.EmitterData, 999),
				"bar": make(chan ably.EmitterData, 999),
			}

			em.Once(nil, func(v ably.EmitterData) {
				received["*"] <- v
			})

			em.Once(ably.EmitterString("foo"), func(v ably.EmitterData) {
				received["foo"] <- v
			})

			em.Once(ably.EmitterString("bar"), func(v ably.EmitterData) {
				received["bar"] <- v
			})

			em.Once(ably.EmitterString("bar"), func(v ably.EmitterData) {
				received["bar"] <- v
			})

			em.Emit(tc.event, tc.data)

			for _, name := range tc.expectRecv {
				fatalf := ablytest.FmtFunc.Wrap(t.Fatalf, t, "received to %q: %s", name)

				var recv ably.EmitterData
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
		received map[ably.EmitterString]chan ably.EmitterData,
		em *ably.EventEmitter,
		allOff, fooOff, barOff1, barOff2 func(),
	) {
		received = map[ably.EmitterString]chan ably.EmitterData{
			"*":   make(chan ably.EmitterData, 999),
			"foo": make(chan ably.EmitterData, 999),
			"bar": make(chan ably.EmitterData, 999),
		}

		em = ably.NewEventEmitter(ably.NewInternalLogger(ablytest.DiscardLogger))

		allOff = em.OnAll(func(v ably.EmitterData) {
			received["*"] <- v
		})

		fooOff = em.On(ably.EmitterString("foo"), func(v ably.EmitterData) {
			received["foo"] <- v
		})

		barOff1 = em.On(ably.EmitterString("bar"), func(v ably.EmitterData) {
			received["bar"] <- v
		})

		barOff2 = em.On(ably.EmitterString("bar"), func(v ably.EmitterData) {
			received["bar"] <- v
		})

		return
	}

	t.Run("specific listener", func(t *testing.T) {
		t.Parallel()

		received, em, _, fooOff, barOff1, _ := setUp()
		defer assertNoFurtherReceives(t, received)

		barOff1()
		em.Emit(ably.EmitterString("bar"), nil)

		// The other listeners work.
		ablytest.Instantly.Recv(t, nil, received["bar"], t.Fatalf)
		ablytest.Instantly.Recv(t, nil, received["*"], t.Fatalf)

		// Other specific listeners unaffected.
		em.Emit(ably.EmitterString("foo"), nil)
		ablytest.Instantly.Recv(t, nil, received["foo"], t.Fatalf)
		ablytest.Instantly.Recv(t, nil, received["*"], t.Fatalf)

		fooOff()
		em.Emit(ably.EmitterString("foo"), nil)

		// Only matching listener affected.
		ablytest.Instantly.NoRecv(t, nil, received["foo"], t.Fatalf)
		ablytest.Instantly.Recv(t, nil, received["*"], t.Fatalf)
	})

	t.Run("specific event", func(t *testing.T) {
		t.Parallel()

		received, em, _, _, _, _ := setUp()
		defer assertNoFurtherReceives(t, received)

		em.Off(ably.EmitterString("bar"))
		em.Emit(ably.EmitterString("bar"), nil)

		// Event-specific listeners affected.
		ablytest.Instantly.NoRecv(t, nil, received["bar"], t.Fatalf)

		// The other listeners work.
		ablytest.Instantly.Recv(t, nil, received["*"], t.Fatalf)

		// Other specific listeners unaffected.
		em.Emit(ably.EmitterString("foo"), nil)
		ablytest.Instantly.Recv(t, nil, received["foo"], t.Fatalf)
		ablytest.Instantly.Recv(t, nil, received["*"], t.Fatalf)

		em.Off(ably.EmitterString("foo"))
		em.Emit(ably.EmitterString("foo"), nil)

		// Only matching listener affected.
		ablytest.Instantly.NoRecv(t, nil, received["foo"], t.Fatalf)
		ablytest.Instantly.Recv(t, nil, received["*"], t.Fatalf)
	})

	t.Run("all", func(t *testing.T) {
		t.Parallel()

		received, em, _, _, _, _ := setUp()

		em.Off(nil)

		em.Emit(ably.EmitterString("foo"), nil)
		em.Emit(ably.EmitterString("bar"), nil)
		em.Emit(ably.EmitterString("qux"), nil)

		// Nothing gets delivered.
		assertNoFurtherReceives(t, received)
	})
}

func Test_RTE6_EventEmitter_EmitPanic(t *testing.T) {
	t.Parallel()

	logs := make(chan ablytest.LogMessage, 999)
	em := ably.NewEventEmitter(ably.NewInternalLogger(ablytest.NewLogger(logs)))

	shouldPanic := true
	called := make(chan struct{}, 999)
	em.OnAll(func(ably.EmitterData) {
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
		em := ably.NewEventEmitter(ably.NewInternalLogger(ablytest.DiscardLogger))

		addedCalled := make(chan struct{}, 1)
		removedCalled := make(chan struct{}, 1)

		var off func()

		// Shuffle the order in which we register the listeners for extra
		// exhaustiveness.
		ons := []func(){func() {
			off = em.OnAll(func(ably.EmitterData) {
				close(removedCalled)
			})
		}, func() {
			em.OnAll(func(ably.EmitterData) {
				off()

				em.OnAll(func(ably.EmitterData) {
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

func assertNoFurtherReceives(t *testing.T, received map[ably.EmitterString]chan ably.EmitterData) {
	t.Helper()
	for name, ch := range received {
		fatalf := ablytest.FmtFunc.Wrap(t.Fatalf, t, "received to %q: %s", name)
		ablytest.Instantly.NoRecv(t, nil, ch, fatalf)
	}
}
