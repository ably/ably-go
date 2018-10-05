package ably_test

import (
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"
)

func TestRestChannel(t *testing.T) {
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
	channel := client.Channel("test_rest_channel")
	event := "sendMessage"
	message := "A message in a bottle"

	t.Run("publishing a message", func(ts *testing.T) {
		err := channel.Publish(event, message)
		if err != nil {
			ts.Fatal(err)
		}
		page, err := channel.History(nil)
		if err != nil {
			ts.Fatal(err)
		}

		messages := page.Messages()
		if len(messages) == 0 {
			ts.Fatal("expected more messages")
		}
		if messages[0].Name != event {
			ts.Errorf("expected %s got %s", event, messages[0].Name)
		}

		if messages[0].Data != message {
			ts.Errorf("expected %s got %s", message, messages[0].Data)
		}
		if messages[0].Encoding != proto.UTF8 {
			ts.Errorf("expected %s got %s", proto.UTF8, messages[0].Encoding)
		}
	})

	t.Run("History", func(ts *testing.T) {
		historyRestChannel := client.Channel("channelhistory")

		for i := 0; i < 2; i++ {
			historyRestChannel.Publish("breakingnews", "Another Shark attack!!")
		}

		page1, err := historyRestChannel.History(&ably.PaginateParams{Limit: 1})
		if err != nil {
			ts.Fatal(err)
		}
		if len(page1.Messages()) != 1 {
			ts.Errorf("expected 1 message got %d", len(page1.Messages()))
		}
		if len(page1.Items()) != 1 {
			ts.Errorf("expected 1 item got %d", len(page1.Items()))
		}

		page2, err := page1.Next()
		if err != nil {
			ts.Fatal(err)
		}
		if len(page2.Messages()) != 1 {
			ts.Errorf("expected 1 message got %d", len(page2.Messages()))
		}
		if len(page2.Items()) != 1 {
			ts.Errorf("expected 1 item got %d", len(page2.Items()))
		}
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
		if len(page.Messages()) != 2 {
			ts.Errorf("expected 2 messages got %d", len(page.Messages()))
		}
		if len(page.Items()) != 2 {
			ts.Errorf("expected 2 items got %d", len(page.Items()))
		}
	})
}
