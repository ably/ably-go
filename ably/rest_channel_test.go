package ably_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
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
		msg := res.Messages()[0]
		if msg.ID != randomStr {
			ts.Errorf("expected id to be %s got %s", randomStr, msg.ID)
		}
	})

	t.Run("multiple messages in one publish operation (#RSL1k3)", func(ts *testing.T) {
		channel := client.Channels.Get("idempotent_test_3", nil)
		err := channel.PublishAll([]*proto.Message{
			{
				ID:   randomStr,
				Data: randomStr,
			},
			{
				ID:   randomStr,
				Data: randomStr,
			},
			{
				ID:   randomStr,
				Data: randomStr,
			},
		})
		if err == nil {
			ts.Fatal("expected an error")
		}
		code := fmt.Sprint(ably.ErrInvalidPublishRequestInvalidClientSpecifiedID)
		if !strings.Contains(err.Error(), code) {
			ts.Errorf("expected error code %s got %s", code, err)
		}
	})

	t.Run("multiple messages in one publish operation with IDs following the required format described in RSL1k1 (#RSL1k3)", func(ts *testing.T) {
		channel := client.Channels.Get("idempotent_test_4", nil)
		var m []*proto.Message
		for i := 0; i < 3; i++ {
			m = append(m, &proto.Message{
				ID: fmt.Sprintf("%s:%d", randomStr, i),
			})
		}
		err := channel.PublishAll(m)
		if err != nil {
			ts.Fatal(err)
		}
		res, err := channel.History(nil)
		if err != nil {
			ts.Fatal(err)
		}
		n := len(res.Items())
		if n != 3 {
			ts.Errorf("expected %d got %d", 3, n)
		}
		messages := res.Messages()

		// we need to sort so we we can easily test the serial in order.
		sort.Slice(messages, func(i, j int) bool {
			p := strings.Split(messages[i].ID, ":")
			p0 := strings.Split(messages[j].ID, ":")
			i1, err := strconv.Atoi(p[1])
			if err != nil {
				ts.Fatal(err)
			}
			i2, err := strconv.Atoi(p0[1])
			if err != nil {
				ts.Fatal(err)
			}
			return i1 < i2
		})
		for k, v := range messages {
			p := strings.Split(v.ID, ":")
			id, err := strconv.Atoi(p[1])
			if err != nil {
				ts.Fatal(err)
			}
			if id != k {
				ts.Errorf("expected serial to be %d got %d", k, id)
			}
		}
	})

	t.Run("the ID is populated with a random ID and serial 0 from this lib (#RSL1k1)", func(ts *testing.T) {
		channel := client.Channels.Get("idempotent_test_5", nil)
		err := channel.Publish("event", "")
		if err != nil {
			ts.Fatal(err)
		}
		res, err := channel.History(nil)
		if err != nil {
			ts.Fatal(err)
		}
		m := res.Messages()
		if len(m) != 1 {
			ts.Fatalf("expected %d got %d", 1, len(m))
		}
		message := m[0]
		if message.ID == "" {
			ts.Fatal("expected message id not to be empty")
		}
		pattern := `^[A-Za-z0-9\+\/]+:0$`
		re := regexp.MustCompile(pattern)
		if !re.MatchString(message.ID) {
			ts.Fatalf("expected id %s to match pattern %q", message.ID, pattern)
		}
		baseID := strings.Split(message.ID, ":")[0]
		v, err := base64.StdEncoding.DecodeString(baseID)
		if err != nil {
			ts.Fatal(err)
		}
		if len(v) != 9 {
			ts.Errorf("expected %d bytes git %d", 9, len(v))
		}
	})

	t.Run("publishing a batch of messages", func(ts *testing.T) {
		channel := client.Channels.Get("idempotent_test_6", nil)
		name := "event"
		err := channel.PublishAll([]*proto.Message{
			{Name: name},
			{Name: name},
			{Name: name},
		})
		if err != nil {
			ts.Fatal(err)
		}
		res, err := channel.History(nil)
		if err != nil {
			ts.Fatal(err)
		}
		m := res.Messages()
		if len(m) != 3 {
			ts.Fatalf("expected %d got %d", 1, len(m))
		}
		pattern := `^[A-Za-z0-9\+\/]+:\d$`
		re := regexp.MustCompile(pattern)
		for _, message := range m {
			if message.ID == "" {
				ts.Fatal("expected message id not to be empty")
			}
			if !re.MatchString(message.ID) {
				ts.Fatalf("expected id %s to match pattern %q", message.ID, pattern)
			}
			baseID := strings.Split(message.ID, ":")[0]
			v, err := base64.StdEncoding.DecodeString(baseID)
			if err != nil {
				ts.Fatal(err)
			}
			if len(v) != 9 {
				ts.Errorf("expected %d bytes git %d", 9, len(v))
			}
		}
	})
}

func TestIdempotent_retry(t *testing.T) {
	t.Parallel()
	app, err := ablytest.NewSandboxWIthEnv(nil, "idempotent-dev")
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	randomStr, err := ablyutil.BaseID()
	if err != nil {
		t.Fatal(err)
	}
	t.Run("when there is a network failure triggering an automatic retry (#RSL1k4)", func(ts *testing.T) {
		var retryCount int
		var hosts []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hosts = append(hosts, r.Host)
			retryCount++
			w.WriteHeader(http.StatusInternalServerError)
		}))

		nopts := &ably.ClientOptions{
			NoTLS:                   true,
			FallbackHostsUseDefault: true,
			AuthOptions: ably.AuthOptions{
				UseTokenAuth: true,
			},
			HTTPClient: &http.Client{
				Transport: &http.Transport{
					Proxy: func(r *http.Request) (*url.URL, error) {
						if strings.HasPrefix(r.URL.Path, "/channels/") {
							if retryCount == 2 {
								return url.Parse(fmt.Sprintf("https://%s", hosts[0]))
							}
							return url.Parse(server.URL)
						}
						return r.URL, nil
					},
				},
			},
		}
		client, err := ably.NewRestClient(app.Options(nopts))
		if err != nil {
			ts.Fatal(err)
		}

		ts.Run("two REST publish retries result in only one message being published'", func(ts *testing.T) {
			channel := client.Channels.Get("idempotent_test_fallback", nil)
			err = channel.Publish("", randomStr)
			if err != nil {
				ts.Error(err)
			}
			if retryCount != 2 {
				ts.Errorf("expected %d retry attempts got %d %#v", 2, retryCount, hosts)
			}
			res, err := channel.History(nil)
			if err != nil {
				ts.Fatal(err)
			}
			m := res.Messages()
			if len(m) != 1 {
				ts.Errorf("expected %d messages got %d", 1, len(m))
			}
		})

	})
}
