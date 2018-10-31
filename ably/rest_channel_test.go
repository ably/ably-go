package ably_test

import (
	"encoding/base64"
	"reflect"
	"sort"
	"testing"

	"github.com/ably/ably-go/ably/internal/ablyutil"

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
	options := app.Options()
	client, err := ably.NewRestClient(options)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Publish", func(ts *testing.T) {
		channel := client.Channels.Get("test_publish_channel", nil)

		type dataSample struct {
			encoding string
			data     interface{}
		}
		m := make(map[string]dataSample)
		if options.NoBinaryProtocol {
			m["string"] = dataSample{
				data: "string",
			}
			m["binary"] = dataSample{
				encoding: proto.Base64,
				data:     []byte("string"),
			}
			m["json"] = dataSample{
				encoding: proto.JSON,
				data: map[string]interface{}{
					"key": "value",
				},
			}
		} else {
			m["string"] = dataSample{
				data: "string",
			}
			m["binary"] = dataSample{
				data: "string",
			}
			m["json"] = dataSample{
				encoding: proto.JSON,
				data: map[string]interface{}{
					"key": "value",
				},
			}
		}
		for k, v := range m {
			err := channel.Publish(k, v.data)
			if err != nil {
				ts.Fatal(err)
			}
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
			for _, v := range messages {
				e := m[v.Name]
				if v.Encoding != e.encoding {
					ts.Errorf("expected %s got %s ", e.encoding, v.Encoding)
				}
			}
		})
	})

	t.Run("History", func(ts *testing.T) {
		historyRestChannel := client.Channels.Get("channelhistory", nil)
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
		encodingRestChannel := client.Channels.Get("this?is#an?encoding#channel", nil)
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

	t.Run("encryption", func(ts *testing.T) {
		key, err := base64.StdEncoding.DecodeString("WUP6u0K7MXI5Zeo0VppPwg==")
		if err != nil {
			ts.Fatal(err)
		}
		iv, err := base64.StdEncoding.DecodeString("HO4cYSP8LybPYBPZPHQOtg==")
		if err != nil {
			ts.Fatal(err)
		}
		opts := &proto.ChannelOptions{
			Cipher: proto.CipherParams{
				Key:       key,
				KeyLength: 128,
				IV:        iv,
				Algorithm: proto.AES,
			},
		}
		channelName := "encrypted_channel"
		channel := client.Channels.Get(channelName, opts)
		sample := []struct {
			event, message string
		}{
			{"publish_0", "first message"},
			{"publish_1", "second message"},
		}
		for _, v := range sample {
			err := channel.Publish(v.event, v.message)
			if err != nil {
				ts.Error(err)
			}
		}

		rst, err := channel.History(nil)
		if err != nil {
			ts.Fatal(err)
		}
		msg := rst.Messages()
		if len(msg) != len(sample) {
			t.Errorf("expected %d messages got %d", len(sample), len(msg))
		}
		sort.Slice(msg, func(i, j int) bool {
			return msg[i].Name < msg[j].Name
		})
		for k, v := range msg {
			e := sample[k]
			if v.Name != e.event {
				t.Errorf("expected %s got %s", e.event, v.Name)
			}
			if !reflect.DeepEqual(v.Data, e.message) {
				t.Errorf("expected %s got %v", e.message, v.Data)
			}
		}
	})
}

func TestIdempotentPublishing(t *testing.T) {
	t.Parallel()
	app, err := ablytest.NewSandboxWIthEnv(nil, "idempotent-dev")
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	options := app.Options()
	client, err := ably.NewRestClient(options)
	if err != nil {
		t.Fatal(err)
	}
	randomStr, err := ablyutil.BaseID()
	if err != nil {
		t.Fatal(err)
	}
	t.Run("when ID is not included (#RSL1k2)", func(ts *testing.T) {
		channel := client.Channels.Get("idempotent_test_1", nil)
		for range make([]struct{}, 3) {
			err := channel.Publish("", randomStr)
			if err != nil {
				ts.Fatal(err)
			}
		}
		res, err := channel.History(nil)
		if err != nil {
			ts.Fatal(err)
		}
		n := len(res.Items())
		if n != 3 {
			ts.Errorf("expected %d got %d", 3, n)
		}
	})

	t.Run("when ID is included (#RSL1k2, #RSL1k5)", func(ts *testing.T) {
		channel := client.Channels.Get("idempotent_test_2", nil)
		for range make([]struct{}, 3) {
			err := channel.PublishAll([]*proto.Message{
				{
					ID:   randomStr,
					Data: randomStr,
				},
			})
			if err != nil {
				ts.Fatal(err)
			}
		}
		res, err := channel.History(nil)
		if err != nil {
			ts.Fatal(err)
		}
		n := len(res.Items())
		if n != 1 {
			// three REST publishes result in only one message being published
			ts.Errorf("expected %d got %d", 1, n)
		}
	})

}
