//go:build !integration
// +build !integration

package ably_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"

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
