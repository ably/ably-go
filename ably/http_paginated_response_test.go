package ably_test

import (
	"context"
	"net/http"
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
	t.Run("request_time", func(ts *testing.T) {
		res, err := client.Request(context.Background(), "get", "/time", nil, nil, nil)
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
		res, err := client.Request(context.Background(), "get", "/keys/ablyjs.test/requestToken", nil, nil, nil)
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
		channelName := "http-paginated-result"
		channelPath := "/channels/" + channelName + "/messages"
		msgs := []proto.Message{
			{Name: "faye", Data: "whittaker"},
			{Name: "martin", Data: "reed"},
		}

		ts.Run("post", func(ts *testing.T) {
			for _, message := range msgs {
				res, err := client.Request(context.Background(), "POST", channelPath, nil, message, nil)
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
			}
		})

		ts.Run("get", func(ts *testing.T) {
			res, err := client.Request(context.Background(), "get", channelPath, &ably.PaginateParams{
				Limit:     1,
				Direction: "forwards",
			}, nil, nil)
			if err != nil {
				ts.Fatal(err)
			}
			if res.StatusCode != http.StatusOK {
				ts.Errorf("expected %d got %d", http.StatusOK, res.StatusCode)
			}
			n := len(res.Items())
			if n != 1 {
				ts.Fatalf("expected 1 item got %d", n)
			}
			m := res.Items()[0].(map[string]interface{})
			name := m["name"].(string)
			data := m["data"].(string)
			if name != msgs[0].Name {
				ts.Errorf("expected %s got %s", msgs[0].Name, name)
			}
			if data != msgs[0].Data.(string) {
				ts.Errorf("expected %v got %s", msgs[0].Data, data)
			}

			res, err = res.Next(context.Background())
			if err != nil {
				ts.Fatal(err)
			}
			n = len(res.Items())
			if n != 1 {
				ts.Fatalf("expected 1 item got %d", n)
			}
			m = res.Items()[0].(map[string]interface{})
			name = m["name"].(string)
			data = m["data"].(string)
			if name != msgs[1].Name {
				ts.Errorf("expected %s got %s", msgs[1].Name, name)
			}
			if data != msgs[1].Data.(string) {
				ts.Errorf("expected %v got %s", msgs[1].Data, data)
			}

		})

		var m []*ably.Message
		err := ablytest.AllPages(&m, client.Channels.Get(channelName).History())
		if err != nil {
			ts.Fatal(err)
		}
		sort.Slice(m, func(i, j int) bool {
			return m[i].Name < m[j].Name
		})
		n := len(m)
		if n != 2 {
			ts.Fatalf("expected 2 item got %d", n)
		}
		if m[0].Name != msgs[0].Name {
			ts.Errorf("expected %s got %s", msgs[0].Name, m[0].Name)
		}
		if !reflect.DeepEqual(m[0].Data, msgs[0].Data) {
			ts.Errorf("expected %v got %s", msgs[0].Data, m[0].Data)
		}
	})
}
