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
}
