package ably_test

import (
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"
)

func TestRestChannel(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	client, err := ably.NewRestClient(app.Options())
	if err != nil {
		t.Fatal(err)
	}
	event := "sendMessage"
	message := "A message in a bottle"
	t.Run("Publish", func(ts *testing.T) {
		channel := client.Channel("test_publish_channel")
		err := channel.Publish(event, message)
		if err != nil {
			ts.Fatal(err)
		}
		ts.Run("is available in the history", func(ts *testing.T) {
			page, err := channel.History(nil)
			if err != nil {
				ts.Fatal(err)
			}
			messages := page.Messages()
			if len(messages) == 0 {
				ts.Fatal("expected messages")
			}
			m := messages[0]
			if m.Name != event {
				ts.Errorf("expected %s got %s", event, m.Name)
			}
			if m.Data != message {
				ts.Errorf("expected %s got %s", message, m.Data)
			}
			if m.Encoding != proto.UTF8 {
				t.Errorf("expected %s got %s", proto.UTF8, m.Encoding)
			}
		})
	})
	t.Run("History", func(ts *testing.T) {
		historyRestChannel := client.Channel("channelhistory")
		for i := 0; i < 2; i++ {
			historyRestChannel.Publish("breakingnews", "Another Shark attack!!")
		}
		ts.Run("returns a paginated result", func(ts *testing.T) {
			page1, err := historyRestChannel.History(&ably.PaginateParams{Limit: 1})
			if err != nil {
				ts.Fatal(err)
			}
			size := len(page1.Messages())
			expectSize := 1
			if size != expectSize {
				ts.Errorf("expected %d got %d", expectSize, size)
			}
			expectItems := 1
			items := len(page1.Items())
			if items != expectItems {
				ts.Errorf("expected %d got %d", expectItems, items)
			}

			page2, err := page1.Next()
			if err != nil {
				ts.Fatal(err)
			}
			size = len(page2.Messages())
			expectSize = 1
			if size != expectSize {
				ts.Errorf("expected %d got %d", expectSize, size)
			}
			items = len(page2.Items())
			expectItems = 1
			if items != expectItems {
				ts.Errorf("expected %d got %d", expectItems, items)
			}
		})
	})

	t.Run("PublishAll", func(ts *testing.T) {
		encodingRestChannel := client.Channel("this?is#an?encoding#channel")
		messages := []*proto.Message{
			{Name: "send", Data: "test data 1"},
			{Name: "send", Data: "test data 2"},
		}
		err := encodingRestChannel.PublishAll(messages)
		if err != nil {
			ts.Fatal(err)
		}
		page, err := encodingRestChannel.History(&ably.PaginateParams{Limit: 2})
		if err != nil {
			ts.Fatal(err)
		}
		size := len(page.Messages())
		expectSize := 2
		if size != expectSize {
			ts.Errorf("expected %d got %d", expectSize, size)
		}
		items := len(page.Items())
		expectItems := 2
		if items != expectItems {
			ts.Errorf("expected %d got %d", expectItems, items)
		}
	})
}
