package ably_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ably/proto"
)

func newHTTPClientMock(srv *httptest.Server) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: func(*http.Request) (*url.URL, error) { return url.Parse(srv.URL) },
		},
	}
}

func TestRestClient(t *testing.T) {
	t.Parallel()
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	t.Run("encoding messages", func(ts *testing.T) {
		ts.Run("json", func(ts *testing.T) {
			var buffer []byte
			mockType := "application/json"
			mockBody := []byte("{}")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				buffer, err = ioutil.ReadAll(r.Body)
				if err != nil {
					ts.Fatal(err)
				}
				w.Header().Set("Content-Type", mockType)
				w.WriteHeader(200)
				w.Write(mockBody)
			}))
			options := &ably.ClientOptions{
				NoTLS:            true,
				NoBinaryProtocol: true,
				HTTPClient:       newHTTPClientMock(server),
				AuthOptions: ably.AuthOptions{
					UseTokenAuth: true,
				},
			}

			client, err := ably.NewRestClient(app.Options(options))
			if err != nil {
				ts.Fatal(err)
			}
			err = client.Channels.Get("test", nil).Publish("ping", "pong")
			if err != nil {
				ts.Fatal(err)
			}
			var anyJson []map[string]interface{}
			err = json.Unmarshal(buffer, &anyJson)
			if err != nil {
				ts.Error(err)
			}
		})
		ts.Run("msgpack", func(ts *testing.T) {
			var buffer []byte
			mockType := "application/x-msgpack"
			mockBody := []byte{0x80}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				buffer, err = ioutil.ReadAll(r.Body)
				if err != nil {
					ts.Fatal(err)
				}
				w.Header().Set("Content-Type", mockType)
				w.WriteHeader(200)
				w.Write(mockBody)
			}))
			options := &ably.ClientOptions{
				NoTLS:            true,
				NoBinaryProtocol: true,
				HTTPClient:       newHTTPClientMock(server),
				AuthOptions: ably.AuthOptions{
					UseTokenAuth: true,
				},
			}
			options = app.Options(options)
			options.NoBinaryProtocol = false
			client, err := ably.NewRestClient(options)
			if err != nil {
				ts.Fatal(err)
			}
			err = client.Channels.Get("test", nil).Publish("ping", "pong")
			if err != nil {
				ts.Fatal(err)
			}
			var anyMsgPack []map[string]interface{}
			err = ablyutil.Unmarshal(buffer, &anyMsgPack)
			if err != nil {
				ts.Fatal(err)
			}
			name := anyMsgPack[0]["name"].([]byte)
			data := anyMsgPack[0]["data"].([]byte)

			expectName := "ping"
			expectData := "pong"
			if string(name) != expectName {
				ts.Errorf("expected %s got %s", expectName, string(name))
			}
			if string(data) != expectData {
				ts.Errorf("expected %s got %s", expectData, string(data))
			}
		})
	})

	t.Run("Time", func(ts *testing.T) {
		client, err := ably.NewRestClient(app.Options())
		if err != nil {
			ts.Fatal(err)
		}
		t, err := client.Time()
		if err != nil {
			ts.Fatal(err)
		}
		before := time.Now().Add(2 * time.Second).Unix()
		after := time.Now().Add(-2 * time.Second).Unix()
		n := t.Unix()
		if n > before {
			ts.Errorf("expected %d <= %d", n, before)
		}
		if n < after {
			ts.Errorf("expected %d >= %d", n, before)
		}
	})

	t.Run("Stats", func(ts *testing.T) {
		client, err := ably.NewRestClient(app.Options())
		if err != nil {
			ts.Fatal(err)
		}
		lastInterval := time.Now().Add(-365 * 24 * time.Hour)
		var stats []*proto.Stats

		var jsonStats = `
		[
			{
				"inbound":{"realtime":{"messages":{"count":50,"data":5000}}},
				"outbound":{"realtime":{"messages":{"count":20,"data":2000}}}
			},
			{
				"inbound":{"realtime":{"messages":{"count":60,"data":6000}}},
				"outbound":{"realtime":{"messages":{"count":10,"data":1000}}}
			},
			{
				"inbound":{"realtime":{"messages":{"count":70,"data":7000}}},
				"outbound":{"realtime":{"messages":{"count":40,"data":4000}}},
				"persisted":{"presence":{"count":20,"data":2000}},
				"connections":{"tls":{"peak":20,"opened":10}},
				"channels":{"peak":50,"opened":30},
				"apiRequests":{"succeeded":50,"failed":10},
				"tokenRequests":{"succeeded":60,"failed":20}
			}
		]
	`
		err = json.Unmarshal([]byte(jsonStats), &stats)
		if err != nil {
			ts.Fatal(err)
		}
		stats[0].IntervalID = proto.IntervalFormatFor(lastInterval.Add(-120*time.Minute), proto.StatGranularityMinute)
		stats[1].IntervalID = proto.IntervalFormatFor(lastInterval.Add(-60*time.Minute), proto.StatGranularityMinute)
		stats[2].IntervalID = proto.IntervalFormatFor(lastInterval.Add(-1*time.Minute), proto.StatGranularityMinute)

		res, err := client.Post("/stats", &stats, nil)
		if err != nil {
			ts.Error(err)
		}
		res.Body.Close()

		longAgo := lastInterval.Add(-120 * time.Minute)
		page, err := client.Stats(&ably.PaginateParams{
			Limit: 1,
			ScopeParams: ably.ScopeParams{
				Start: ably.Time(longAgo),
				Unit:  proto.StatGranularityMinute,
			},
		})
		if err != nil {
			ts.Fatal(err)
		}
		re := regexp.MustCompile("[0-9]+\\-[0-9]+\\-[0-9]+:[0-9]+:[0-9]+")
		interval := page.Stats()[0].IntervalID
		if !re.MatchString(interval) {
			ts.Errorf("got %s which is wrong interval format", interval)
		}
	})
}

func TestRSC7(t *testing.T) {
	t.Parallel()
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	c, err := ably.NewRestClient(app.Options())
	if err != nil {
		t.Fatal(err)
	}
	t.Run("must set version header", func(ts *testing.T) {
		req, err := c.NewHTTPRequest(&ably.Request{})
		if err != nil {
			ts.Fatal(err)
		}
		h := req.Header.Get(ably.AblyVersionHeader)
		if h != ably.AblyVersion {
			t.Errorf("expected %s got %s", ably.AblyVersion, h)
		}
	})
	t.Run("must set lib header", func(ts *testing.T) {
		req, err := c.NewHTTPRequest(&ably.Request{})
		if err != nil {
			ts.Fatal(err)
		}
		h := req.Header.Get(ably.AblyLibHeader)
		if h != ably.LibraryString {
			t.Errorf("expected %s got %s", ably.LibraryString, h)
		}
	})
}

func TestRest_hostfallback(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	runTestServer := func(ts *testing.T, options *ably.ClientOptions) (int, []string) {
		var retryCount int
		var hosts []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hosts = append(hosts, r.Host)
			retryCount++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		options.HTTPClient = newHTTPClientMock(server)
		client, err := ably.NewRestClient(app.Options(options))
		if err != nil {
			ts.Fatal(err)
		}
		err = client.Channels.Get("test", nil).Publish("ping", "pong")
		if err == nil {
			ts.Error("expected an error")
		}
		return retryCount, hosts
	}
	t.Run("RSC15d RSC15a must use alternative host", func(ts *testing.T) {
		options := &ably.ClientOptions{
			NoTLS: true,
			AuthOptions: ably.AuthOptions{
				UseTokenAuth: true,
			},
		}
		retryCount, hosts := runTestServer(ts, options)
		if retryCount != 4 {
			ts.Fatalf("expected 4 http calls got %d", retryCount)
		}
		// make sure the host header is set. Since we are using defaults from the spec
		// the hosts should be in [a..e].ably-realtime.com
		expect := strings.Join(ably.DefaultFallbackHosts(), ", ")
		for _, host := range hosts[1:] {
			if !strings.Contains(expect, host) {
				ts.Errorf("expected %s got be in %s", host, expect)
			}
		}

		// ensure all picked fallbacks are unique
		uniq := make(map[string]bool)
		for _, h := range hosts {
			if _, ok := uniq[h]; ok {
				ts.Errorf("duplicate fallback %s", h)
			} else {
				uniq[h] = true
			}
		}
	})
	t.Run("rsc15b", func(ts *testing.T) {
		ts.Run("must not occur when default  rest.ably.io is overriden", func(ts *testing.T) {
			customHost := "example.com"
			options := &ably.ClientOptions{
				NoTLS:    true,
				RestHost: customHost,
				AuthOptions: ably.AuthOptions{
					UseTokenAuth: true,
				},
			}
			retryCount, hosts := runTestServer(ts, options)
			if retryCount != 1 {
				ts.Fatalf("expected 1 http call got %d", retryCount)
			}
			host := hosts[0]
			if host != customHost {
				ts.Errorf("expected %s got %s", customHost, host)
			}
		})
		ts.Run("must occur when fallbackHostsUseDefault is true", func(ts *testing.T) {
			customHost := "example.com"
			options := &ably.ClientOptions{
				NoTLS:                   true,
				RestHost:                customHost,
				FallbackHostsUseDefault: true,
				AuthOptions: ably.AuthOptions{
					UseTokenAuth: true,
				},
			}
			retryCount, hosts := runTestServer(ts, options)
			if retryCount != 4 {
				ts.Fatalf("expected 4 http call got %d", retryCount)
			}
			expect := strings.Join(ably.DefaultFallbackHosts(), ", ")
			for _, host := range hosts[1:] {
				if !strings.Contains(expect, host) {
					t.Errorf("expected %s got be in %s", host, expect)
				}
			}
		})
		ts.Run("must occur when fallbackHosts is set", func(ts *testing.T) {
			customHost := "example.com"
			fallback := "a.example.com"
			options := &ably.ClientOptions{
				NoTLS:         true,
				RestHost:      customHost,
				FallbackHosts: []string{fallback},
				AuthOptions: ably.AuthOptions{
					UseTokenAuth: true,
				},
			}
			retryCount, hosts := runTestServer(ts, options)
			if retryCount != 2 {
				ts.Fatalf("expected 2 http call got %d", retryCount)
			}
			host := hosts[1]
			if host != fallback {
				t.Errorf("expected %s got %s", fallback, host)
			}
		})
	})
	t.Run("RSC15e must start with default host", func(ts *testing.T) {
		options := &ably.ClientOptions{
			NoTLS: true,
			AuthOptions: ably.AuthOptions{
				UseTokenAuth: true,
			},
		}
		retryCount, hosts := runTestServer(ts, options)
		if retryCount != 4 {
			ts.Fatalf("expected 4 http calls got %d", retryCount)
		}
		firstHostCalled := hosts[0]
		if !strings.HasSuffix(firstHostCalled, ably.RestHost) {
			ts.Errorf("expected primary host got %s", firstHostCalled)
		}
	})
	t.Run("must not occur when FallbackHosts is an empty array", func(ts *testing.T) {
		customHost := "example.com"
		options := &ably.ClientOptions{
			NoTLS:         true,
			RestHost:      customHost,
			FallbackHosts: []string{},
			AuthOptions: ably.AuthOptions{
				UseTokenAuth: true,
			},
		}
		retryCount, _ := runTestServer(ts, options)
		if retryCount != 1 {
			ts.Fatalf("expected 1 http calls got %d", retryCount)
		}
	})
}

func TestRest_rememberHostFallback(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	t.Run("remember success host RSC15f", func(ts *testing.T) {
		var retryCount int
		var hosts []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hosts = append(hosts, r.Host)
			retryCount++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		fallbacks := ably.DefaultFallbackHosts()
		nopts := &ably.ClientOptions{
			NoTLS:                   true,
			FallbackHostsUseDefault: true,
			AuthOptions: ably.AuthOptions{
				UseTokenAuth: true,
			},
			HTTPClient: &http.Client{
				Transport: &http.Transport{
					Proxy: func(r *http.Request) (*url.URL, error) {
						if strings.HasPrefix(r.URL.Path, "/channels/") {
							if r.URL.Host == fallbacks[3] {
								return url.Parse(fmt.Sprintf("https://%s", hosts[0]))
							}
							return url.Parse(server.URL)
						}
						return r.URL, nil
					},
				},
			},
		}
		client, err := ably.NewRestClient(app.Options(nopts))
		if err != nil {
			ts.Fatal(err)
		}
		channel := client.Channels.Get("remember_fallback_host", nil)
		err = channel.Publish("ping", "pong")
		if err != nil {
			ts.Fatal(err)
		}
		cachedHost := client.GetCachedFallbackHost()
		if cachedHost != fallbacks[3] {
			ts.Errorf("expected cached host to be %s got %s", fallbacks[3], cachedHost)
		}
		retryCount = 0

		// the same cached host is used again
		err = channel.Publish("pong", "ping")
		if err != nil {
			ts.Fatal(err)
		}
		cachedHost = client.GetCachedFallbackHost()
		if cachedHost != fallbacks[3] {
			ts.Errorf("expected cached host to be %s got %s", fallbacks[3], cachedHost)
		}
		if retryCount != 0 {
			ts.Errorf("expected  0 retries got %d retries", retryCount)
		}
	})

	t.Run("configurable fallbackRetryTimeout", func(ts *testing.T) {
		ts.Run("defaults to 10 minutes", func(ts *testing.T) {
			opts := &ably.ClientOptions{}
			expect := 10 * time.Minute
			got := opts.GetFallbackRetryTimeout()
			if got != expect {
				ts.Errorf("expected %s got %s", expect, got)
			}
		})

		ts.Run("uses FallbackRetryTimeout if set", func(ts *testing.T) {
			expect := 10 * time.Second
			opts := &ably.ClientOptions{FallbackRetryTimeout: expect}
			got := opts.GetFallbackRetryTimeout()
			if got != expect {
				ts.Errorf("expected %s got %s", expect, got)
			}
		})
	})
}
func TestRestChannels_RSN1(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	client, err := ably.NewRestClient(app.Options())
	if err != nil {
		t.Fatal(err)
	}
	if client.Channels == nil {
		t.Errorf("expected Channels to be initialized")
	}
	sample := []struct {
		name string
	}{
		{name: "first_channel"},
		{name: "second_channel"},
		{name: "third_channel"},
	}

	t.Run("RSN3 RSN3a  must create new channels when they don't exist", func(ts *testing.T) {
		for _, v := range sample {
			client.Channels.Get(v.name, nil)
		}
		size := client.Channels.Len()
		if size != len(sample) {
			ts.Errorf("expected %d got %d", len(sample), size)
		}
	})
	t.Run("RSN4 RSN4a must release channels", func(ts *testing.T) {
		for _, v := range sample {
			ch := client.Channels.Get(v.name, nil)
			client.Channels.Release(ch)
		}
		size := client.Channels.Len()
		if size != 0 {
			ts.Errorf("expected 0 channels  got %d", size)
		}
	})
	t.Run("ensure no deadlock in Range", func(ts *testing.T) {
		for _, v := range sample {
			client.Channels.Get(v.name, nil)
		}
		client.Channels.Range(func(name string, _ *ably.RestChannel) bool {
			n := client.Channels.Get(name+"_range", nil)
			return client.Channels.Exists(n.Name)
		})
	})
}

func TestFixConnLeak_ISSUE89(t *testing.T) {
	var trackRecord []httptrace.GotConnInfo
	trace := &httptrace.ClientTrace{
		GotConn: func(c httptrace.GotConnInfo) {
			trackRecord = append(trackRecord, c)
		},
	}
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	opts := app.Options()
	opts.HTTPClient = ablytest.NewHTTPClientNoKeepAlive()
	opts.Trace = trace
	client, err := ably.NewRestClient(opts)
	if err != nil {
		t.Fatal(err)
	}
	channel := client.Channels.Get("issue89", nil)
	for i := 0; i < 10; i++ {
		err := channel.Publish(fmt.Sprintf("msg_%d", i), fmt.Sprint(i))
		if err != nil {
			t.Error(err)
		}
	}

	for _, v := range trackRecord {
		v.Conn.SetReadDeadline(time.Now())
		if _, err := v.Conn.Read(make([]byte, 1)); err != nil {
			if !connIsClosed(err) {
				t.Errorf("expected conn %s to be closed", v.Conn.LocalAddr())
			}
		}
	}
}

func connIsClosed(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection")
}
