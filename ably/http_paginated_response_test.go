package ably_test

import (
	"net/http"
	"testing"

	"github.com/ably/ably-go/ably"

	"github.com/ably/ably-go/ably/ablytest"
)

func TestHTTPPaginatedResponse(t *testing.T) {
	t.Parallel()
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	opts := app.Options()
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
}
