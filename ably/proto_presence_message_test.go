//go:build !integration
// +build !integration

package ably_test

import (
	"context"
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
		presenceMsgCh chan *ably.PresenceMessage,
	) {
		in = make(chan *ably.ProtocolMessage, 1)
		out = make(chan *ably.ProtocolMessage, 16)
		presenceMsgCh = make(chan *ably.PresenceMessage, 16)

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
		channel.OnAll(stateChanges.Receive)

		in <- &ably.ProtocolMessage{
			Action:  ably.ActionAttached,
			Channel: channel.Name,
		}

		var change ably.ChannelStateChange

		ablytest.Instantly.Recv(t, &change, stateChanges, t.Fatalf)
		assert.Equal(t, ably.ChannelStateAttached, change.Current,
			"expected %v; got %v (event: %+v)", ably.ChannelStateAttached, change.Current)

		channel.Presence.SubscribeAll(context.Background(), func(message *ably.PresenceMessage) {
			presenceMsgCh <- message
		})
		return
	}

	t.Run("RTP2: should maintain a list of members present on the channel", func(t *testing.T) {
		in, _, _, channel, _, _, presenceMsgCh := setup(t)

		initialMembers := channel.Presence.GetMembers()
		assert.Empty(t, initialMembers)

		presenceMsg1 := &ably.PresenceMessage{
			Action: ably.PresenceActionPresent,
			Message: ably.Message{
				ID:           "123:12:1",
				Timestamp:    125,
				ConnectionID: "987",
				ClientID:     "999",
			},
		}

		presenceMsg2 := &ably.PresenceMessage{
			Action: ably.PresenceActionPresent,
			Message: ably.Message{
				ID:           "123:12:2",
				Timestamp:    123,
				ConnectionID: "784",
				ClientID:     "999",
			},
		}

		msg := &ably.ProtocolMessage{
			Action:   ably.ActionPresence,
			Channel:  channel.Name,
			Presence: []*ably.PresenceMessage{presenceMsg1},
		}

		in <- msg
		ablytest.Instantly.Recv(t, nil, presenceMsgCh, t.Fatalf)
		presenceMembers := channel.Presence.GetMembers()

		assert.Equal(t, 1, len(presenceMembers))
		member := presenceMembers[presenceMsg1.ConnectionID+presenceMsg1.ClientID]
		assert.Equal(t, ably.PresenceActionPresent, member.Action)

		msg.Presence = []*ably.PresenceMessage{presenceMsg2}
		in <- msg

		ablytest.Instantly.Recv(t, nil, presenceMsgCh, t.Fatalf)
		assert.Equal(t, 2, len(channel.Presence.GetMembers()))
		member2 := presenceMembers[presenceMsg2.ConnectionID+presenceMsg2.ClientID]
		assert.Equal(t, ably.PresenceActionPresent, member2.Action)
	})

	t.Run("RTP2b1: check for newness by timestamp is synthesized", func(t *testing.T) {
		in, _, _, channel, _, _, presenceMsgCh := setup(t)

		initialMembers := channel.Presence.GetMembers()
		assert.Empty(t, initialMembers)

		presenceMsg1 := &ably.PresenceMessage{
			Action: ably.PresenceActionPresent,
			Message: ably.Message{
				ID:           "987:12:1",
				Timestamp:    125,
				ConnectionID: "987",
				ClientID:     "999",
			},
		}

		presenceMsg2 := &ably.PresenceMessage{
			Action: ably.PresenceActionPresent,
			Message: ably.Message{
				ID:           "989:12:2",
				Timestamp:    128,
				ConnectionID: "987",
				ClientID:     "999",
			},
		}

		msg := &ably.ProtocolMessage{
			Action:   ably.ActionPresence,
			Channel:  channel.Name,
			Presence: []*ably.PresenceMessage{presenceMsg1},
		}

		in <- msg
		ablytest.Soon.Recv(t, nil, presenceMsgCh, t.Fatalf)
		presenceMembers := channel.Presence.GetMembers()

		assert.Equal(t, 1, len(presenceMembers))
		member := presenceMembers[presenceMsg1.ConnectionID+presenceMsg1.ClientID]
		assert.Equal(t, ably.PresenceActionPresent, member.Action)
		assert.Equal(t, "987:12:1", member.ID)

		msg.Presence = []*ably.PresenceMessage{presenceMsg2}
		in <- msg

		ablytest.Soon.Recv(t, nil, presenceMsgCh, t.Fatalf)
		presenceMembers = channel.Presence.GetMembers()
		assert.Equal(t, 1, len(presenceMembers))
		member = presenceMembers[presenceMsg1.ConnectionID+presenceMsg1.ClientID]
		assert.Equal(t, ably.PresenceActionPresent, member.Action)
		assert.Equal(t, "989:12:2", member.ID)
	})

	t.Run("RTP2b2: check for newness by timestamp is not synthesized", func(t *testing.T) {

	})

	t.Run("RTP2c: check for newness during sync", func(t *testing.T) {

	})

	t.Run("RTP2d: when presence msg with ENTER, UPDATE AND PRESENT arrives, add to presence map with action as present", func(t *testing.T) {

	})

	t.Run("RTP2e: when presence msg with LEAVE action arrives, remove member from presence map", func(t *testing.T) {

	})

	t.Run("RTP2f: when presence msg with LEAVE action arrives, if sync in progress, store as absent and remove it later", func(t *testing.T) {

	})

	t.Run("RTP2g: incoming event should be emitted on realtimepresence object", func(t *testing.T) {

	})

}

func Test_Presence_server_initiated_sync_RTP18(t *testing.T) {
	t.Run("RTP18a: client determines a new sync started with <sync sequence id>:<cursor value>", func(t *testing.T) {

	})

	t.Run("RTP18b: client determines sync ended with <sync sequence id>:", func(t *testing.T) {

	})

	t.Run("RTP18: client determines sync started and ended with <sync sequence id>:", func(t *testing.T) {

	})
}

func Test_internal_presencemap_RTP17(t *testing.T) {
	t.Run("RTP17: presence object should have second presencemap containing only currentConnectionId", func(t *testing.T) {

	})

	t.Run("RTP17b: apply presence message events as per spec", func(t *testing.T) {

	})

	t.Run("RTP17h: presencemap should be keyed by clientId", func(t *testing.T) {

	})

	t.Run("RTP17f, RTP17g: automatic re-entry whenever channel moves into ATTACHED state", func(t *testing.T) {

	})

	t.Run("RTP17e: publish error if automatic re-enter failed", func(t *testing.T) {

	})
}
