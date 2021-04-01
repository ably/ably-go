package ably_test

import (
	"context"
	"testing"

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
	client, err := ably.NewREST(app.Options()...)
	if err != nil {
		t.Fatal(err)
	}
	channel := client.Channels.Get("persisted:presence_fixtures")
	presence := channel.Presence

	t.Run("Get", func(ts *testing.T) {
		page, err := presence.Get(context.Background(), nil)
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
			page1, err := presence.Get(context.Background(), &ably.PaginateParams{Limit: limit})
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

			page2, err := page1.Next(context.Background())
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

			noPage, err := page2.Next(context.Background())
			if err != nil {
				ts.Fatal(err)
			}
			if noPage != nil {
				ts.Fatal("no more pages expected")
			}
		})
	})
}
