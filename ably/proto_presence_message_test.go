//go:build !integration
// +build !integration

package ably_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"
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
			if err != nil {
				ts.Fatal(err)
			}
			msg := ably.PresenceMessage{}
			err = json.Unmarshal(b, &msg)
			if err != nil {
				ts.Fatal(err)
			}
			if msg.ID != id {
				ts.Errorf("expected id to be %s got %s", id, msg.ID)
			}
			if msg.Action != a {
				ts.Errorf("expected action to be %d got %d", a, msg.Action)
			}
		})
		t.Run("msgpack", func(ts *testing.T) {
			b, err := ablyutil.MarshalMsgpack(m)
			if err != nil {
				ts.Fatal(err)
			}
			msg := ably.PresenceMessage{}
			err = ablyutil.UnmarshalMsgpack(b, &msg)
			if err != nil {
				ts.Fatal(err)
			}
			if msg.ID != id {
				ts.Errorf("expected id to be %s got %s", id, msg.ID)
			}
			if msg.Action != a {
				ts.Errorf("expected action to be %d got %d", a, msg.Action)
			}
		})
	}
}
