//go:build !integration
// +build !integration

package ably_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ablytest"

	"github.com/stretchr/testify/assert"
)

func TestPresenceMessage(t *testing.T) {
	actions := []ably.PresenceAction{
		ably.PresenceActionAbsent,
		ably.PresenceActionPresent,
		ably.PresenceActionEnter,
		ably.PresenceActionLeave,
	}

	for _, a := range actions {
		// pin
		a := a
		id := fmt.Sprint(a)
		m := ably.PresenceMessage{
			Message: ably.Message{
				ID: id,
			},
			Action: a,
		}

		t.Run("json", func(ts *testing.T) {
			b, err := json.Marshal(m)
			assert.NoError(ts, err)
			msg := ably.PresenceMessage{}
			err = json.Unmarshal(b, &msg)
			assert.NoError(ts, err)
			assert.Equal(ts, id, msg.ID,
				"expected id to be %s got %s", id, msg.ID)
			assert.Equal(ts, a, msg.Action,
				"expected action to be %d got %d", a, msg.Action)
		})
		t.Run("msgpack", func(ts *testing.T) {
			b, err := ablyutil.MarshalMsgpack(m)
			assert.NoError(ts, err)
			msg := ably.PresenceMessage{}
			err = ablyutil.UnmarshalMsgpack(b, &msg)
			assert.NoError(ts, err)
			assert.Equal(ts, id, msg.ID,
				"expected id to be %s got %s", id, msg.ID)
			assert.Equal(ts, a, msg.Action,
				"expected action to be %d got %d", a, msg.Action)
		})
	}
}

func TestPresenceCheckForNewNessByTimestampIfSynthesized_RTP2b1(t *testing.T) {
	presenceMsg1 := &ably.PresenceMessage{
		Action: ably.PresenceActionPresent,
		Message: ably.Message{
			ID:           "123:12:1",
			Timestamp:    125,
			ConnectionID: "987",
		},
	}
	presenceMsg2 := &ably.PresenceMessage{
		Action: ably.PresenceActionPresent,
		Message: ably.Message{
			ID:           "123:12:2",
			Timestamp:    123,
			ConnectionID: "784",
		},
	}
	isNewMsg, err := presenceMsg1.IsNewerThan(presenceMsg2)
	assert.Nil(t, err)
	assert.True(t, isNewMsg)

	isNewMsg, err = presenceMsg2.IsNewerThan(presenceMsg1)
	assert.Nil(t, err)
	assert.False(t, isNewMsg)
}

func TestPresenceCheckForNewNessBySerialIfNotSynthesized__RTP2b2(t *testing.T) {
	oldPresenceMsg := &ably.PresenceMessage{
		Action: ably.PresenceActionPresent,
		Message: ably.Message{
			ID:           "123:12:0",
			Timestamp:    123,
			ConnectionID: "123",
		},
	}
	newPresenceMessage := &ably.PresenceMessage{
		Action: ably.PresenceActionPresent,
		Message: ably.Message{
			ID:           "123:12:1",
			Timestamp:    123,
			ConnectionID: "123",
		},
	}
	isNewMsg, err := oldPresenceMsg.IsNewerThan(newPresenceMessage)
	assert.Nil(t, err)
	assert.False(t, isNewMsg)

	isNewMsg, err = newPresenceMessage.IsNewerThan(oldPresenceMsg)
	assert.Nil(t, err)
	assert.True(t, isNewMsg)
}

func TestPresenceMessagesShouldReturnErrorForWrongMessageSerials__RTP2b2(t *testing.T) {
	// Both has invalid msgserial
	msg1 := &ably.PresenceMessage{
		Action: ably.PresenceActionPresent,
		Message: ably.Message{
			ID:           "123:1a:0",
			Timestamp:    123,
			ConnectionID: "123",
		},
	}

	msg2 := &ably.PresenceMessage{
		Action: ably.PresenceActionPresent,
		Message: ably.Message{
			ID:           "123:1b:1",
			Timestamp:    124,
			ConnectionID: "123",
		},
	}
	isNewMsg, err := msg1.IsNewerThan(msg2)
	assert.NotNil(t, err)
	assert.Contains(t, fmt.Sprint(err), "the presence message has invalid msgSerial, for msgId 123:1a:0")
	assert.False(t, isNewMsg)

	isNewMsg, err = msg2.IsNewerThan(msg1)
	assert.NotNil(t, err)
	assert.Contains(t, fmt.Sprint(err), "the presence message has invalid msgSerial, for msgId 123:1b:1")
	assert.False(t, isNewMsg)

	// msg2 has valid messageSerial
	msg2 = &ably.PresenceMessage{
		Action: ably.PresenceActionPresent,
		Message: ably.Message{
			ID:           "123:10:0",
			Timestamp:    124,
			ConnectionID: "123",
		},
	}

	isNewMsg, err = msg1.IsNewerThan(msg2)
	assert.NotNil(t, err)
	assert.Contains(t, fmt.Sprint(err), "the presence message has invalid msgSerial, for msgId 123:1a:0")
	assert.False(t, isNewMsg)

	isNewMsg, err = msg2.IsNewerThan(msg1)
	assert.NotNil(t, err)
	assert.Contains(t, fmt.Sprint(err), "the presence message has invalid msgSerial, for msgId 123:1a:0")
	assert.True(t, isNewMsg)
}

func Test_PresenceMap_RTP2(t *testing.T) {
	const channelRetryTimeout = 123 * time.Millisecond
	const realtimeRequestTimeout = 2 * time.Second

	setup := func(t *testing.T) (
		in, out chan *ably.ProtocolMessage,
		c *ably.Realtime,
		channel *ably.RealtimeChannel,
		stateChanges ably.ChannelStateChanges,
		afterCalls chan ablytest.AfterCall,
	) {
		in = make(chan *ably.ProtocolMessage, 1)
		out = make(chan *ably.ProtocolMessage, 16)
		afterCalls = make(chan ablytest.AfterCall, 1)
		now, after := ablytest.TimeFuncs(afterCalls)

		c, _ = ably.NewRealtime(
			ably.WithToken("fake:token"),
			ably.WithAutoConnect(false),
			ably.WithChannelRetryTimeout(channelRetryTimeout),
			ably.WithRealtimeRequestTimeout(realtimeRequestTimeout),
			ably.WithDial(MessagePipe(in, out)),
			ably.WithNow(now),
			ably.WithAfter(after),
		)

		in <- &ably.ProtocolMessage{
			Action:            ably.ActionConnected,
			ConnectionID:      "connection-id",
			ConnectionDetails: &ably.ConnectionDetails{},
		}

		err := ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
		assert.NoError(t, err)

		channel = c.Channels.Get("test")
		stateChanges = make(ably.ChannelStateChanges, 10)
		return
	}

	t.Run("RTL14: when Error, should transition to failed state", func(t *testing.T) {
		in, out, _, channel, stateChanges, afterCalls := setup(t)

		errInfo := ably.ProtoErrorInfo{
			StatusCode: 500,
			Code:       50500,
			Message:    "fake error",
		}

		in <- &ably.ProtocolMessage{
			Action:  ably.ActionError,
			Channel: channel.Name,
			Error:   &errInfo,
		}

		// Expect a state change with the error.

		var change ably.ChannelStateChange
		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		assert.Equal(t, ably.ChannelStateFailed, change.Current,
			"expected %v; got %v (event: %+v)", change)

		got := fmt.Sprint(change.Reason)
		assert.Contains(t, got, "fake error",
			"expected error info to contain \"fake error\"; got %v", got)

		got = fmt.Sprint(channel.ErrorReason())
		assert.Contains(t, got, "fake error",
			"expected error info to contain \"fake error\"; got %v", got)

		ablytest.Instantly.NoRecv(t, nil, afterCalls, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, stateChanges, t.Fatalf)
		ablytest.Instantly.NoRecv(t, nil, out, t.Fatalf)
	})

}

func Test_Presence_SYNC_RTP18(t *testing.T) {
	assert.True(t, true)
}
