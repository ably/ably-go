package proto_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ably/proto"
)

func TestPresenceMessage(t *testing.T) {
	actions := []proto.PresenceState{
		proto.PresenceAbsent,
		proto.PresencePresent,
		proto.PresenceEnter,
		proto.PresenceLeave,
	}

	for _, a := range actions {
		id := fmt.Sprint(a)
		m := proto.PresenceMessage{
			Message: proto.Message{
				ID: id,
			},
			State: a,
		}

		t.Run("json", func(ts *testing.T) {
			b, err := json.Marshal(m)
			if err != nil {
				ts.Fatal(err)
			}
			msg := proto.PresenceMessage{}
			err = json.Unmarshal(b, &msg)
			if err != nil {
				ts.Fatal(err)
			}
			if msg.ID != id {
				ts.Errorf("expected id to be %s got %s", id, msg.ID)
			}
			if msg.State != a {
				ts.Errorf("expected action to be %d got %d", a, msg.State)
			}
		})
		t.Run("msgpack", func(ts *testing.T) {
			b, err := ablyutil.Marshal(m)
			if err != nil {
				ts.Fatal(err)
			}
			msg := proto.PresenceMessage{}
			err = ablyutil.Unmarshal(b, &msg)
			if err != nil {
				ts.Fatal(err)
			}
			if msg.ID != id {
				ts.Errorf("expected id to be %s got %s", id, msg.ID)
			}
			if msg.State != a {
				ts.Errorf("expected action to be %d got %d", a, msg.State)
			}
		})
	}
}
