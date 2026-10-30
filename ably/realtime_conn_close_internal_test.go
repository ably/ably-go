//go:build !integration
// +build !integration

package ably

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMsgQueue_FailInterrupted(t *testing.T) {
	client, err := NewRealtime(WithKey("fake.key:secret"), WithAutoConnect(false))
	require.NoError(t, err)
	q := newMsgQueue(client.Connection)

	failed := map[string]error{}
	enqueue := func(name string, interruptions uint32) {
		q.Enqueue(&protocolMessage{Action: actionMessage}, &msgAckCallback{onAck: func(err error) {
			failed[name] = err
		}}, interruptions)
	}
	enqueue("before", 0)
	client.Connection.interruptions++ // e.g. Close, then Connect before the handler runs
	// Checked the connection state before the interruption, but enqueued after it.
	enqueue("raced", 0)
	enqueue("after", 1)

	failure := errors.New("closed")
	q.FailInterrupted(failure)

	assert.Equal(t, map[string]error{"before": failure, "raced": failure}, failed)
	require.Len(t, q.queue, 1, "expected the message queued after the interruption to be kept")

	// Checked the connection state before the interruption, but enqueued
	// after its handler ran.
	enqueue("late", 0)
	assert.Equal(t, failure, failed["late"])
	require.Len(t, q.queue, 1)
}

func TestMsgQueue_FlushFailsInterrupted(t *testing.T) {
	client, err := NewRealtime(WithKey("fake.key:secret"), WithAutoConnect(false))
	require.NoError(t, err)
	c := client.Connection
	q := newMsgQueue(c)

	failed := map[string]error{}
	enqueue := func(name string, interruptions uint32) {
		q.Enqueue(&protocolMessage{Action: actionMessage}, &msgAckCallback{onAck: func(err error) {
			failed[name] = err
		}}, interruptions)
	}
	enqueue("before", 0)
	q.Enqueue(&protocolMessage{Action: actionAttach}, nil, 0)
	failure := errors.New("suspended")
	c.mtx.Lock()
	c.lockFailPending(failure) // as on SUSPENDED, before FailInterrupted runs
	c.mtx.Unlock()
	enqueue("after", 1)

	q.Flush()

	assert.Equal(t, map[string]error{"before": failure}, failed)
	// The connection isn't connected, so send queues the rest on it.
	require.Len(t, c.queue.queue, 2, "expected the ATTACH and the message queued after the interruption to be sent")
	assert.Equal(t, actionAttach, c.queue.queue[0].msg.Action)
}

// Flush and Fail don't hold the queue's lock while sending or calling
// callbacks, which may queue messages on it.
func TestMsgQueue_SendsAndCallsBackWithoutLock(t *testing.T) {
	client, err := NewRealtime(WithKey("fake.key:secret"), WithAutoConnect(false))
	require.NoError(t, err)
	q := client.Connection.queue

	done := make(chan struct{})
	go func() {
		defer close(done)
		// The connection isn't connected, so send queues the message on q again.
		q.Enqueue(&protocolMessage{Action: actionAttach}, nil, 0)
		q.Flush()

		q.Enqueue(&protocolMessage{Action: actionMessage}, &msgAckCallback{onAck: func(error) {
			q.Enqueue(&protocolMessage{Action: actionAttach}, nil, 0)
		}}, 0)
		q.Fail(errors.New("failed"))
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("deadlocked on the queue's lock")
	}
	assert.Len(t, q.queue, 1)
}

func TestConnection_LockSetState_ClosingDoesNotRetry(t *testing.T) {
	for _, retrying := range []ConnectionState{ConnectionStateDisconnected, ConnectionStateSuspended} {
		t.Run("RTN12: "+retrying.String()+" while CLOSING ends CLOSED", func(t *testing.T) {
			client, err := NewRealtime(WithKey("fake.key:secret"), WithAutoConnect(false))
			require.NoError(t, err)
			c := client.Connection

			c.mtx.Lock()
			defer c.mtx.Unlock()
			c.lockSetState(ConnectionStateClosing, nil, 0)
			c.lockSetState(retrying, errors.New("transport failed"), 0)
			assert.Equal(t, ConnectionStateClosed, c.state)

			c.lockSetState(retrying, errors.New("transport failed"), 0)
			assert.Equal(t, ConnectionStateClosed, c.state)
		})
	}
}
