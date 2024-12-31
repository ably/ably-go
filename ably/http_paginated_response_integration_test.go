//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"

	"github.com/stretchr/testify/assert"
)

func TestHTTPPaginatedFallback(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	assert.NoError(t, err)
	defer app.Close()
	opts := app.Options(ably.WithUseBinaryProtocol(false),
		ably.WithEndpoint("ably.invalid"),
		ably.WithFallbackHosts(nil))
	client, err := ably.NewREST(opts...)
	assert.NoError(t, err)
	t.Run("request_time", func(t *testing.T) {
		_, err := client.Request("get", "/time").Pages(context.Background())
		assert.Error(t, err)
	})
}

func TestHTTPPaginatedResponse(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	assert.NoError(t, err)
	defer app.Close()
	client, err := ably.NewREST(app.Options()...)
	assert.NoError(t, err)
	t.Run("request_time", func(t *testing.T) {
		res, err := client.Request("get", "/time").Pages(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode(),
			"expected %d got %d", http.StatusOK, res.StatusCode())
		assert.True(t, res.Success(), "expected success to be true")
		res.Next(context.Background())
		var items []interface{}
		err = res.Items(&items)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(items),
			"expected 1 item got %d", len(items))
	})

	t.Run("request_404", func(t *testing.T) {
		res, err := client.Request("get", "/keys/ablyjs.test/requestToken").Pages(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, res.StatusCode(),
			"expected %d got %d", http.StatusNotFound, res.StatusCode())
		assert.Equal(t, ably.ErrNotFound, res.ErrorCode(),
			"expected %d got %d", ably.ErrNotFound, res.ErrorCode())
		assert.False(t, res.Success(),
			"expected success to be false")
		assert.NotEqual(t, "", res.ErrorMessage(),
			"expected error message")
	})

	t.Run("request_post_get_messages", func(t *testing.T) {
		channelName := "http-paginated-result"
		channelPath := "/channels/" + channelName + "/messages"
		msgs := []ably.Message{
			{Name: "faye", Data: "whittaker"},
			{Name: "martin", Data: "reed"},
		}

		t.Run("post", func(t *testing.T) {
			for _, message := range msgs {
				res, err := client.Request("POST", channelPath, ably.RequestWithBody(message)).Pages(context.Background())
				assert.NoError(t, err)
				assert.Equal(t, http.StatusCreated, res.StatusCode(),
					"expected %d got %d", http.StatusCreated, res.StatusCode())
				assert.True(t, res.Success(),
					"expected success to be true")
				res.Next(context.Background())
				var items []interface{}
				err = res.Items(&items)
				assert.NoError(t, err)
				assert.Equal(t, 1, len(items), "expected 1 item got %d", len(items))
			}
		})

		t.Run("get", func(t *testing.T) {
			res, err := client.Request("get", channelPath, ably.RequestWithParams(url.Values{
				"limit":     {"1"},
				"direction": {"forwards"},
			})).Pages(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, res.StatusCode(),
				"expected %d got %d", http.StatusOK, res.StatusCode())
			res.Next(context.Background())
			var items []map[string]interface{}
			err = res.Items(&items)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(items), "expected 1 item got %d", len(items))
			item := items[0]
			name := item["name"]
			data := item["data"]
			assert.Equal(t, "faye", name, "expected name \"faye\" got %s", name)
			assert.Equal(t, "whittaker", data, "expected data \"whittaker\" got %s", data)

			if !res.Next(context.Background()) {
				t.Fatal(res.Err())
			}
			err = res.Items(&items)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(items),
				"expected 1 item got %d", len(items))
			item = items[0]
			name = item["name"]
			data = item["data"]
			assert.Equal(t, "martin", name,
				"expected name \"martin\" got %s", name)
			assert.Equal(t, "reed", data,
				"expected data \"reed\" got %s", data)
		})

		var msg []*ably.Message
		err := ablytest.AllPages(&msg, client.Channels.Get(channelName).History())
		assert.NoError(t, err)
		sort.Slice(msg, func(i, j int) bool {
			return msg[i].Name < msg[j].Name
		})
		assert.Equal(t, 2, len(msg),
			"expected 2 items in message got %d", len(msg))
		assert.Equal(t, "faye", msg[0].Name,
			"expected name \"faye\" got %s", msg[0].Name)
		assert.Equal(t, "whittaker", msg[0].Data,
			"expected data \"whittaker\" got %s", msg[0].Data)
	})
}
