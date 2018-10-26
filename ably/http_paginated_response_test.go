package ably_test

import (
	"net/http"
	"testing"

	"github.com/ably/ably-go/ably"

	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"
)

func TestHTTPPaginatedResponse(t *testing.T) {
	t.Parallel()
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	opts := app.Options()
	opts.NoBinaryProtocol = true
	client, err := ably.NewRestClient(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("request_time", func(ts *testing.T) {
		res, err := client.Request("get", "/time", nil, nil, nil)
		if err != nil {
			ts.Fatal(err)
		}
		if res.StatusCode != http.StatusOK {
			ts.Errorf("expected %d got %d", http.StatusOK, res.StatusCode)
		}
		if !res.Success {
			ts.Error("expected success to be true")
		}
		n := len(res.Items())
		if n != 1 {
			ts.Errorf("expected 1 item got %d", n)
		}
	})

	t.Run("request_404", func(ts *testing.T) {
		res, err := client.Request("get", "/keys/ablyjs.test/requestToken", nil, nil, nil)
		if err != nil {
			ts.Fatal(err)
		}
		if res.StatusCode != http.StatusNotFound {
			ts.Errorf("expected %d got %d", http.StatusNotFound, res.StatusCode)
		}
		if res.ErrorCode != ably.ErrNotFound {
			ts.Errorf("expected %d got %d", ably.ErrNotFound, res.ErrorCode)
		}
		if res.Success {
			ts.Error("expected success to be false")
		}
		if res.ErrorMessage == "" {
			ts.Error("expected error message")
		}
	})

	t.Run("request_post_get_messages", func(ts *testing.T) {
		channelPath := "/channels/http-paginated-result/messages"
		msgOne := proto.Message{
			Name: "faye",
			Data: "whittaker",
		}

		ts.Run("post", func(ts *testing.T) {
			res, err := client.Request("POST", channelPath, nil, msgOne, nil)
			if err != nil {
				ts.Fatal(err)
			}
			if res.StatusCode != http.StatusCreated {
				ts.Errorf("expected %d got %d", http.StatusCreated, res.StatusCode)
			}
			if !res.Success {
				ts.Error("expected success to be true")
			}
			n := len(res.Items())
			if n != 1 {
				ts.Errorf("expected 1 item got %d", n)
			}
		})

	})
}
