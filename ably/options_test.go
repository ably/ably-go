package ably_test

import (
	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
	"net/url"
	"reflect"
	"testing"
)

func TestDefaultOptions(t *testing.T) {
	t.Parallel()
	t.Run("with env should return environment fallback hosts", func(ts *testing.T) {
		expectedFallBackHosts := [] string {
				"sandbox-a-fallback.ably-realtime.com",
				"sandbox-b-fallback.ably-realtime.com",
				"sandbox-c-fallback.ably-realtime.com",
				"sandbox-d-fallback.ably-realtime.com",
				"sandbox-e-fallback.ably-realtime.com",
		};
		hosts := ably.GetEnvFallbackHosts("sandbox")
		assert.True(ts, reflect.DeepEqual(expectedFallBackHosts, hosts), " %s should match %s", hosts, expectedFallBackHosts)
	})
}

func TestClientOptions(t *testing.T) {
	t.Parallel()
	t.Run("parses it into a set of known parameters", func(ts *testing.T) {
		options := ably.NewClientOptions("name:secret")
		_, err := ably.NewRestClient(options)
		if err != nil {
			ts.Fatal(err)
		}
		key := "name"
		secret := "secret"
		if options.KeyName() != key {
			t.Errorf("expected %s got %s", key, options.KeyName())
		}
		if options.KeySecret() != secret {
			t.Errorf("expected %s got %s", secret, options.KeySecret())
		}
	})
	t.Run("must return error on invalid key", func(ts *testing.T) {
		_, err := ably.NewRestClient(ably.NewClientOptions("invalid"))
		if err == nil {
			ts.Error("expected an error")
		}
	})

	t.Run("with default options", func(ts *testing.T) {
		clientOptions := ably.NewClientOptions("")
		assert.Equal(ts, "realtime.ably.io", clientOptions.GetRealtimeHost())
		assert.Equal(ts, "rest.ably.io", clientOptions.GetRestHost())
		assert.False(ts, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(ts, 443, port)
		assert.True(ts, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Equal(ts, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("with production environment", func(ts *testing.T) {
		clientOptions := ably.NewClientOptions("")
		clientOptions.Environment = "production"
		assert.Equal(ts, "realtime.ably.io", clientOptions.GetRealtimeHost())
		assert.Equal(ts, "rest.ably.io", clientOptions.GetRestHost())
		assert.False(ts, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(ts, 443, port)
		assert.True(ts, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Equal(ts, ably.DefaultFallbackHosts(), fallbackHosts)
	})

	t.Run("with custom environment", func(ts *testing.T) {
		clientOptions := ably.NewClientOptions("")
		clientOptions.Environment = "sandbox"
		assert.Equal(ts, "sandbox-realtime.ably.io", clientOptions.GetRealtimeHost())
		assert.Equal(ts, "sandbox-rest.ably.io", clientOptions.GetRestHost())
		assert.False(ts, clientOptions.NoTLS)
		port, isDefaultPort := clientOptions.ActivePort()
		assert.Equal(ts, 443, port)
		assert.True(ts, isDefaultPort)
		fallbackHosts, _ := clientOptions.GetFallbackHosts()
		assert.Equal(ts, ably.GetEnvFallbackHosts("sandbox"), fallbackHosts)
	})

	t.Run("with custom environment and default fallbacks", func(ts *testing.T) {

	})

	t.Run("with custom environment and non default ports", func(ts *testing.T) {

	})

	t.Run("with custom host", func(ts *testing.T) {

	})

	t.Run("with custom rest host and realtime host", func(ts *testing.T) {

	})

	t.Run("with custom host and realtime host and fallbackHostsUseDefault", func(ts *testing.T) {

	})

	t.Run("with fallbackHosts and fallbackHostsUseDefault", func(ts *testing.T) {

	})

	t.Run("with fallbackHostsUseDefault And default custom port", func(ts *testing.T) {

	})

	t.Run("with custom host and realtime host", func(ts *testing.T) {

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
