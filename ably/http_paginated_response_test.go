package ably_test

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"sort"
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
	opts := app.Options(ably.WithUseBinaryProtocol(false))
	client, err := ably.NewREST(opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("request_time", func(t *testing.T) {
		t.Parallel()

		res, err := client.Request("get", "/time").Pages(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode() != http.StatusOK {
			t.Errorf("expected %d got %d", http.StatusOK, res.StatusCode())
		}
		if !res.Success() {
			t.Error("expected success to be true")
		}
		res.Next(context.Background())
		var items []interface{}
		err = res.Items(&items)
		if err != nil {
			t.Error(err)
		}
		n := len(items)
		if n != 1 {
			t.Errorf("expected 1 item got %d", n)
		}
	})

	t.Run("request_404", func(t *testing.T) {

		res, err := client.Request("get", "/keys/ablyjs.test/requestToken").Pages(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode() != http.StatusNotFound {
			t.Errorf("expected %d got %d", http.StatusNotFound, res.StatusCode())
		}
		if res.ErrorCode() != ably.ErrNotFound {
			t.Errorf("expected %d got %d", ably.ErrNotFound, res.ErrorCode())
		}
		if res.Success() {
			t.Error("expected success to be false")
		}
		if res.ErrorMessage() == "" {
			t.Error("expected error message")
		}
	})

	t.Run("request_post_get_messages", func(t *testing.T) {
		channelName := "http-paginated-result"
		channelPath := "/channels/" + channelName + "/messages"
		msgs := []proto.Message{
			{Name: "faye", Data: "whittaker"},
			{Name: "martin", Data: "reed"},
		}

		t.Run("post", func(t *testing.T) {
			for _, message := range msgs {
				res, err := client.Request("POST", channelPath, ably.RequestWithBody(message)).Pages(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				if res.StatusCode() != http.StatusCreated {
					t.Errorf("expected %d got %d", http.StatusCreated, res.StatusCode())
				}
				if !res.Success() {
					t.Error("expected success to be true")
				}
				res.Next(context.Background())
				var items []interface{}
				err = res.Items(&items)
				if err != nil {
					t.Error(err)
				}
				n := len(items)
				if n != 1 {
					t.Errorf("expected 1 item got %d", n)
				}
			}
		})

		t.Run("get", func(t *testing.T) {
			res, err := client.Request("get", channelPath, ably.RequestWithParams(url.Values{
				"limit":     {"1"},
				"direction": {"forwards"},
			})).Pages(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if res.StatusCode() != http.StatusOK {
				t.Errorf("expected %d got %d", http.StatusOK, res.StatusCode())
			}
			res.Next(context.Background())
			var items []interface{}
			err = res.Items(&items)
			if err != nil {
				t.Error(err)
			}
			n := len(items)
			if n != 1 {
				t.Fatalf("expected 1 item got %d", n)
			}
			m := items[0].(map[string]interface{})
			name := m["name"].(string)
			data := m["data"].(string)
			if name != msgs[0].Name {
				t.Errorf("expected %s got %s", msgs[0].Name, name)
			}
			if data != msgs[0].Data.(string) {
				t.Errorf("expected %v got %s", msgs[0].Data, data)
			}

			if !res.Next(context.Background()) {
				t.Fatal(res.Err())
			}
			err = res.Items(&items)
			if err != nil {
				t.Error(err)
			}
			n = len(items)
			if n != 1 {
				t.Fatalf("expected 1 item got %d", n)
			}
			m = items[0].(map[string]interface{})
			name = m["name"].(string)
			data = m["data"].(string)
			if name != msgs[1].Name {
				t.Errorf("expected %s got %s", msgs[1].Name, name)
			}
			if data != msgs[1].Data.(string) {
				t.Errorf("expected %v got %s", msgs[1].Data, data)
			}

		})

		var m []*ably.Message
		err := ablytest.AllPages(&m, client.Channels.Get(channelName).History())
		if err != nil {
			t.Fatal(err)
		}
		sort.Slice(m, func(i, j int) bool {
			return m[i].Name < m[j].Name
		})
		n := len(m)
		if n != 2 {
			t.Fatalf("expected 2 item got %d", n)
		}
		if m[0].Name != msgs[0].Name {
			t.Errorf("expected %s got %s", msgs[0].Name, m[0].Name)
		}
		if !reflect.DeepEqual(m[0].Data, msgs[0].Data) {
			t.Errorf("expected %v got %s", msgs[0].Data, m[0].Data)
		}
	})
}
