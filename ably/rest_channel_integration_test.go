//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ablytest"

	"github.com/stretchr/testify/assert"
)

func TestRESTChannel(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	assert.NoError(t, err)
	defer app.Close()
	options := app.Options()
	client, err := ably.NewREST(options...)
	assert.NoError(t, err)
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
			assert.NoError(t, err)
		}
		t.Run("is available in the history", func(t *testing.T) {
			var messages []*ably.Message
			err := ablytest.AllPages(&messages, channel.History())
			assert.NoError(t, err)
			assert.NotEqual(t, 0, len(messages),
				"expected messages")
		})
	})

	t.Run("PublishMultiple", func(t *testing.T) {
		encodingRESTChannel := client.Channels.Get("this?is#an?encoding#channel")
		messages := []*ably.Message{
			{Name: "send", Data: "test data 1"},
			{Name: "send", Data: "test data 2"},
		}
		err := encodingRESTChannel.PublishMultiple(context.Background(), messages)
		assert.NoError(t, err)
		var history []*ably.Message
		err = ablytest.AllPages(&history, encodingRESTChannel.History(ably.HistoryWithLimit(2)))
		assert.NoError(t, err)
		assert.Equal(t, 2, len(history),
			"expected 2 messages got %d", len(history))
	})

	t.Run("encryption", func(t *testing.T) {
		key, err := base64.StdEncoding.DecodeString("WUP6u0K7MXI5Zeo0VppPwg==")
		assert.NoError(t, err)
		iv, err := base64.StdEncoding.DecodeString("HO4cYSP8LybPYBPZPHQOtg==")
		assert.NoError(t, err)
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
			assert.NoError(t, err)
		}

		var msg []*ably.Message
		err = ablytest.AllPages(&msg, channel.History())
		assert.NoError(t, err)
		sort.Slice(msg, func(i, j int) bool {
			return msg[i].Name < msg[j].Name
		})

		assert.Equal(t, "publish_0", msg[0].Name,
			"expected \"publish_0\" got %s", msg[0].Name)
		assert.Equal(t, "first message", msg[0].Data,
			"expected \"first message\" got %v", msg[0].Data)
		assert.Equal(t, "publish_1", msg[1].Name,
			"expected \"publish_1\" got %s", msg[1].Name)
		assert.Equal(t, "second message", msg[1].Data,
			"expected \"second message\" got %v", msg[1].Data)
	})
}

func TestIdempotentPublishing(t *testing.T) {
	app, err := ablytest.NewSandboxWithEndpoint(nil, ablytest.Endpoint, ablytest.Environment)
	assert.NoError(t, err)
	defer app.Close()
	options := app.Options(ably.WithIdempotentRESTPublishing(true))
	client, err := ably.NewREST(options...)
	assert.NoError(t, err)
	randomStr, err := ablyutil.BaseID()
	assert.NoError(t, err)
	t.Run("when ID is not included (#RSL1k2)", func(t *testing.T) {
		channel := client.Channels.Get("idempotent_test_1")
		for range make([]struct{}, 3) {
			err := channel.Publish(context.Background(), "", randomStr)
			assert.NoError(t, err)
		}
		var history []*ably.Message
		err := ablytest.AllPages(&history, channel.History())
		assert.NoError(t, err)
		assert.Equal(t, 3, len(history),
			"expected 3 got %d", len(history))
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
			assert.NoError(t, err)
		}
		var history []*ably.Message
		err := ablytest.AllPages(&history, channel.History())
		assert.NoError(t, err)
		assert.Equal(t, 1, len(history),
			"expected 1 got %d", len(history))
		msg := history[0]
		assert.Equal(t, randomStr, msg.ID,
			"expected id to be %s got %s", randomStr, msg.ID)
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
		assert.Error(t, err,
			"expected an error")
		assert.Contains(t, err.Error(), "40031",
			"expected error to contain code 40031 got %s", err.Error())
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
		assert.NoError(t, err)
		var messages []*ably.Message
		err = ablytest.AllPages(&messages, channel.History())
		assert.NoError(t, err)
		assert.Equal(t, 3, len(messages),
			"expected 3 got %d", len(messages))

		// we need to sort so we can easily test the serial in order.
		sort.Slice(messages, func(i, j int) bool {
			p := strings.Split(messages[i].ID, ":")
			p0 := strings.Split(messages[j].ID, ":")
			i1, err := strconv.Atoi(p[1])
			assert.NoError(t, err)
			i2, err := strconv.Atoi(p0[1])
			assert.NoError(t, err)
			return i1 < i2
		})
		for k, v := range messages {
			p := strings.Split(v.ID, ":")
			id, err := strconv.Atoi(p[1])
			assert.NoError(t, err)
			assert.Equal(t, k, id,
				"expected serial to be %d got %d", k, id)
		}
	})

	t.Run("the ID is populated with a random ID and serial 0 from this lib (#RSL1k1)", func(t *testing.T) {
		channel := client.Channels.Get("idempotent_test_5")
		err := channel.Publish(context.Background(), "event", "")
		assert.NoError(t, err)
		var m []*ably.Message
		err = ablytest.AllPages(&m, channel.History())
		assert.NoError(t, err)
		assert.Equal(t, 1, len(m),
			"expected 1 got %d", len(m))
		message := m[0]
		assert.NotEqual(t, "", message,
			"expected message id not to be empty")
		pattern := `^[A-Za-z0-9\+\/]+:0$`
		re := regexp.MustCompile(pattern)
		assert.True(t, re.MatchString(message.ID),
			"expected id %s to match pattern %q", message.ID, pattern)
		baseID := strings.Split(message.ID, ":")[0]
		v, err := base64.StdEncoding.DecodeString(baseID)
		assert.NoError(t, err)
		assert.Equal(t, 9, len(v),
			"expected 9 bytes got %d", len(v))
	})

	t.Run("publishing a batch of messages", func(t *testing.T) {
		channel := client.Channels.Get("idempotent_test_6")
		name := "event"
		err := channel.PublishMultiple(context.Background(), []*ably.Message{
			{Name: name},
			{Name: name},
			{Name: name},
		})
		assert.NoError(t, err)
		var m []*ably.Message
		err = ablytest.AllPages(&m, channel.History())
		assert.NoError(t, err)
		assert.Equal(t, 3, len(m),
			"expected 3 got %d", 1, len(m))
		pattern := `^[A-Za-z0-9\+\/]+:\d$`
		re := regexp.MustCompile(pattern)
		for _, message := range m {
			assert.NotEqual(t, message.ID, "",
				"expected message id not to be empty")
			assert.True(t, re.MatchString(message.ID),
				"expected id %s to match pattern %q", message.ID, pattern)
			baseID := strings.Split(message.ID, ":")[0]
			v, err := base64.StdEncoding.DecodeString(baseID)
			assert.NoError(t, err)
			assert.Equal(t, 9, len(v),
				"expected 9 bytes got %d", len(v))
		}
	})
}

func TestIdempotent_retry(t *testing.T) {
	app, err := ablytest.NewSandboxWithEndpoint(nil, ablytest.Endpoint, ablytest.Environment)
	assert.NoError(t, err)
	defer app.Close()
	randomStr, err := ablyutil.BaseID()
	assert.NoError(t, err)
	t.Run("when there is a network failure triggering an automatic retry (#RSL1k4)", func(t *testing.T) {
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
			ably.WithEndpoint(ablytest.Endpoint),
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
		assert.NoError(t, err)

		t.Run("two REST publish retries result in only one message being published'", func(t *testing.T) {
			channel := client.Channels.Get("idempotent_test_fallback")
			err = channel.Publish(context.Background(), "", randomStr)
			assert.NoError(t, err)
			assert.Equal(t, 2, retryCount,
				"expected 2 retry attempts got %d", retryCount)
			var m []*ably.Message
			err = ablytest.AllPages(&m, channel.History())
			assert.NoError(t, err)
			assert.Equal(t, 1, len(m),
				"expected 1 message got %d", len(m))
		})
		t.Run("or multiple messages in one publish operation'", func(t *testing.T) {
			retryCount = 0
			channel := client.Channels.Get("idempotent_test_fallback_1")
			msgs := []*ably.Message{
				{Data: randomStr},
				{Data: randomStr},
				{Data: randomStr},
			}
			for range msgs {
				channel.PublishMultiple(context.Background(), msgs)
			}
			var m []*ably.Message
			err := ablytest.AllPages(&m, channel.History())
			assert.NoError(t, err)
			assert.Equal(t, len(msgs), len(m),
				"expected %d messages got %d", len(msgs), len(m))
		})
	})
}
