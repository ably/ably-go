package ably_test

import (
	"net/url"
	"testing"

	"github.com/ably/ably-go/ably"
)

func TestClientOptions(t *testing.T) {
	t.Parallel()
	t.Run("must return error on invalid key", func(ts *testing.T) {
		_, err := ably.NewREST(ably.NewClientOptionsV12("invalid"))
		if err == nil {
			ts.Error("expected an error")
		}
	})
}

func TestScopeParams(t *testing.T) {
	t.Parallel()
	t.Run("must error when given invalid range", func(ts *testing.T) {
		params := ably.ScopeParams{
			Start: 123,
			End:   122,
		}
		err := params.EncodeValues(nil)
		if err == nil {
			ts.Fatal("expected an error")
		}
	})

	t.Run("must set url values", func(ts *testing.T) {
		params := ably.ScopeParams{
			Start: 122,
			End:   123,
		}
		u := make(url.Values)
		err := params.EncodeValues(&u)
		if err != nil {
			ts.Fatal(err)
		}
		start := u.Get("start")
		end := u.Get("end")
		if start != "122" {
			t.Errorf("expected 122 got %s", start)
		}
		if end != "123" {
			t.Errorf("expected 123 got %s", end)
		}
	})
}

func TestPaginateParams(t *testing.T) {
	t.Parallel()
	t.Run("returns nil with no values", func(ts *testing.T) {
		params := ably.PaginateParams{}
		values := make(url.Values)
		err := params.EncodeValues(&values)
		if err != nil {
			ts.Fatal(err)
		}
		encode := values.Encode()
		if encode != "" {
			t.Errorf("expected empty string got %s", encode)
		}
	})

	t.Run("returns the full params encoded", func(ts *testing.T) {
		params := ably.PaginateParams{
			Limit:     1,
			Direction: "backwards",
			ScopeParams: ably.ScopeParams{
				Start: 123,
				End:   124,
				Unit:  "hello",
			},
		}
		values := make(url.Values)
		err := params.EncodeValues(&values)
		if err != nil {
			ts.Fatal(err)
		}
		expect := "direction=backwards&end=124&limit=1&start=123&unit=hello"
		got := values.Encode()
		if got != expect {
			t.Errorf("expected %s got %s", expect, got)
		}
	})

	t.Run("with value", func(ts *testing.T) {
		params := ably.PaginateParams{
			Limit:     10,
			Direction: "backwards",
		}
		values := make(url.Values)
		err := params.EncodeValues(&values)
		if err != nil {
			ts.Fatal(err)
		}
	})

	t.Run("with a value for ScopeParams", func(ts *testing.T) {
		values := make(url.Values)
		params := ably.PaginateParams{}
		params.Start = 123
		err := params.EncodeValues(&values)
		if err != nil {
			ts.Fatal(err)
		}
		start := values.Get("start")
		if start != "123" {
			ts.Errorf("expected 123 got %s", start)
		}
	})
	t.Run("with invalid value for direction", func(ts *testing.T) {
		values := make(url.Values)
		params := ably.PaginateParams{}
		params.Direction = "unknown"
		err := params.EncodeValues(&values)
		if err == nil {
			ts.Fatal("expected an error")
		}
	})
	t.Run("with invalid value for limit", func(ts *testing.T) {
		values := make(url.Values)
		params := ably.PaginateParams{}
		params.Limit = -1
		err := params.EncodeValues(&values)
		if err != nil {
			ts.Fatal(err)
		}
		limit := values.Get("limit")
		if limit != "100" {
			t.Errorf("expected 100 got %s", limit)
		}
	})
}
