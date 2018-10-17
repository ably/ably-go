package ably_test

import (
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
)

func TestChannel_Presence(t *testing.T) {
	t.Parallel()
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	client, err := ably.NewRestClient(app.Options())
	if err != nil {
		t.Fatal(err)
	}
	channel := client.Channels.Get("persisted:presence_fixtures", nil)
	presence := channel.Presence

	t.Run("Get", func(ts *testing.T) {
		page, err := presence.Get(nil)
		if err != nil {
			ts.Fatal(err)
		}
		n := len(page.PresenceMessages())
		expect := len(app.Config.Channels[0].Presence)
		if n != expect {
			ts.Errorf("expected %d got %d", expect, n)
		}

		ts.Run("With limit option", func(ts *testing.T) {
			limit := 2
			page1, err := presence.Get(&ably.PaginateParams{Limit: limit})
			if err != nil {
				ts.Fatal(err)
			}
			n := len(page1.PresenceMessages())
			if n != limit {
				ts.Errorf("expected %d messages got %d", limit, n)
			}
			n = len(page1.Items())
			if n != limit {
				ts.Errorf("expected %d items got %d", limit, n)
			}

			page2, err := page1.Next()
			if err != nil {
				ts.Fatal(err)
			}
			n = len(page2.PresenceMessages())
			if n != limit {
				ts.Errorf("expected %d messages got %d", limit, n)
			}
			n = len(page2.Items())
			if n != limit {
				ts.Errorf("expected %d items got %d", limit, n)
			}

			_, err = page2.Next()
			if err == nil {
				ts.Fatal("expected an error")
			}
		})
	})

	t.Run("History", func(ts *testing.T) {
		page, err := presence.History(nil)
		if err != nil {
			ts.Fatal(err)
		}
		n := len(page.PresenceMessages())
		expect := len(app.Config.Channels[0].Presence)
		if n != expect {
			ts.Errorf("expected %d got %d", expect, n)
		}

		ts.Run("with start and end time", func(ts *testing.T) {
			params := &ably.PaginateParams{
				ScopeParams: ably.ScopeParams{
					Start: ably.Time(time.Now().Add(-24 * time.Hour)),
					End:   ably.Time(time.Now()),
				},
			}
			page, err := presence.History(params)
			if err != nil {
				ts.Fatal(err)
			}
			n := len(page.PresenceMessages())
			expect := len(app.Config.Channels[0].Presence)
			if n != expect {
				ts.Errorf("expected %d got %d", expect, n)
			}
		})
	})

}
