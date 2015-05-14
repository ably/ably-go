package ably

import (
	"errors"
	"testing"
)

var errNotEmitted = errors.New("not emitted")

func chans(n int) []chan error {
	ch := make([]chan error, n)
	for i := range ch {
		ch[i] = make(chan error, 1)
	}
	return ch
}

func receive(ch ...chan error) []error {
	errs := make([]error, len(ch))
	for i, ch := range ch {
		select {
		case e := <-ch:
			errs[i] = e
		default:
			errs[i] = errNotEmitted
		}
	}
	return errs
}

func emit(serial int64, count int, fn func(*pendingEmitter, int64, int, error)) func(*pendingEmitter) {
	return func(q *pendingEmitter) {
		fn(q, serial, count, nil)
	}
}

func testQueuedEmitter(t *testing.T, serials, ack, nack []int64, emit func(*pendingEmitter)) {
	ch := chans(len(serials))
	index := make(map[int64]int)
	for i, serial := range serials {
		index[serial] = i
	}
	q := &pendingEmitter{}
	for serial, i := range index {
		q.Enqueue(serial, ch[i])
	}
	emit(q)
	errs := receive(ch...)
	for _, serial := range ack {
		switch err := errs[index[serial]]; err {
		case errNotEmitted:
			t.Errorf("ack for message serial %d was not emitted", serial)
		case nil:
		default:
			t.Errorf("unexpected error for message serial %d: %v", serial, err)
		}
	}
	for _, serial := range nack {
		switch err := errs[index[serial]]; err {
		case errNotEmitted:
			t.Errorf("ack for message serial %d was not emitted", serial)
		case nil:
			t.Errorf("unexpected nil error for message serial %d", serial)
		}
	}
	if expected := len(serials) - len(ack) - len(nack); q.Len() != expected {
		t.Errorf("want q.Len()=%d; got %d (q=%v)", expected, q.Len(), q)
	}
}

func TestQueuedEmitter(t *testing.T) {
	cases := [...]struct {
		serial, ack, nack []int64
		emit              func(*pendingEmitter)
	}{{ // 5 pending messages, ack first two
		serial: []int64{1, 2, 3, 4, 5},
		ack:    []int64{1, 2},
		emit:   emit(1, 2, (*pendingEmitter).Ack),
	}, { // 5 pending messages, ack for second and third, first should got nack
		serial: []int64{1, 2, 3, 4, 5},
		ack:    []int64{2, 3},
		nack:   []int64{1},
		emit:   emit(2, 2, (*pendingEmitter).Ack),
	}, { // 5 pending messages, 1 and 2 get nack, rest ack
		serial: []int64{5, 2, 4, 1, 3},
		ack:    []int64{3, 4, 5},
		nack:   []int64{1, 2},
		emit:   emit(3, 3, (*pendingEmitter).Ack),
	}, { // 10 pending messages, first 5 get nacks, next 4 ack
		serial: []int64{9, 4, 2, 6, 10, 3, 1, 7, 5, 8},
		ack:    []int64{6, 7, 8, 9},
		nack:   []int64{1, 2, 3, 4, 5},
		emit:   emit(6, 4, (*pendingEmitter).Ack),
	}, { // 5 pending messages, nack first 3
		serial: []int64{3, 5, 1, 4, 2},
		nack:   []int64{1, 2, 3},
		emit:   emit(1, 3, (*pendingEmitter).Nack),
	}, { // 5 pending messages, nack all
		serial: []int64{2, 1, 3, 5, 4},
		nack:   []int64{1, 2, 3, 4, 5},
		emit:   emit(1, 5, (*pendingEmitter).Nack),
	}, { // 5 pending messages, nack all
		serial: []int64{4, 5, 2, 3, 1},
		nack:   []int64{1, 2, 3, 4, 5},
		emit:   emit(2, 10, (*pendingEmitter).Nack),
	}}
	for _, cas := range cases {
		testQueuedEmitter(t, cas.serial, cas.ack, cas.nack, cas.emit)
	}
}
