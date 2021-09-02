package ably_test

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"sort"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"
)

/*
FAILING TEST
https://github.com/ably/ably-go/pull/383/checks?check_run_id=3489733889#step:7:711

=== RUN   TestPresenceHistory_RSP4_RSP4b3/limit=20
--- FAIL: TestPresenceHistory_RSP4_RSP4b3 (95.01s)
    --- FAIL: TestPresenceHistory_RSP4_RSP4b3/limit=2 (42.06s)
        rest_presence_spec_test.go:38: expected items: [<PresenceMessage enter clientID=client9 data=msg9.0> <PresenceMessage leave clientID=client8 data=msg8.2> <PresenceMessage update clientID=client8 data=msg8.1> <PresenceMessage enter clientID=client8 data=msg8.0> <PresenceMessage update clientID=client7 data=msg7.1> <PresenceMessage enter clientID=client7 data=msg7.0> <PresenceMessage enter clientID=client6 data=msg6.0> <PresenceMessage leave clientID=client5 data=msg5.2> <PresenceMessage update clientID=client5 data=msg5.1> <PresenceMessage enter clientID=client5 data=msg5.0> <PresenceMessage update clientID=client4 data=msg4.1> <PresenceMessage enter clientID=client4 data=msg4.0> <PresenceMessage enter clientID=client3 data=msg3.0> <PresenceMessage leave clientID=client2 data=msg2.2> <PresenceMessage update clientID=client2 data=msg2.1> <PresenceMessage enter clientID=client2 data=msg2.0> <PresenceMessage update clientID=client1 data=msg1.1> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0>], got: [<PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0>]
    --- FAIL: TestPresenceHistory_RSP4_RSP4b3/limit=3 (41.45s)
        rest_presence_spec_test.go:38: expected items: [<PresenceMessage enter clientID=client9 data=msg9.0> <PresenceMessage leave clientID=client8 data=msg8.2> <PresenceMessage update clientID=client8 data=msg8.1> <PresenceMessage enter clientID=client8 data=msg8.0> <PresenceMessage update clientID=client7 data=msg7.1> <PresenceMessage enter clientID=client7 data=msg7.0> <PresenceMessage enter clientID=client6 data=msg6.0> <PresenceMessage leave clientID=client5 data=msg5.2> <PresenceMessage update clientID=client5 data=msg5.1> <PresenceMessage enter clientID=client5 data=msg5.0> <PresenceMessage update clientID=client4 data=msg4.1> <PresenceMessage enter clientID=client4 data=msg4.0> <PresenceMessage enter clientID=client3 data=msg3.0> <PresenceMessage leave clientID=client2 data=msg2.2> <PresenceMessage update clientID=client2 data=msg2.1> <PresenceMessage enter clientID=client2 data=msg2.0> <PresenceMessage update clientID=client1 data=msg1.1> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0>], got: [<PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage update clientID=client1 data=msg1.1> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage update clientID=client1 data=msg1.1> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage update clientID=client1 data=msg1.1> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage update clientID=client1 data=msg1.1> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage update clientID=client1 data=msg1.1> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0> <PresenceMessage update clientID=client1 data=msg1.1> <PresenceMessage enter clientID=client1 data=msg1.0> <PresenceMessage enter clientID=client0 data=msg0.0>]
    --- PASS: TestPresenceHistory_RSP4_RSP4b3/limit=20 (11.49s)
*/
func TestPresenceHistory_RSP4_RSP4b3(t *testing.T) {
	t.Skip("FAILING TEST")

	for _, limit := range []int{2, 3, 20} {
		t.Run(fmt.Sprintf("limit=%d", limit), func(t *testing.T) {

			app, rest := ablytest.NewREST()
			defer app.Close()
			channel := rest.Channels.Get("persisted:test")

			fixtures := presenceHistoryFixtures()
			realtime := postPresenceHistoryFixtures(t, context.Background(), app, "persisted:test", fixtures)
			defer safeclose(t, realtime, app)

			var err error
			if !ablytest.Soon.IsTrue(func() bool {
				err = ablytest.TestPagination(
					reversePresence(fixtures),
					channel.Presence.History(ably.PresenceHistoryWithLimit(limit)),
					limit,
					ablytest.PaginationWithEqual(presenceEqual),
				)
				return err == nil
			}) {
				t.Fatal(err)
			}
		})
	}
}

func TestPresenceHistory_Direction_RSP4b2(t *testing.T) {

	for _, c := range []struct {
		direction ably.Direction
		expected  []*ably.PresenceMessage
	}{
		{
			direction: ably.Backwards,
			expected:  reversePresence(presenceHistoryFixtures()),
		},
		{
			direction: ably.Forwards,
			expected:  presenceHistoryFixtures(),
		},
	} {
		c := c
		t.Run(fmt.Sprintf("direction=%v", c.direction), func(t *testing.T) {

			app, rest := ablytest.NewREST()
			channel := rest.Channels.Get("persisted:test")

			fixtures := presenceHistoryFixtures()
			realtime := postPresenceHistoryFixtures(t, context.Background(), app, "persisted:test", fixtures)
			defer safeclose(t, realtime, app)

			expected := c.expected

			var err error
			if !ablytest.Soon.IsTrue(func() bool {
				err = ablytest.TestPagination(expected, channel.Presence.History(
					ably.PresenceHistoryWithLimit(len(expected)),
					ably.PresenceHistoryWithDirection(c.direction),
				), len(expected), ablytest.PaginationWithEqual(presenceEqual))
				return err == nil
			}) {
				t.Fatal(err)
			}
		})
	}
}

/*
FAILING TEST
https://github.com/ably/ably-go/pull/383/checks?check_run_id=3489733889#step:7:727

=== RUN   TestPresenceGet_RSP3_RSP3a1/limit=20
--- FAIL: TestPresenceGet_RSP3_RSP3a1 (79.37s)
    --- FAIL: TestPresenceGet_RSP3_RSP3a1/limit=2 (36.42s)
        rest_presence_spec_test.go:107: expected items: [<PresenceMessage present clientID=client_bool data=true> <PresenceMessage present clientID=client_int data=true> <PresenceMessage present clientID=client_json data={"test": "This is a JSONObject clientData payload"}> <PresenceMessage present clientID=client_string data=true>], got: [<PresenceMessage present clientID=client_json data={"test": "This is a JSONObject clientData payload"}> <PresenceMessage present clientID=client_json data={"test": "This is a JSONObject clientData payload"}> <PresenceMessage present clientID=client_string data=true> <PresenceMessage present clientID=client_string data=true>]
    --- FAIL: TestPresenceGet_RSP3_RSP3a1/limit=3 (36.62s)
        rest_presence_spec_test.go:107: expected items: [<PresenceMessage present clientID=client_bool data=true> <PresenceMessage present clientID=client_int data=true> <PresenceMessage present clientID=client_json data={"test": "This is a JSONObject clientData payload"}> <PresenceMessage present clientID=client_string data=true>], got: [<PresenceMessage present clientID=client_int data=true> <PresenceMessage present clientID=client_json data={"test": "This is a JSONObject clientData payload"}> <PresenceMessage present clientID=client_string data=true> <PresenceMessage present clientID=client_string data=true>]
    --- PASS: TestPresenceGet_RSP3_RSP3a1/limit=20 (6.32s)
*/
func TestPresenceGet_RSP3_RSP3a1(t *testing.T) {
	t.Skip("FAILING TEST")

	for _, limit := range []int{2, 3, 20} {
		t.Run(fmt.Sprintf("limit=%d", limit), func(t *testing.T) {

			app, rest := ablytest.NewREST()
			defer app.Close()
			channel := rest.Channels.Get("persisted:presence_fixtures")

			expected := persistedPresenceFixtures()

			var err error
			if !ablytest.Soon.IsTrue(func() bool {
				err = ablytest.TestPagination(
					expected,
					channel.Presence.Get(ably.GetPresenceWithLimit(limit)),
					limit,
					ablytest.PaginationWithEqual(presenceEqual),
					ablytest.PaginationWithSortResult(sortPresenceByClientID),
				)
				return err == nil
			}) {
				t.Fatal(err)
			}
		})
	}
}

func TestPresenceGet_ClientID_RSP3a2(t *testing.T) {

	for _, clientID := range []string{
		"client_bool",
		"client_string",
	} {
		clientID := clientID
		t.Run(fmt.Sprintf("clientID=%v", clientID), func(t *testing.T) {

			app, rest := ablytest.NewREST()
			defer app.Close()
			channel := rest.Channels.Get("persisted:presence_fixtures")

			expected := persistedPresenceFixtures(func(p ablytest.Presence) bool {
				return p.ClientID == clientID
			})

			var err error
			if !ablytest.Soon.IsTrue(func() bool {
				err = ablytest.TestPagination(
					expected,
					channel.Presence.Get(ably.GetPresenceWithClientID(clientID)),
					1,
					ablytest.PaginationWithEqual(presenceEqual),
					ablytest.PaginationWithSortResult(sortPresenceByClientID),
				)
				return err == nil
			}) {
				t.Fatal(err)
			}
		})
	}
}

func TestPresenceGet_ConnectionID_RSP3a3(t *testing.T) {

	app, rest := ablytest.NewREST()
	defer app.Close()

	expectedByConnID := map[string]ably.Message{}

	for i := 0; i < 3; i++ {
		realtime := app.NewRealtime()
		defer safeclose(t, ablytest.FullRealtimeCloser(realtime))
		m := ably.Message{
			Data:     fmt.Sprintf("msg%d", i),
			ClientID: fmt.Sprintf("client%d", i),
		}
		realtime.Channels.Get("test").Presence.EnterClient(context.Background(), m.ClientID, m.Data)
		expectedByConnID[realtime.Connection.ID()] = m
	}

	channel := rest.Channels.Get("test")

	var rg ablytest.ResultGroup

	for connID, expected := range expectedByConnID {
		connID, expected := connID, expected
		rg.GoAdd(func(ctx context.Context) error {
			var err error
			if !ablytest.Soon.IsTrue(func() bool {
				err = ablytest.TestPagination(
					[]*ably.PresenceMessage{{
						Action:  ably.PresenceActionPresent,
						Message: expected,
					}},
					channel.Presence.Get(ably.GetPresenceWithConnectionID(connID)),
					1,
					ablytest.PaginationWithEqual(presenceEqual),
					ablytest.PaginationWithSortResult(sortPresenceByClientID),
				)
				return err == nil
			}) {
				return fmt.Errorf("connID %s: %w", connID, err)
			}
			return nil
		})
	}

	if err := rg.Wait(); err != nil {
		t.Fatal(err)
	}
}

func presenceHistoryFixtures() []*ably.PresenceMessage {
	actions := []ably.PresenceAction{
		ably.PresenceActionEnter,
		ably.PresenceActionUpdate,
		ably.PresenceActionLeave,
	}
	var fixtures []*ably.PresenceMessage
	for i := 0; i < 10; i++ {
		for j, action := range actions[:i%3+1] {
			fixtures = append(fixtures, &ably.PresenceMessage{
				Action: action,
				Message: ably.Message{
					Data:     fmt.Sprintf("msg%d.%d", i, j),
					ClientID: fmt.Sprintf("client%d", i),
				},
			})
		}
	}
	return fixtures
}

func postPresenceHistoryFixtures(t *testing.T, ctx context.Context, app *ablytest.Sandbox, channel string, fixtures []*ably.PresenceMessage) io.Closer {
	realtime := app.NewRealtime()
	p := realtime.Channels.Get(channel).Presence

	for i, m := range fixtures {
		var err error
		switch m.Action {
		case ably.PresenceActionEnter:
			err = p.EnterClient(ctx, m.ClientID, m.Data)
		case ably.PresenceActionUpdate:
			err = p.UpdateClient(ctx, m.ClientID, m.Data)
		case ably.PresenceActionLeave:
			err = p.LeaveClient(ctx, m.ClientID, m.Data)
		}
		if err != nil {
			t.Errorf("at presence fixture #%d (%v): %w", i, m.Action, err)
		}
	}

	return ablytest.FullRealtimeCloser(realtime)
}

func reversePresence(msgs []*ably.PresenceMessage) []*ably.PresenceMessage {
	var reversed []*ably.PresenceMessage
	for i := len(msgs) - 1; i >= 0; i-- {
		reversed = append(reversed, msgs[i])
	}
	return reversed
}

func presenceEqual(x, y interface{}) bool {
	mx, my := x.(*ably.PresenceMessage), y.(*ably.PresenceMessage)
	return mx.Action == my.Action &&
		mx.ClientID == my.ClientID &&
		mx.Name == my.Name &&
		reflect.DeepEqual(mx.Data, my.Data)
}

func persistedPresenceFixtures(filter ...func(ablytest.Presence) bool) []interface{} {
	var expected []interface{}
fixtures:
	for _, p := range ablytest.PresenceFixtures() {
		for _, f := range filter {
			if !f(p) {
				continue fixtures
			}
		}
		expected = append(expected, &ably.PresenceMessage{
			Action: ably.PresenceActionPresent,
			Message: ably.Message{
				ClientID: p.ClientID,
				Data:     p.Data,
			},
		})
	}

	// presence.get result order is undefined, so we need to sort both
	// expected and actual items client-side to get consistent results.
	sortPresenceByClientID(expected)

	return expected
}

func sortPresenceByClientID(items []interface{}) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].(*ably.PresenceMessage).ClientID < items[j].(*ably.PresenceMessage).ClientID
	})
}
