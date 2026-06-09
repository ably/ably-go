//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"

	"github.com/stretchr/testify/assert"
)

func TestPresenceHistory_RSP4_RSP4b3(t *testing.T) {
	for _, limit := range []int{2, 3, 20} {
		t.Run(fmt.Sprintf("limit=%d", limit), func(t *testing.T) {

			app, rest := ablytest.NewREST()
			defer app.Close()
			channelName := ablytest.ChannelName("persisted:test")
			channel := rest.Channels.Get(channelName)

			fixtures := presenceHistoryFixtures()
			realtime := postPresenceHistoryFixtures(t, context.Background(), app, channelName, fixtures)
			defer safeclose(t, realtime, app)

			var err error
			testFunc := func() bool {
				err = ablytest.TestPagination(
					fixtures,
					channel.Presence.History(ably.PresenceHistoryWithLimit(limit)),
					limit,
					ablytest.PaginationWithEqual(presenceEqual),
					ablytest.PaginationWithSortResult(sortPresenceByData),
				)
				return err == nil
			}
			assert.True(t, ablytest.Soon.IsTrue(testFunc))
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
			channelName := ablytest.ChannelName("persisted:test")
			channel := rest.Channels.Get(channelName)

			fixtures := presenceHistoryFixtures()
			realtime := postPresenceHistoryFixtures(t, context.Background(), app, channelName, fixtures)
			defer safeclose(t, realtime, app)

			expected := c.expected

			presenceHistory := channel.Presence.History(
				ably.PresenceHistoryWithLimit(len(expected)),
				ably.PresenceHistoryWithDirection(c.direction),
			)
			testFunc := func() bool {
				err := ablytest.TestPagination(
					expected,
					presenceHistory,
					len(expected),
					ablytest.PaginationWithEqual(presenceEqual),
				)
				return err == nil
			}
			assert.True(t, ablytest.Soon.IsTrue(testFunc))
		})
	}
}

func TestPresenceGet_RSP3_RSP3a1(t *testing.T) {
	for _, limit := range []int{2, 3, 20} {
		t.Run(fmt.Sprintf("limit=%d", limit), func(t *testing.T) {

			app, rest := ablytest.NewREST()
			defer app.Close()
			channel := presenceFixturesChannel(rest)

			expected := persistedPresenceFixtures()

			var err error
			testFunc := func() bool {
				err = ablytest.TestPagination(
					expected,
					channel.Presence.Get(ably.GetPresenceWithLimit(limit)),
					limit,
					ablytest.PaginationWithEqual(presenceEqual),
					ablytest.PaginationWithSortResult(sortPresenceByClientID),
				)
				return err == nil
			}
			assert.True(t, ablytest.Soon.IsTrue(testFunc))

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
			channel := presenceFixturesChannel(rest)

			expected := persistedPresenceFixtures(func(p ablytest.Presence) bool {
				return p.ClientID == clientID
			})

			presence := channel.Presence.Get(
				ably.GetPresenceWithClientID(clientID),
			)
			testFunc := func() bool {
				err := ablytest.TestPagination(
					expected,
					presence,
					1,
					ablytest.PaginationWithEqual(presenceEqual),
					ablytest.PaginationWithSortResult(sortPresenceByClientID),
				)
				return err == nil
			}
			assert.True(t, ablytest.Soon.IsTrue(testFunc))
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
			presence := channel.Presence.Get(ably.GetPresenceWithConnectionID(connID))
			var err error
			testFunc := func() bool {
				err = ablytest.TestPagination(
					[]*ably.PresenceMessage{{
						Action:  ably.PresenceActionPresent,
						Message: expected,
					}},
					presence,
					1,
					ablytest.PaginationWithEqual(presenceEqual),
					ablytest.PaginationWithSortResult(sortPresenceByClientID),
				)
				return err == nil
			}
			assert.True(t, ablytest.Soon.IsTrue(testFunc),
				"connID %s: %w", connID, err)
			return nil
		})
	}

	err := rg.Wait()
	assert.NoError(t, err)
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
		assert.NoError(t, err,
			"at presence fixture #%d (%v): %v", i, m.Action, err)
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

// presenceFixturesChannelName is the channel pre-populated with presence
// members by the appspec. It contains a cipher-encoded member (client_encoded),
// so reads must configure the channel cipher to decode it.
const presenceFixturesChannelName = "persisted:presence_fixtures"

// presenceFixturesChannel returns the presence-fixtures channel configured with
// the cipher needed to decode the client_encoded fixture member.
func presenceFixturesChannel(rest *ably.REST) *ably.RESTChannel {
	return rest.Channels.Get(
		presenceFixturesChannelName,
		ably.ChannelWithCipher(ablytest.PresenceFixturesCipher()),
	)
}

// expectedFixtureData returns the data a presence fixture member is expected to
// have once the client has decoded it, mirroring Message decoding (TM3): members
// with a "json" encoding decode to a Go value; the cipher-encoded member decodes
// to the same value as its plaintext twin (client_decoded).
func expectedFixtureData(p ablytest.Presence) interface{} {
	if strings.Contains(p.Encoding, "json") {
		var v interface{}
		if err := json.Unmarshal([]byte(decodedFixtureJSON(p)), &v); err != nil {
			panic(err)
		}
		return v
	}
	return p.Data
}

// decodedFixtureJSON returns the plaintext JSON string for a json-encoded
// fixture member. For the cipher-encoded member this is the known plaintext
// shared with client_decoded; otherwise the data is already plaintext JSON.
func decodedFixtureJSON(p ablytest.Presence) string {
	if p.ClientID == "client_encoded" {
		return `{"example":{"json":"Object"}}`
	}
	return p.Data
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
				Data:     expectedFixtureData(p),
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

func sortPresenceByData(items []interface{}) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].(*ably.PresenceMessage).Data.(string) < items[j].(*ably.PresenceMessage).Data.(string)
	})
}
