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
	conn := client.Connection
	q := newMsgQueue(conn)

	var interruptedErr, reopenedErr error
	interruptedCalled, reopenedCalled := false, false
	q.Enqueue(&protocolMessage{Action: actionMessage}, &msgAckCallback{onAck: func(err error) {
		interruptedCalled, interruptedErr = true, err
	}})
	conn.interruptions++ // e.g. Close, then Connect before the handler runs
	q.Enqueue(&protocolMessage{Action: actionMessage}, &msgAckCallback{onAck: func(err error) {
		reopenedCalled, reopenedErr = true, err
	}})

	failure := errors.New("closed")
	q.FailInterrupted(failure)

	assert.True(t, interruptedCalled, "expected message queued before the interruption to fail")
	assert.Equal(t, failure, interruptedErr)
	assert.False(t, reopenedCalled, "expected message queued after the interruption to be kept")
	assert.NoError(t, reopenedErr)
	require.Len(t, q.queue, 1)
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
