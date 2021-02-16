package ably_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
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
			options := []ably.ClientOption{
				ably.WithTLS(false),
				ably.WithUseTokenAuth(true),
				ably.WithUseBinaryProtocol(false),
				ably.WithHTTPClient(newHTTPClientMock(server)),
			}

			client, err := ably.NewREST(app.Options(options...)...)
			if err != nil {
				ts.Fatal(err)
			}
			err = client.Channels.Get("test").Publish(context.Background(), "ping", "pong")
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
			options := []ably.ClientOption{
				ably.WithTLS(false),
				ably.WithUseTokenAuth(true),
				ably.WithUseBinaryProtocol(true),
				ably.WithHTTPClient(newHTTPClientMock(server)),
			}

			client, err := ably.NewREST(app.Options(options...)...)
			if err != nil {
				ts.Fatal(err)
			}
			err = client.Channels.Get("test").Publish(context.Background(), "ping", "pong")
			if err != nil {
				ts.Fatal(err)
			}
			var anyMsgPack []map[string]interface{}
			err = ablyutil.UnmarshalMsgpack(buffer, &anyMsgPack)
			if err != nil {
				ts.Fatal(err)
			}
			name := anyMsgPack[0]["name"].(string)
			data := anyMsgPack[0]["data"].(string)

			expectName := "ping"
			expectData := "pong"
			if name != expectName {
				ts.Errorf("expected %s got %s", expectName, string(name))
			}
			if data != expectData {
				ts.Errorf("expected %s got %s", expectData, string(data))
			}
		})
	})

	t.Run("Time", func(ts *testing.T) {
		client, err := ably.NewREST(app.Options()...)
		if err != nil {
			ts.Fatal(err)
		}
		t, err := client.Time(context.Background())
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
		client, err := ably.NewREST(app.Options()...)
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

		res, err := client.Post(context.Background(), "/stats", &stats, nil)
		if err != nil {
			ts.Error(err)
		}
		res.Body.Close()

		statsCh := make(chan []*proto.Stats, 1)
		errCh := make(chan error, 1)

		go func() {
			longAgo := lastInterval.Add(-120 * time.Minute)

			timeout := time.After(time.Second * 10)
			tick := time.Tick(time.Millisecond * 500)

			// keep trying until we get a pagination result, error, or timeout
			for {
				select {
				case <-timeout:
					errCh <- errors.New("timeout waiting for client.Stats to return nonempty value")
					return
				case <-tick:
					page, err := client.Stats(
						ably.StatsWithLimit(1),
						ably.StatsWithStart(longAgo),
						ably.StatsWithUnit(ably.PeriodMinute),
					).Pages(context.Background())
					if err != nil {
						errCh <- err
						return
					}
					if !page.Next(context.Background()) {
						errCh <- page.Err()
						return
					}
					if stats := page.Items(); len(stats) != 0 {
						statsCh <- stats
						return
					}
				}
			}
		}()

		select {
		case pageStats := <-statsCh:
			re := regexp.MustCompile("[0-9]+\\-[0-9]+\\-[0-9]+:[0-9]+:[0-9]+")
			interval := pageStats[0].IntervalID
			if !re.MatchString(interval) {
				ts.Errorf("got %s which is wrong interval format", interval)
			}
		case err := <-errCh:
			ts.Fatal(err)
		}
	})
}

type httpRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f httpRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRSC7(t *testing.T) {
	t.Parallel()

	client := &http.Client{}
	requests := make(chan *http.Request, 1)
	client.Transport = httpRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requests <- req
		return nil, errors.New("fake round tripper")
	})

	c, err := ably.NewREST(
		ably.WithKey("fake:key"),
		ably.WithHTTPClient(client))
	if err != nil {
		t.Fatal(err)
	}

	_, _ = c.Request(context.Background(), "POST", "/foo", nil, nil, nil)

	var req *http.Request
	ablytest.Instantly.Recv(t, &req, requests, t.Fatalf)

	t.Run("must set version header", func(ts *testing.T) {
		h := req.Header.Get(proto.AblyVersionHeader)
		if h != proto.AblyVersion {
			t.Errorf("expected %s got %s", proto.AblyVersion, h)
		}
	})
	t.Run("must set lib header", func(ts *testing.T) {
		h := req.Header.Get(proto.AblyLibHeader)
		if h != proto.LibraryString {
			t.Errorf("expected %s got %s", proto.LibraryString, h)
		}
	})
}

func TestRest_hostfallback(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	runTestServer := func(ts *testing.T, options []ably.ClientOption) (int, []string) {
		var retryCount int
		var hosts []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hosts = append(hosts, r.Host)
			retryCount++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		client, err := ably.NewREST(app.Options(append(options, ably.WithHTTPClient(newHTTPClientMock(server)))...)...)
		if err != nil {
			ts.Fatal(err)
		}
		err = client.Channels.Get("test").Publish(context.Background(), "ping", "pong")
		if err == nil {
			ts.Error("expected an error")
		}
		return retryCount, hosts
	}
	t.Run("RSC15d RSC15a must use alternative host", func(ts *testing.T) {
		options := []ably.ClientOption{
			ably.WithFallbackHosts(ably.DefaultFallbackHosts()),
			ably.WithTLS(false),
			ably.WithUseTokenAuth(true),
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
			options := []ably.ClientOption{
				ably.WithTLS(false),
				ably.WithRESTHost(customHost),
				ably.WithUseTokenAuth(true),
			}
			retryCount, hosts := runTestServer(ts, options)
			if retryCount != 1 {
				ts.Fatalf("expected 1 http call got %d", retryCount)
			}
			host := hosts[0]
			if !strings.Contains(host, customHost) {
				ts.Errorf("expected %s got %s", customHost, host)
			}
		})
		ts.Run("must occur when fallbackHostsUseDefault is true", func(ts *testing.T) {
			customHost := "example.com"
			options := []ably.ClientOption{
				ably.WithTLS(false),
				ably.WithRESTHost(customHost),
				ably.WithFallbackHosts(ably.DefaultFallbackHosts()),
				ably.WithUseTokenAuth(true),
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
			options := []ably.ClientOption{
				ably.WithTLS(false),
				ably.WithRESTHost(customHost),
				ably.WithFallbackHosts([]string{fallback}),
				ably.WithUseTokenAuth(true),
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
		options := []ably.ClientOption{
			ably.WithEnvironment("production"),
			ably.WithTLS(false),
			ably.WithUseTokenAuth(true),
		}
		retryCount, hosts := runTestServer(ts, options)
		if retryCount != 4 {
			ts.Fatalf("expected 4 http calls got %d", retryCount)
		}
		firstHostCalled := hosts[0]
		restURL, _ := url.Parse(ably.ApplyOptionsWithDefaults(options...).RestURL())
		if !strings.HasPrefix(firstHostCalled, restURL.Hostname()) {
			ts.Errorf("expected primary host got %s", firstHostCalled)
		}
	})
	t.Run("must not occur when FallbackHosts is an empty array", func(ts *testing.T) {
		customHost := "example.com"
		options := []ably.ClientOption{
			ably.WithTLS(false),
			ably.WithRESTHost(customHost),
			ably.WithFallbackHosts([]string{}),
			ably.WithUseTokenAuth(true),
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

	fallbackHosts := []string{"fallback0", "fallback1", "fallback2"}
	var nopts []ably.ClientOption

	t.Run("remember success host RSC15f", func(ts *testing.T) {
		var retryCount int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			retryCount++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		nopts = []ably.ClientOption{
			ably.WithEnvironment(ablytest.Environment),
			ably.WithTLS(false),
			ably.WithFallbackHosts(fallbackHosts),
			ably.WithUseTokenAuth(true),
		}

		// set up the proxy to forward all requests except a specific fallback to the server,
		// whilst that fallback goes to the regular endpoint
		serverURL, _ := url.Parse(server.URL)
		defaultURL, _ := url.Parse(ably.ApplyOptionsWithDefaults(nopts...).RestURL())

		proxy := func(r *http.Request) (*url.URL, error) {
			if r.URL.Hostname() == "fallback2" {
				// set the Host in the request to the intended destination
				r.Host = defaultURL.Hostname()
				return defaultURL, nil
			} else {
				return serverURL, nil
			}
		}
		nopts = append(nopts, ably.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				Proxy:        proxy,
				TLSNextProto: map[string]func(authority string, c *tls.Conn) http.RoundTripper{},
			},
		}))

		client, err := ably.NewREST(app.Options(nopts...)...)
		if err != nil {
			ts.Fatal(err)
		}
		channel := client.Channels.Get("remember_fallback_host")
		err = channel.Publish(context.Background(), "ping", "pong")
		if err != nil {
			ts.Fatal(err)
		}
		cachedHost := client.GetCachedFallbackHost()
		if cachedHost != fallbackHosts[2] {
			ts.Errorf("expected cached host to be %s got %s", fallbackHosts[2], cachedHost)
		}
		retryCount = 0

		// the same cached host is used again
		err = channel.Publish(context.Background(), "pong", "ping")
		if err != nil {
			ts.Fatal(err)
		}
		cachedHost = client.GetCachedFallbackHost()
		if cachedHost != fallbackHosts[2] {
			ts.Errorf("expected cached host to be %s got %s", fallbackHosts[2], cachedHost)
		}
		if retryCount != 0 {
			ts.Errorf("expected 0 retries got %d retries", retryCount)
		}
	})
}
func TestRESTChannels_RSN1(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	client, err := ably.NewREST(app.Options()...)
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
			client.Channels.Get(v.name)
		}
		size := client.Channels.Len()
		if size != len(sample) {
			ts.Errorf("expected %d got %d", len(sample), size)
		}
	})
	t.Run("RSN4 RSN4a must release channels", func(ts *testing.T) {
		for _, v := range sample {
			ch := client.Channels.Get(v.name)
			client.Channels.Release(ch.Name)
		}
		size := client.Channels.Len()
		if size != 0 {
			ts.Errorf("expected 0 channels  got %d", size)
		}
	})
	t.Run("ensure no deadlock in Range", func(ts *testing.T) {
		for _, v := range sample {
			client.Channels.Get(v.name)
		}
		client.Channels.Range(func(name string, _ *ably.RESTChannel) bool {
			n := client.Channels.Get(name + "_range")
			return client.Channels.Exists(n.Name)
		})
	})
}

func TestFixConnLeak_ISSUE89(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	var conns []*connCloseTracker

	httpClient := ablytest.NewHTTPClientNoKeepAlive()
	transport := httpClient.Transport.(*http.Transport)
	dial := transport.Dial
	transport.Dial = func(network, address string) (net.Conn, error) {
		c, err := dial(network, address)
		if err != nil {
			return nil, err
		}
		tracked := &connCloseTracker{Conn: c}
		conns = append(conns, tracked)
		return tracked, nil
	}

	opts := app.Options(ably.WithHTTPClient(httpClient))
	client, err := ably.NewREST(opts...)
	if err != nil {
		t.Fatal(err)
	}
	channel := client.Channels.Get("issue89")
	for i := 0; i < 10; i++ {
		err := channel.Publish(context.Background(), fmt.Sprintf("msg_%d", i), fmt.Sprint(i))
		if err != nil {
			t.Error(err)
		}
	}

	for _, c := range conns {
		if !ablytest.Before(1 * time.Second).IsTrue(func() bool {
			return atomic.LoadUintptr(&c.closed) != 0
		}) {
			t.Errorf("conn to %v wasn't closed", c.RemoteAddr())
		}
	}
}

type connCloseTracker struct {
	net.Conn
	closed uintptr
}

func (c *connCloseTracker) Close() error {
	atomic.StoreUintptr(&c.closed, 1)
	return c.Conn.Close()
}
