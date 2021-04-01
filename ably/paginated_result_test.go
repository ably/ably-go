package ably_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ably/ably-go/ably"
)

func TestPaginatedResult(t *testing.T) {
	t.Parallel()
	result := &ably.PaginatedResult{}
	newPath := result.BuildPath("/path/to/resource?hello", "./newresource?world")
	expected := "/path/to/newresource?world"
	if newPath != expected {
		t.Errorf("expected %s got %s", expected, newPath)
	}
}

func TestMalformedPaginatedResult(t *testing.T) {
	bodyBytes, _ := json.Marshal([]string{"\x00 not really a PaginatedResult"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write(bodyBytes)
	}))
	defer srv.Close()

	srvAddr := srv.Listener.Addr().(*net.TCPAddr)
	client, err := ably.NewREST(
		ably.WithToken("xxxxxxx.yyyyyyy:zzzzzzz"),
		ably.WithTLS(false),
		ably.WithRESTHost(srvAddr.IP.String()),
		ably.WithPort(srvAddr.Port),
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Request("POST", "/foo").Pages(context.Background())
	if resp != nil {
		t.Errorf("expected no HTTPPaginatedResult; got %+v", resp)
	}
	if err == nil {
		t.Fatal("expected an error")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "status: 400") {
		t.Errorf("expected error to contain status code; got: %v", err)
	}
	if !strings.Contains(errMsg, fmt.Sprintf("%q", bodyBytes)) {
		t.Errorf("expected error to contain body; got: %v", err)
	}
}
