package ably_test

import (
	"context"
	"crypto/tls"
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
	"github.com/ably/ably-go/ablytest"
)

func TestRESTChannel(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	options := app.Options()
	client, err := ably.NewREST(options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Publish", func(t *testing.T) {
		channel := client.Channels.Get("test_publish_channel")

		type dataSample struct {
			encoding string
			data     interface{}
		}
		m := make(map[string]dataSample)
		if ablytest.NoBinaryProtocol {
			m["string"] = dataSample{
				encoding: ably.EncUTF8,
				data:     "string",
			}
			m["binary"] = dataSample{
				encoding: ably.EncBase64,
				data:     []byte("string"),
			}
			m["json"] = dataSample{
				encoding: ably.EncJSON,
				data: map[string]interface{}{
					"key": "value",
				},
			}
		} else {
			m["string"] = dataSample{
				encoding: ably.EncUTF8,
				data:     "string",
			}
			m["binary"] = dataSample{
				encoding: ably.EncUTF8,
				data:     "string",
			}
			m["json"] = dataSample{
				encoding: ably.EncJSON,
				data: map[string]interface{}{
					"key": "value",
				},
			}
		}
		for k, v := range m {
			err := channel.Publish(context.Background(), k, v.data)
			if err != nil {
				t.Fatal(err)
			}
		}
		t.Run("is available in the history", func(t *testing.T) {
			var messages []*ably.Message
			err := ablytest.AllPages(&messages, channel.History())
			if err != nil {
				t.Fatal(err)
			}
			if len(messages) == 0 {
				t.Fatal("expected messages")
			}
		})
	})

	t.Run("PublishMultiple", func(t *testing.T) {
		encodingRESTChannel := client.Channels.Get("this?is#an?encoding#channel")
		messages := []*ably.Message{
			{Name: "send", Data: "test data 1"},
			{Name: "send", Data: "test data 2"},
		}
		err := encodingRESTChannel.PublishMultiple(context.Background(), messages)
		if err != nil {
			t.Fatal(err)
		}
		var history []*ably.Message
		err = ablytest.AllPages(&history, encodingRESTChannel.History(ably.HistoryWithLimit(2)))
		if err != nil {
			t.Fatal(err)
		}
		if len(history) != 2 {
			t.Errorf("expected 2 messages got %d", len(history))
		}
	})

	t.Run("encryption", func(t *testing.T) {
		key, err := base64.StdEncoding.DecodeString("WUP6u0K7MXI5Zeo0VppPwg==")
		if err != nil {
			t.Fatal(err)
		}
		iv, err := base64.StdEncoding.DecodeString("HO4cYSP8LybPYBPZPHQOtg==")
		if err != nil {
			t.Fatal(err)
		}
		cipher := ably.CipherParams{
			Key:       key,
			KeyLength: 128,
			Algorithm: ably.CipherAES,
		}
		cipher.SetIV(iv)
		opts := []ably.ChannelOption{ably.ChannelWithCipher(cipher)}
		channelName := "encrypted_channel"
		channel := client.Channels.Get(channelName, opts...)
		sample := []struct {
			event, message string
		}{
			{"publish_0", "first message"},
			{"publish_1", "second message"},
		}
		for _, v := range sample {
			err := channel.Publish(context.Background(), v.event, v.message)
			if err != nil {
				t.Error(err)
			}
		}

		var msg []*ably.Message
		err = ablytest.AllPages(&msg, channel.History())
		if err != nil {
			t.Fatal(err)
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
	app, err := ablytest.NewSandboxWIthEnv(nil, ablytest.Environment)
	assertNil(t, err)
	defer app.Close()

	options := app.Options(ably.WithIdempotentRESTPublishing(true))
	client, err := ably.NewREST(options...)
	assertNil(t, err)

	randomStr, err := ablyutil.BaseID()
	assertNil(t, err)

	t.Run("when ID is not included (#RSL1k2)", func(t *testing.T) {
		channel := client.Channels.Get("idempotent_test_1")
		for range make([]struct{}, 3) {
			err := channel.Publish(context.Background(), "", randomStr)
			assertNil(t, err)
		}

		var history []*ably.Message
		err := ablytest.AllPages(&history, channel.History())
		assertNil(t, err)
		assertEquals(t, 3, len(history))
	})

	t.Run("when ID is included (#RSL1k2, #RSL1k5)", func(t *testing.T) {
		channel := client.Channels.Get("idempotent_test_2")
		for range make([]struct{}, 3) {
			err := channel.PublishMultiple(context.Background(), []*ably.Message{
				{
					ID:   randomStr,
					Data: randomStr,
				},
			})
			assertNil(t, err)
		}

		var history []*ably.Message
		err := ablytest.AllPages(&history, channel.History())
		assertNil(t, err)
		assertEquals(t, 1, len(history)) // three REST publishes result in only one message being published
		assertEquals(t, randomStr, history[0].ID)
	})

	t.Run("multiple messages in one publish operation (#RSL1k3)", func(t *testing.T) {
		channel := client.Channels.Get("idempotent_test_3")
		err := channel.PublishMultiple(context.Background(), []*ably.Message{
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
			t.Fatal("expected an error")
		}
		code := fmt.Sprint(ably.ErrInvalidPublishRequestInvalidClientSpecifiedID)
		if !strings.Contains(err.Error(), code) {
			t.Errorf("expected error code %s got %s", code, err)
		}
	})

	t.Run("multiple messages in one publish operation with IDs following the required format described in RSL1k1 (#RSL1k3)", func(t *testing.T) {
		channel := client.Channels.Get("idempotent_test_4")
		var m []*ably.Message
		for i := 0; i < 3; i++ {
			m = append(m, &ably.Message{
				ID: fmt.Sprintf("%s:%d", randomStr, i),
			})
		}
		err := channel.PublishMultiple(context.Background(), m)
		assertNil(t, err)

		var messages []*ably.Message
		err = ablytest.AllPages(&messages, channel.History())
		assertNil(t, err)
		assertEquals(t, 3, len(messages))

		// we need to sort so we can easily test the serial in order.
		sort.Slice(messages, func(i, j int) bool {
			p := strings.Split(messages[i].ID, ":")
			p0 := strings.Split(messages[j].ID, ":")
			i1, err := strconv.Atoi(p[1])
			assertNil(t, err)

			i2, err := strconv.Atoi(p0[1])
			assertNil(t, err)

			return i1 < i2
		})

		for msgIndex, msg := range messages {
			p := strings.Split(msg.ID, ":")
			msgSerial, err := strconv.Atoi(p[1])
			assertNil(t, err)
			assertEquals(t, msgIndex, msgSerial)
		}
	})

	t.Run("the ID is populated with a random ID and serial 0 from this lib (#RSL1k1)", func(t *testing.T) {
		channel := client.Channels.Get("idempotent_test_5")
		err := channel.Publish(context.Background(), "event", "")
		assertNil(t, err)

		var msgHistory []*ably.Message
		err = ablytest.AllPages(&msgHistory, channel.History())
		assertNil(t, err)

		assertEquals(t, 1, len(msgHistory))

		message := msgHistory[0]
		if message.ID == "" {
			t.Fatal("expected message id not to be empty")
		}
		pattern := `^[A-Za-z0-9\+\/]+:0$`
		re := regexp.MustCompile(pattern)
		if !re.MatchString(message.ID) {
			t.Fatalf("expected id %s to match pattern %q", message.ID, pattern)
		}
		baseID := strings.Split(message.ID, ":")[0]
		decodedMsgIdempotentId, err := base64.StdEncoding.DecodeString(baseID)
		assertNil(t, err)
		assertEquals(t, 9, len(decodedMsgIdempotentId))
	})

	t.Run("publishing a batch of messages", func(t *testing.T) {
		channel := client.Channels.Get("idempotent_test_6")
		name := "event"
		err := channel.PublishMultiple(context.Background(), []*ably.Message{
			{Name: name},
			{Name: name},
			{Name: name},
		})
		assertNil(t, err)

		var messages []*ably.Message
		err = ablytest.AllPages(&messages, channel.History())
		assertNil(t, err)
		assertEquals(t, 3, len(messages))

		pattern := `^[A-Za-z0-9\+\/]+:\d$`
		re := regexp.MustCompile(pattern)
		for _, message := range messages {
			if message.ID == "" {
				t.Fatal("expected message id not to be empty")
			}
			if !re.MatchString(message.ID) {
				t.Fatalf("expected id %s to match pattern %q", message.ID, pattern)
			}
			baseID := strings.Split(message.ID, ":")[0]
			decodedMsgIdempotentId, err := base64.StdEncoding.DecodeString(baseID)
			assertNil(t, err)
			assertEquals(t, 9, len(decodedMsgIdempotentId))
		}
	})
}

func TestIdempotent_retry_RSL1k4(t *testing.T) {
	app, err := ablytest.NewSandboxWIthEnv(nil, ablytest.Environment)
	assertNil(t, err)

	defer app.Close()
	randomStr, err := ablyutil.BaseID()
	assertNil(t, err)

	t.Run("when there is a network failure triggering an automatic retry", func(t *testing.T) {
		var retryCount int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			retryCount++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		// set up the proxy to forward the second retry to the correct endpoint,
		// failing all others via the test server
		fallbackHosts := []string{"fallback0", "fallback1", "fallback2"}
		nopts := []ably.ClientOption{
			ably.WithEnvironment(ablytest.Environment),
			ably.WithTLS(false),
			ably.WithFallbackHosts(fallbackHosts),
			ably.WithIdempotentRESTPublishing(true),
			ably.WithUseTokenAuth(true),
		}

		serverURL, _ := url.Parse(server.URL)
		defaultURL, _ := url.Parse(ably.ApplyOptionsWithDefaults(nopts...).RestURL())
		proxy := func(r *http.Request) (*url.URL, error) {
			if !strings.HasPrefix(r.URL.Path, "/channels/") {
				// this is to handle token requests
				// set the Host in the request to the intended destination
				r.Host = defaultURL.Hostname()
				return defaultURL, nil
			}
			if retryCount < 2 {
				// ensure initial requests fail
				return serverURL, nil
			} else {
				// allow subsequent requests tos ucceed
				// set the Host in the request to the intended destination
				r.Host = defaultURL.Hostname()
				return defaultURL, nil
			}
		}
		nopts = append(nopts, ably.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				Proxy:        proxy,
				TLSNextProto: map[string]func(authority string, c *tls.Conn) http.RoundTripper{},
			},
		}))

		client, err := ably.NewREST(app.Options(nopts...)...)
		assertNil(t, err)

		t.Run("for multiple retries, channel publish should publish only once", func(t *testing.T) {
			channel := client.Channels.Get("idempotent_test_fallback")
			err = channel.Publish(context.Background(), "", randomStr)
			assertNil(t, err)
			assertEquals(t, 2, retryCount)

			var m []*ably.Message
			err = ablytest.AllPages(&m, channel.History())
			assertNil(t, err)
			assertEquals(t, 1, len(m))
			assertEquals(t, randomStr, m[0].Data)
		})

		t.Run("for multiple retries, channel publishMultiple/publishBatch should publish only once", func(t *testing.T) {
			retryCount = 0
			channel := client.Channels.Get("idempotent_test_fallback_1")
			msgs := []*ably.Message{
				{Data: randomStr},
				{Data: randomStr},
				{Data: randomStr},
			}

			for range msgs {
				err := channel.PublishMultiple(context.Background(), msgs)
				assertNil(t, err)
				assertEquals(t, 2, retryCount)
			}

			var m []*ably.Message
			err := ablytest.AllPages(&m, channel.History())
			assertNil(t, err)
			assertEquals(t, len(msgs), len(m))
			for _, message := range m {
				assertEquals(t, randomStr, message.Data)
			}
		})
	})
}
