//go:build !integration
// +build !integration

package ably

import (
	"errors"
	"testing"

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
