//go:build !unit
// +build !unit

package ably_test

import (
	"bytes"
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
	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ablytest"

	"github.com/stretchr/testify/assert"
)

func newHTTPClientMock(srv *httptest.Server) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: func(*http.Request) (*url.URL, error) { return url.Parse(srv.URL) },
		},
	}
}

func TestRestClient(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	assert.NoError(t, err)
	defer app.Close()
	t.Run("encoding messages", func(t *testing.T) {
		t.Run("json", func(t *testing.T) {
			var buffer []byte
			mockType := "application/json"
			mockBody := []byte("{}")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				buffer, err = ioutil.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
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
			assert.NoError(t, err)
			err = client.Channels.Get("test").Publish(context.Background(), "ping", "pong")
			assert.NoError(t, err)
			var anyJson []map[string]interface{}
			err = json.Unmarshal(buffer, &anyJson)
			assert.NoError(t, err)
		})
		t.Run("msgpack", func(t *testing.T) {
			var buffer []byte
			mockType := "application/x-msgpack"
			mockBody := []byte{0x80}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				buffer, err = ioutil.ReadAll(r.Body)
				assert.NoError(t, err)
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
			assert.NoError(t, err)
			err = client.Channels.Get("test").Publish(context.Background(), "ping", "pong")
			assert.NoError(t, err)
			var anyMsgPack []map[string]interface{}
			err = ablyutil.UnmarshalMsgpack(buffer, &anyMsgPack)
			assert.NoError(t, err)
			name := anyMsgPack[0]["name"].(string)
			data := anyMsgPack[0]["data"].(string)
			assert.Equal(t, "ping", name,
				"expected \"ping\" got %s", name)
			assert.Equal(t, "pong", data,
				"expected \"ping\" got %s", data)
		})
	})

	t.Run("Time", func(t *testing.T) {
		client, err := ably.NewREST(app.Options()...)
		assert.NoError(t, err)
		ti, err := client.Time(context.Background())
		assert.NoError(t, err)
		before := time.Now().Add(2 * time.Second).Unix()
		after := time.Now().Add(-2 * time.Second).Unix()
		n := ti.Unix()
		assert.LessOrEqual(t, n, before,
			"expected %d <= %d", n, before)
		assert.GreaterOrEqual(t, n, after,
			"expected %d >= %d", n, after)
	})

	t.Run("Stats", func(t *testing.T) {
		client, err := ably.NewREST(app.Options()...)
		assert.NoError(t, err)
		lastInterval := time.Now().Add(-365 * 24 * time.Hour)
		var stats []*ably.Stats

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
		assert.NoError(t, err)
		stats[0].IntervalID = intervalFormatFor(lastInterval.Add(-120*time.Minute), ably.StatGranularityMinute)
		stats[1].IntervalID = intervalFormatFor(lastInterval.Add(-60*time.Minute), ably.StatGranularityMinute)
		stats[2].IntervalID = intervalFormatFor(lastInterval.Add(-1*time.Minute), ably.StatGranularityMinute)

		res, err := client.Post(context.Background(), "/stats", &stats, nil)
		assert.NoError(t, err)
		res.Body.Close()

		statsCh := make(chan []*ably.Stats, 1)
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
			re := regexp.MustCompile(`[0-9]+\-[0-9]+\-[0-9]+:[0-9]+:[0-9]+`)
			interval := pageStats[0].IntervalID
			assert.True(t, re.MatchString(interval),
				"got %s which is wrong interval format", interval)
		case err := <-errCh:
			t.Fatal(err)
		}
	})
}

type httpRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f httpRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRSC7(t *testing.T) {

	client := &http.Client{}
	requests := make(chan *http.Request, 1)
	client.Transport = httpRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requests <- req
		return nil, errors.New("fake round tripper")
	})

	c, err := ably.NewREST(
		ably.WithKey("fake:key"),
		ably.WithHTTPClient(client))
	assert.NoError(t, err)

	_, _ = c.Request("POST", "/foo").Pages(context.Background())

	var req *http.Request
	ablytest.Instantly.Recv(t, &req, requests, t.Fatalf)

	t.Run("must set version header", func(t *testing.T) {
		h := req.Header.Get(ably.AblyProtocolVersionHeader)
		assert.Equal(t, ably.AblyProtocolVersion, h,
			"expected %s got %s", ably.AblyProtocolVersion, h)
	})
}

func TestRest_RSC7_AblyAgent(t *testing.T) {
	t.Run("RSC7d2 : Should set ablyAgent header with correct identifiers", func(t *testing.T) {
		var agentHeaderValue string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			agentHeaderValue = r.Header.Get(ably.AblyAgentHeader)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		serverURL, err := url.Parse(server.URL)
		assert.NoError(t, err)

		opts := []ably.ClientOption{
			ably.WithEnvironment(ablytest.Environment),
			ably.WithTLS(false),
			ably.WithUseTokenAuth(true),
			ably.WithRESTHost(serverURL.Host),
		}

		client, err := ably.NewREST(opts...)
		assert.NoError(t, err)
		expectedAgentHeaderValue := ably.AblySDKIdentifier + " " + ably.GoRuntimeIdentifier + " " + ably.GoOSIdentifier()

		client.Time(context.Background())
		assert.Equal(t, expectedAgentHeaderValue, agentHeaderValue)
	})

	t.Run("RSC7d6 : Should set ablyAgent header with custom agents", func(t *testing.T) {
		var agentHeaderValue string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			agentHeaderValue = r.Header.Get(ably.AblyAgentHeader)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		serverURL, err := url.Parse(server.URL)
		assert.NoError(t, err)

		opts := []ably.ClientOption{
			ably.WithEnvironment(ablytest.Environment),
			ably.WithTLS(false),
			ably.WithUseTokenAuth(true),
			ably.WithRESTHost(serverURL.Host),
			ably.WithAgents(map[string]string{
				"foo": "1.2.3",
			}),
		}

		client, err := ably.NewREST(opts...)
		assert.NoError(t, err)
		expectedAgentHeaderValue := ably.AblySDKIdentifier + " " + ably.GoRuntimeIdentifier + " " + ably.GoOSIdentifier() + " foo/1.2.3"

		client.Time(context.Background())
		assert.Equal(t, expectedAgentHeaderValue, agentHeaderValue)
	})

	t.Run("RSC7d6 : Should set ablyAgent header with custom agents missing version", func(t *testing.T) {
		var agentHeaderValue string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			agentHeaderValue = r.Header.Get(ably.AblyAgentHeader)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		serverURL, err := url.Parse(server.URL)
		assert.NoError(t, err)

		opts := []ably.ClientOption{
			ably.WithEnvironment(ablytest.Environment),
			ably.WithTLS(false),
			ably.WithUseTokenAuth(true),
			ably.WithRESTHost(serverURL.Host),
			ably.WithAgents(map[string]string{
				"bar": "",
			}),
		}

		client, err := ably.NewREST(opts...)
		assert.NoError(t, err)
		expectedAgentHeaderValue := ably.AblySDKIdentifier + " " + ably.GoRuntimeIdentifier + " " + ably.GoOSIdentifier() + " bar"

		client.Time(context.Background())
		assert.Equal(t, expectedAgentHeaderValue, agentHeaderValue)
	})
}

func TestRest_RSC15_HostFallback(t *testing.T) {

	app, err := ablytest.NewSandbox(nil)
	assert.NoError(t, err)
	defer app.Close()
	runTestServer := func(t *testing.T, options []ably.ClientOption) (int, []string) {
		var retryCount int
		var hosts []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hosts = append(hosts, strings.Split(r.Host, ":")[0])
			retryCount++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		client, err := ably.NewREST(app.Options(append(options, ably.WithHTTPClient(newHTTPClientMock(server)))...)...)
		assert.NoError(t, err)
		err = client.Channels.Get("test").Publish(context.Background(), "ping", "pong")
		assert.Error(t, err, "expected an error")
		return retryCount, hosts
	}

	t.Run("RSC15a, RSC15b, RSC15d, RSC15g3: must use alternative host", func(t *testing.T) {
		options := []ably.ClientOption{
			ably.WithFallbackHosts(ably.DefaultFallbackHosts()),
			ably.WithTLS(false),
			ably.WithEnvironment(""), // remove default sandbox env
			ably.WithHTTPMaxRetryCount(10),
			ably.WithUseTokenAuth(true),
		}
		retryCount, hosts := runTestServer(t, options)
		assert.Equal(t, 6, retryCount)                          // 1 primary and 5 default fallback hosts
		assert.Equal(t, "rest.ably.io", hosts[0])               // primary host
		assertSubset(t, ably.DefaultFallbackHosts(), hosts[1:]) // remaining fallback hosts
		assertUnique(t, hosts)                                  // ensure all picked fallbacks are unique
	})

	runTestServerWithRequestTimeout := func(t *testing.T, options []ably.ClientOption) (int, []string) {
		var retryCount int
		var hosts []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hosts = append(hosts, strings.Split(r.Host, ":")[0])
			retryCount++
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		httpClientMock := &http.Client{
			Timeout: 1 * time.Second,
			Transport: &http.Transport{
				Proxy: func(*http.Request) (*url.URL, error) { return url.Parse(server.URL) },
			},
		}
		client, err := ably.NewREST(app.Options(append(options, ably.WithHTTPClient(httpClientMock))...)...)
		assert.NoError(t, err)
		err1 := client.Channels.Get("test").Publish(context.Background(), "ping", "pong")
		assert.Contains(t, err1.Error(), "context deadline exceeded (Client.Timeout exceeded while awaiting headers)")
		return retryCount, hosts
	}

	t.Run("RSC15l2 must use alternative host on timeout", func(t *testing.T) {

		options := []ably.ClientOption{
			ably.WithFallbackHosts(ably.DefaultFallbackHosts()),
			ably.WithTLS(false),
			ably.WithEnvironment(""), // remove default sandbox env
			ably.WithHTTPMaxRetryCount(10),
			ably.WithUseTokenAuth(true),
		}
		retryCount, hosts := runTestServerWithRequestTimeout(t, options)
		assert.Equal(t, 6, retryCount)                          // 1 primary and 5 default fallback hosts
		assert.Equal(t, "rest.ably.io", hosts[0])               // primary host
		assertSubset(t, ably.DefaultFallbackHosts(), hosts[1:]) // remaining fallback hosts
		assertUnique(t, hosts)                                  // ensure all picked fallbacks are unique
	})

	t.Run("rsc15b", func(t *testing.T) {
		t.Run("must not occur when default rest.ably.io is overridden", func(t *testing.T) {

			options := []ably.ClientOption{
				ably.WithTLS(false),
				ably.WithRESTHost("example.com"),
				ably.WithUseTokenAuth(true),
			}
			retryCount, hosts := runTestServer(t, options)
			assert.Equal(t, 1, retryCount,
				"expected 1 http call got %d", retryCount)
			host := hosts[0]
			assert.Contains(t, host, "example.com",
				"expected host to contain \"example.com\" got %s", host)
		})
		t.Run("must occur when fallbackHostsUseDefault is true", func(t *testing.T) {

			options := []ably.ClientOption{
				ably.WithTLS(false),
				ably.WithRESTHost("example.com"),
				ably.WithFallbackHosts(ably.DefaultFallbackHosts()),
				ably.WithUseTokenAuth(true),
			}
			retryCount, hosts := runTestServer(t, options)
			assert.Equal(t, 4, retryCount,
				"expected 4 http call got %d", retryCount)
			expect := strings.Join(ably.DefaultFallbackHosts(), ", ")
			for _, host := range hosts[1:] {
				assert.Contains(t, expect, host,
					"expected %s to contain %s", expect, host)
			}
		})
		t.Run("must occur when fallbackHosts is set", func(t *testing.T) {
			options := []ably.ClientOption{
				ably.WithTLS(false),
				ably.WithRESTHost("example.com"),
				ably.WithFallbackHosts([]string{"a.example.com"}),
				ably.WithUseTokenAuth(true),
			}
			retryCount, hosts := runTestServer(t, options)
			if retryCount != 2 {
				t.Fatalf("expected 2 http call got %d", retryCount)
			}
			host := hosts[1]
			assert.Equal(t, "a.example.com", host,
				"expected \"a.example.com\" got %s", host)
		})
	})
	t.Run("RSC15e must start with default host", func(t *testing.T) {

		options := []ably.ClientOption{
			ably.WithEnvironment("production"),
			ably.WithTLS(false),
			ably.WithUseTokenAuth(true),
		}
		retryCount, hosts := runTestServer(t, options)
		assert.Equal(t, 4, retryCount,
			"expected 4 http calls got %d", retryCount)
		firstHostCalled := hosts[0]
		restURL, _ := url.Parse(ably.ApplyOptionsWithDefaults(options...).RestURL())
		assert.True(t, strings.HasPrefix(firstHostCalled, restURL.Hostname()),
			"expected primary host got %s", firstHostCalled)
	})
	t.Run("must not occur when FallbackHosts is an empty array", func(t *testing.T) {

		options := []ably.ClientOption{
			ably.WithTLS(false),
			ably.WithRESTHost("example.com"),
			ably.WithFallbackHosts([]string{}),
			ably.WithUseTokenAuth(true),
		}
		retryCount, _ := runTestServer(t, options)
		assert.Equal(t, 1, retryCount,
			"expected 1 http calls got %d", retryCount)
	})
}

func TestRest_rememberHostFallback(t *testing.T) {

	app, err := ablytest.NewSandbox(nil)
	assert.NoError(t, err)
	defer app.Close()

	var nopts []ably.ClientOption

	t.Run("remember success host RSC15f", func(t *testing.T) {
		var retryCount int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			retryCount++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		nopts = []ably.ClientOption{
			ably.WithEnvironment(ablytest.Environment),
			ably.WithTLS(false),
			ably.WithFallbackHosts([]string{"fallback0", "fallback1", "fallback2"}),
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
		assert.NoError(t, err)
		channel := client.Channels.Get("remember_fallback_host")
		err = channel.Publish(context.Background(), "ping", "pong")
		assert.NoError(t, err)
		cachedHost := client.GetCachedFallbackHost()
		assert.Equal(t, "fallback2", cachedHost,
			"expected cached host to be \"fallback2\" got %s", cachedHost)
		retryCount = 0

		// the same cached host is used again
		err = channel.Publish(context.Background(), "pong", "ping")
		assert.NoError(t, err)
		cachedHost = client.GetCachedFallbackHost()
		assert.Equal(t, "fallback2", cachedHost,
			"expected cached host to be \"fallback2\" got %s", cachedHost)
		assert.Equal(t, 0, retryCount,
			"expected 0 retries got %d retries", retryCount)
	})
}
func TestRESTChannels_RSN1(t *testing.T) {

	app, err := ablytest.NewSandbox(nil)
	assert.NoError(t, err)
	defer app.Close()
	client, err := ably.NewREST(app.Options()...)
	assert.NoError(t, err)
	assert.NotNil(t, client.Channels,
		"expected Channels to be initialized")
	sample := []struct {
		name string
	}{
		{name: "first_channel"},
		{name: "second_channel"},
		{name: "third_channel"},
	}

	t.Run("RSN3 RSN3a  must create new channels when they don't exist", func(t *testing.T) {
		for _, v := range sample {
			client.Channels.Get(v.name)
		}
		size := len(client.Channels.Iterate())
		assert.Equal(t, 3, size,
			"expected 3 got %d", size)
	})
	t.Run("RSN4 RSN4a must release channels", func(t *testing.T) {
		for _, v := range sample {
			ch := client.Channels.Get(v.name)
			client.Channels.Release(ch.Name)
		}
		size := len(client.Channels.Iterate())
		assert.Equal(t, 0, size,
			"expected 0 channels got %d", size)
	})
}

func TestFixConnLeak_ISSUE89(t *testing.T) {

	app, err := ablytest.NewSandbox(nil)
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	channel := client.Channels.Get("issue89")
	for i := 0; i < 10; i++ {
		err := channel.Publish(context.Background(), fmt.Sprintf("msg_%d", i), fmt.Sprint(i))
		assert.NoError(t, err)
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

func TestStatsPagination_RSC6a_RSCb3(t *testing.T) {

	for _, limit := range []int{2, 3, 20} {
		t.Run(fmt.Sprintf("limit=%d", limit), func(t *testing.T) {
			app, rest := ablytest.NewREST()
			defer app.Close()

			fixtures := statsFixtures()
			postStats(app, fixtures)

			err := ablytest.TestPagination(
				reverseStats(fixtures),
				rest.Stats(
					ably.StatsWithLimit(limit),

					// We must set an end parameter. Otherwise, we may get the
					// *current* minute's stats alongside the fixtures.
					ably.StatsWithEnd(time.Date(2020, time.January, 29, 0, 0, 0, 0, time.UTC)),
				),
				limit,
			)
			assert.NoError(t, err)
		})
	}
}

func TestStats_StartEnd_RSC6b1(t *testing.T) {

	ctx := context.Background()

	app, rest := ablytest.NewREST()
	defer app.Close()

	fixtures := statsFixtures()
	postStats(app, fixtures)

	expected := reverseStats(fixtures[1:3])

	pages, err := rest.Stats(
		ably.StatsWithStart(time.Date(2020, time.January, 28, 14, 1, 0, 0, time.UTC)),
		ably.StatsWithEnd(time.Date(2020, time.January, 28, 14, 2, 30, 0, time.UTC)),
	).Pages(ctx)
	assert.NoError(t, err)
	var got []*ably.Stats
	for pages.Next(ctx) {
		got = append(got, pages.Items()...)
	}
	assert.NoError(t, pages.Err())
	assert.Equal(t, expected, got,
		"expected: %+v; got: %+v", expected, got)
}

func TestStats_Direction_RSC6b2(t *testing.T) {

	ctx := context.Background()
	app, rest := ablytest.NewREST()
	defer app.Close()

	for _, c := range []struct {
		direction ably.Direction
		expected  []*ably.Stats
	}{
		{
			direction: ably.Backwards,
			expected:  reverseStats(statsFixtures()),
		},
		{
			direction: ably.Forwards,
			expected:  statsFixtures(),
		},
	} {
		c := c
		t.Run(fmt.Sprintf("direction=%v", c.direction), func(t *testing.T) {
			fixtures := statsFixtures()
			postStats(app, fixtures)

			expected := c.expected

			pages, err := rest.Stats(
				ably.StatsWithLimit(len(expected)),
				ably.StatsWithDirection(c.direction),

				// We must set an end parameter. Otherwise, we may get the
				// *current* minute's stats alongside the fixtures.
				ably.StatsWithEnd(time.Date(2020, time.January, 29, 0, 0, 0, 0, time.UTC)),
			).Pages(ctx)
			assert.NoError(t, err)
			var got []*ably.Stats
			for pages.Next(ctx) {
				got = append(got, pages.Items()...)
			}
			assert.NoError(t, pages.Err())
			assert.Equal(t, expected, got,
				"expected: %+v; got: %+v", expected, got)
		})
	}
}

func TestStats_Unit_RSC6b4(t *testing.T) {

	ctx := context.Background()

	app, rest := ablytest.NewREST()
	defer app.Close()

	fixtures := statsFixtures()
	postStats(app, fixtures)

	pages, err := rest.Stats(
		ably.StatsWithUnit(ably.PeriodMonth),

		// We must set an end parameter. Otherwise, we may get the
		// *current* minute's stats alongside the fixtures.
		ably.StatsWithEnd(time.Date(2020, time.January, 29, 0, 0, 0, 0, time.UTC)),
	).Pages(ctx)
	assert.NoError(t, err)
	var got []*ably.Stats
	for pages.Next(ctx) {
		got = append(got, pages.Items()...)
	}
	assert.NoError(t, pages.Err())
	assert.Equal(t, 1, len(got),
		"expected: 1; got: %v", got)

	stats := got[0]
	assert.Equal(t, "month", stats.Unit,
		"expected: \"month\"; got: %v", got)
}

func statsFixtures() []*ably.Stats {
	var fixtures []*ably.Stats
	baseDate := time.Date(2020, time.January, 28, 14, 0, 0, 0, time.UTC)
	msgCounts := ably.StatsMessageCount{
		Count: 50,
		Data:  5000,
	}
	msgTypes := ably.StatsMessageTypes{
		All:      msgCounts,
		Messages: msgCounts,
	}
	for i := time.Duration(0); i < 10; i++ {
		fixtures = append(fixtures, &ably.Stats{
			IntervalID: baseDate.Add(i * time.Minute).Format("2006-01-02:15:04"),
			Unit:       "minute",
			All:        msgTypes,
			Inbound: ably.StatsMessageTraffic{
				All:      msgTypes,
				RealTime: msgTypes,
			},
		})
	}
	return fixtures
}

func postStats(app *ablytest.Sandbox, stats []*ably.Stats) error {
	ctx, cancel := context.WithTimeout(context.Background(), ablytest.Timeout)
	defer cancel()

	statsJSON, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("marshaling stats: %w", err)
	}

	req, err := http.NewRequest("POST", "https://sandbox-rest.ably.io/stats", bytes.NewReader(statsJSON))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req = req.WithContext(ctx)
	req.SetBasicAuth(app.KeyParts())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("performing request: %w", err)
	}
	resp.Body.Close()
	return nil
}

func reverseStats(stats []*ably.Stats) []*ably.Stats {
	var reversed []*ably.Stats
	for i := len(stats) - 1; i >= 0; i-- {
		reversed = append(reversed, stats[i])
	}
	return reversed
}

var (
	intervalFormats = map[string]string{
		ably.StatGranularityMinute: "2006-01-02:15:04",
		ably.StatGranularityHour:   "2006-01-02:15",
		ably.StatGranularityDay:    "2006-01-02",
		ably.StatGranularityMonth:  "2006-01",
	}
)

func intervalFormatFor(t time.Time, granularity string) string {
	return t.Format(intervalFormats[granularity])
}
