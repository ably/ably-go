package ably_test

import (
	"testing"

	"github.com/ably/ably-go/ably"
)

func TestCheckValidHTTPResponse(t *testing.T) {
	opts := ably.NewClientOptions(":")
	_, e := ably.NewRestClient(opts)
	if e == nil {
		t.Fatal("NewRestClient(): expected err != nil")
	}
	err, ok := e.(*ably.Error)
	if !ok {
		t.Fatalf("want e be *ably.Error; was %T", e)
	}
	if err.StatusCode != 400 {
		t.Errorf("want StatusCode=400; got %d", err.StatusCode)
	}
	if err.Code != 40005 {
		t.Errorf("want Code=40005; got %d", err.Code)
	}
	if err.Err == nil {
		t.Error("want Err to be non-nil")
	}
}
