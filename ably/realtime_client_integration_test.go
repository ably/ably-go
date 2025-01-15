//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ablytest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealtime_RealtimeHost(t *testing.T) {
	hosts := []string{
		"127.0.0.1",
		"localhost",
		"::1",
	}

	t.Run("REC1b with endpoint option", func(t *testing.T) {
		for _, host := range hosts {
			dial := make(chan string, 1)
			client, err := ably.NewRealtime(
				ably.WithKey("xxx:xxx"),
				ably.WithEndpoint(host),
				ably.WithAutoConnect(false),
				ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
					dial <- u.Host
					return MessagePipe(nil, nil)(protocol, u, timeout)
				}),
			)
			assert.NoError(t, err)
			client.Connect()
			var recordedHost string
			ablytest.Instantly.Recv(t, &recordedHost, dial, t.Fatalf)
			h, _, err := net.SplitHostPort(recordedHost)
			assert.NoError(t, err)
			assert.Equal(t, host, h, "expected %q got %q", host, h)
		}
	})

	t.Run("REC1d2 with legacy realtimeHost option", func(t *testing.T) {
		for _, host := range hosts {
			dial := make(chan string, 1)
			client, err := ably.NewRealtime(
				ably.WithKey("xxx:xxx"),
				ably.WithRealtimeHost(host),
				ably.WithAutoConnect(false),
				ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
					dial <- u.Host
					return MessagePipe(nil, nil)(protocol, u, timeout)
				}),
			)
			assert.NoError(t, err)
			client.Connect()
			var recordedHost string
			ablytest.Instantly.Recv(t, &recordedHost, dial, t.Fatalf)
			h, _, err := net.SplitHostPort(recordedHost)
			assert.NoError(t, err)
			assert.Equal(t, host, h, "expected %q got %q", host, h)
		}
	})
}

func TestRealtime_RSC7_AblyAgent(t *testing.T) {
	t.Run("using endpoint option", func(t *testing.T) {
		t.Run("RSC7d3 : Should set ablyAgent header with correct identifiers", func(t *testing.T) {
			var agentHeaderValue string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				agentHeaderValue = r.Header.Get(ably.AblyAgentHeader)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			serverURL, err := url.Parse(server.URL)
			assert.NoError(t, err)

			client, err := ably.NewRealtime(
				ably.WithEndpoint(serverURL.Host),
				ably.WithTLS(false),
				ably.WithToken("fake:token"),
				ably.WithUseTokenAuth(true))
			assert.NoError(t, err)
			defer client.Close()

			expectedAgentHeaderValue := ably.AblySDKIdentifier + " " + ably.GoRuntimeIdentifier + " " + ably.GoOSIdentifier()
			ablytest.Wait(ablytest.ConnWaiter(client, nil, ably.ConnectionEventDisconnected), nil)

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

			client, err := ably.NewRealtime(
				ably.WithEndpoint(serverURL.Host),
				ably.WithTLS(false),
				ably.WithToken("fake:token"),
				ably.WithUseTokenAuth(true),
				ably.WithAgents(map[string]string{
					"foo": "1.2.3",
				}),
			)
			assert.NoError(t, err)
			defer client.Close()

			expectedAgentHeaderValue := ably.AblySDKIdentifier + " " + ably.GoRuntimeIdentifier + " " + ably.GoOSIdentifier() + " foo/1.2.3"
			ablytest.Wait(ablytest.ConnWaiter(client, nil, ably.ConnectionEventDisconnected), nil)

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

			client, err := ably.NewRealtime(
				ably.WithEndpoint(serverURL.Host),
				ably.WithTLS(false),
				ably.WithToken("fake:token"),
				ably.WithUseTokenAuth(true),
				ably.WithAgents(map[string]string{
					"bar": "",
				}),
			)
			assert.NoError(t, err)
			defer client.Close()

			expectedAgentHeaderValue := ably.AblySDKIdentifier + " " + ably.GoRuntimeIdentifier + " " + ably.GoOSIdentifier() + " bar"
			ablytest.Wait(ablytest.ConnWaiter(client, nil, ably.ConnectionEventDisconnected), nil)

			assert.Equal(t, expectedAgentHeaderValue, agentHeaderValue)
		})
	})

	t.Run("using legacy options", func(t *testing.T) {
		t.Run("RSC7d3 : Should set ablyAgent header with correct identifiers", func(t *testing.T) {
			var agentHeaderValue string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				agentHeaderValue = r.Header.Get(ably.AblyAgentHeader)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			serverURL, err := url.Parse(server.URL)
			assert.NoError(t, err)

			client, err := ably.NewRealtime(
				ably.WithEnvironment(ablytest.Environment),
				ably.WithTLS(false),
				ably.WithToken("fake:token"),
				ably.WithUseTokenAuth(true),
				ably.WithRealtimeHost(serverURL.Host))
			assert.NoError(t, err)
			defer client.Close()

			expectedAgentHeaderValue := ably.AblySDKIdentifier + " " + ably.GoRuntimeIdentifier + " " + ably.GoOSIdentifier()
			ablytest.Wait(ablytest.ConnWaiter(client, nil, ably.ConnectionEventDisconnected), nil)

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

			client, err := ably.NewRealtime(
				ably.WithEnvironment(ablytest.Environment),
				ably.WithTLS(false),
				ably.WithToken("fake:token"),
				ably.WithUseTokenAuth(true),
				ably.WithRealtimeHost(serverURL.Host),
				ably.WithAgents(map[string]string{
					"foo": "1.2.3",
				}),
			)
			assert.NoError(t, err)
			defer client.Close()

			expectedAgentHeaderValue := ably.AblySDKIdentifier + " " + ably.GoRuntimeIdentifier + " " + ably.GoOSIdentifier() + " foo/1.2.3"
			ablytest.Wait(ablytest.ConnWaiter(client, nil, ably.ConnectionEventDisconnected), nil)

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

			client, err := ably.NewRealtime(
				ably.WithEnvironment(ablytest.Environment),
				ably.WithTLS(false),
				ably.WithToken("fake:token"),
				ably.WithUseTokenAuth(true),
				ably.WithRealtimeHost(serverURL.Host),
				ably.WithAgents(map[string]string{
					"bar": "",
				}),
			)
			assert.NoError(t, err)
			defer client.Close()

			expectedAgentHeaderValue := ably.AblySDKIdentifier + " " + ably.GoRuntimeIdentifier + " " + ably.GoOSIdentifier() + " bar"
			ablytest.Wait(ablytest.ConnWaiter(client, nil, ably.ConnectionEventDisconnected), nil)

			assert.Equal(t, expectedAgentHeaderValue, agentHeaderValue)
		})
	})
}

func TestRealtime_RTN17_HostFallback(t *testing.T) {
	t.Parallel()

	getDNSErr := func() *net.DNSError {
		return &net.DNSError{
			IsTimeout: false,
		}
	}

	getTimeoutErr := func() error {
		return &errTimeout{}
	}

	initClientWithConnError := func(customErr error, opts ...ably.ClientOption) (visitedHosts []string) {
		client, err := ably.NewRealtime(append(opts, ably.WithAutoConnect(false), ably.WithKey("fake:key"),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				visitedHosts = append(visitedHosts, u.Hostname())
				return nil, customErr
			}))...)
		require.NoError(t, err)
		ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventDisconnected), nil)
		return
	}

	t.Run("RTN17a: First attempt should be made on default primary host", func(t *testing.T) {
		visitedHosts := initClientWithConnError(errors.New("host url is wrong"))
		assert.Equal(t, "main.realtime.ably.net", visitedHosts[0])
	})

	t.Run("RTN17b: Fallback behaviour", func(t *testing.T) {
		t.Parallel()

		t.Run("apply when default realtime endpoint is not overridden, port/tlsport not set", func(t *testing.T) {
			visitedHosts := initClientWithConnError(getTimeoutErr())
			expectedPrimaryHost := "main.realtime.ably.net"
			expectedFallbackHosts := ably.DefaultFallbackHosts()

			assert.Equal(t, 6, len(visitedHosts))
			assert.Equal(t, expectedPrimaryHost, visitedHosts[0])
			assert.ElementsMatch(t, expectedFallbackHosts, visitedHosts[1:])
		})

		t.Run("does not apply when endpoint with fqdn is used", func(t *testing.T) {
			visitedHosts := initClientWithConnError(getTimeoutErr(), ably.WithEndpoint("custom-realtime.ably.io"))
			expectedHost := "custom-realtime.ably.io"

			require.Equal(t, 1, len(visitedHosts))
			assert.Equal(t, expectedHost, visitedHosts[0])
		})

		t.Run("does not apply when legacy custom realtimeHost is used", func(t *testing.T) {
			visitedHosts := initClientWithConnError(getTimeoutErr(), ably.WithRealtimeHost("custom-realtime.ably.io"))
			expectedHost := "custom-realtime.ably.io"

			require.Equal(t, 1, len(visitedHosts))
			assert.Equal(t, expectedHost, visitedHosts[0])
		})

		t.Run("apply when fallbacks are provided", func(t *testing.T) {
			fallbacks := []string{"fallback0", "fallback1", "fallback2"}
			visitedHosts := initClientWithConnError(getTimeoutErr(), ably.WithFallbackHosts(fallbacks))
			expectedPrimaryHost := "main.realtime.ably.net"

			assert.Equal(t, 4, len(visitedHosts))
			assert.Equal(t, expectedPrimaryHost, visitedHosts[0])
			assert.ElementsMatch(t, fallbacks, visitedHosts[1:])
		})

		t.Run("apply when fallbackHostUseDefault is true, even if endpoint option is used", func(t *testing.T) {
			visitedHosts := initClientWithConnError(
				getTimeoutErr(),
				ably.WithFallbackHostsUseDefault(true),
				ably.WithEndpoint("custom"))

			expectedPrimaryHost := "custom.realtime.ably.net"
			expectedFallbackHosts := ably.DefaultFallbackHosts()

			assert.Equal(t, 6, len(visitedHosts))
			assert.Equal(t, expectedPrimaryHost, visitedHosts[0])
			assert.ElementsMatch(t, expectedFallbackHosts, visitedHosts[1:])
		})

		t.Run("apply when fallbackHostUseDefault is true, even if legacy realtimeHost is set", func(t *testing.T) {
			visitedHosts := initClientWithConnError(
				getTimeoutErr(),
				ably.WithFallbackHostsUseDefault(true),
				ably.WithRealtimeHost("custom-ably.realtime.com"))

			expectedPrimaryHost := "custom-ably.realtime.com"
			expectedFallbackHosts := ably.DefaultFallbackHosts()

			assert.Equal(t, 6, len(visitedHosts))
			assert.Equal(t, expectedPrimaryHost, visitedHosts[0])
			assert.ElementsMatch(t, expectedFallbackHosts, visitedHosts[1:])
		})
	})

	t.Run("RTN17c: Verifies internet connection is active in case of error necessitating use of an alternative host", func(t *testing.T) {
		t.Parallel()
		const internetCheckUrl = "https://internet-up.ably-realtime.com/is-the-internet-up.txt"
		rec, optn := ablytest.NewHttpRecorder()
		visitedHosts := initClientWithConnError(getDNSErr(), optn...)
		assert.Equal(t, 6, len(visitedHosts)) // including primary host
		assert.Equal(t, 5, len(rec.Requests()))
		for _, request := range rec.Requests() {
			assert.Equal(t, request.URL.String(), internetCheckUrl)
		}
	})

	t.Run("RTN17d: Check for compatible errors before attempting to reconnect to a fallback host", func(t *testing.T) {
		visitedHosts := initClientWithConnError(fmt.Errorf("host url is wrong")) // non-dns or non-timeout error
		assert.Equal(t, 1, len(visitedHosts))
		visitedHosts = initClientWithConnError(getDNSErr())
		assert.Equal(t, 6, len(visitedHosts))
		visitedHosts = initClientWithConnError(getTimeoutErr())
		assert.Equal(t, 6, len(visitedHosts))
	})

	t.Run("RTN17e: Same fallback host should be used for REST as Realtime Fallback Host for a given active connection", func(t *testing.T) {
		errCh := make(chan error, 1)
		errCh <- getTimeoutErr()
		realtimeMsgRecorder := NewMessageRecorder()         // websocket recorder
		restMsgRecorder, optn := ablytest.NewHttpRecorder() // http recorder
		_, client := ablytest.NewRealtime(ably.WithAutoConnect(false),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				err, ok := <-errCh
				if ok {
					close(errCh)
					return nil, err // return timeout error for primary host
				}
				return realtimeMsgRecorder.Dial(protocol, u, timeout) // return dial for subsequent dials
			}), optn[0])
		defer client.Close()

		err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
		if err != nil {
			t.Fatalf("Error connecting host with error %v", err)
		}
		realtimeSuccessHost := realtimeMsgRecorder.URLs()[0].Hostname()
		fallbackHosts := ably.GetEndpointFallbackHosts("sandbox")
		if !ablyutil.SliceContains(fallbackHosts, realtimeSuccessHost) {
			t.Fatalf("realtime host must be one of fallback hosts, received %v", realtimeSuccessHost)
		}

		client.Time(context.Background())                            // make a rest request
		restSuccessHost := restMsgRecorder.Request(1).URL.Hostname() // second request is to get the time, first for internet connection
		assert.Equal(t, realtimeSuccessHost, restSuccessHost)
	})
}

func TestRealtime_RTN17_Integration_HostFallback_Internal_Server_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	assert.NoError(t, err)

	fallbackHost := "sandbox-a-fallback.ably-realtime.com"
	connAttempts := 0

	app, realtime := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithTLS(false),
		ably.WithUseTokenAuth(true),
		ably.WithFallbackHosts([]string{fallbackHost}),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			connAttempts += 1
			conn, err := ably.DialWebsocket(protocol, u, timeout)
			if connAttempts == 1 {
				assert.Equal(t, serverURL.Host, u.Host)
				var websocketErr *ably.WebsocketErr
				assert.ErrorAs(t, err, &websocketErr)
				assert.Equal(t, http.StatusInternalServerError, websocketErr.HttpResp().StatusCode)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, fallbackHost, u.Hostname())
			}
			return conn, err
		}),
		ably.WithEndpoint(serverURL.Host))

	defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

	err = ablytest.Wait(ablytest.ConnWaiter(realtime, realtime.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	assert.Equal(t, 2, connAttempts)
	assert.Equal(t, fallbackHost, realtime.Rest().ActiveRealtimeHost())
}

func TestRealtime_RTN17_Integration_HostFallback_Timeout(t *testing.T) {
	timedOut := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-timedOut
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	assert.NoError(t, err)

	fallbackHost := "sandbox-a-fallback.ably-realtime.com"
	requestTimeout := 2 * time.Second
	connAttempts := 0

	app, realtime := ablytest.NewRealtime(
		ably.WithAutoConnect(false),
		ably.WithTLS(false),
		ably.WithUseTokenAuth(true),
		ably.WithFallbackHosts([]string{fallbackHost}),
		ably.WithRealtimeRequestTimeout(requestTimeout),
		ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
			connAttempts += 1
			assert.Equal(t, requestTimeout, timeout)
			conn, err := ably.DialWebsocket(protocol, u, timeout)
			if connAttempts == 1 {
				assert.Equal(t, serverURL.Host, u.Host)
				var timeoutError net.Error
				assert.ErrorAs(t, err, &timeoutError)
				assert.True(t, timeoutError.Timeout())
				timedOut <- true
			} else {
				assert.NoError(t, err)
				assert.Equal(t, fallbackHost, u.Hostname())
			}
			return conn, err
		}),
		ably.WithEndpoint(serverURL.Host))

	defer safeclose(t, ablytest.FullRealtimeCloser(realtime), app)

	err = ablytest.Wait(ablytest.ConnWaiter(realtime, realtime.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	assert.Equal(t, 2, connAttempts)
	assert.Equal(t, fallbackHost, realtime.Rest().ActiveRealtimeHost())
}

func checkUnique(ch chan string, typ string, n int) error {
	close(ch)
	uniq := make(map[string]struct{}, n)
	for s := range ch {
		if _, ok := uniq[s]; ok {
			return fmt.Errorf("duplicate connection %s: %s", typ, s)
		}
		uniq[s] = struct{}{}
	}
	if len(uniq) != n {
		return fmt.Errorf("want len(%s)=%d; got %d", typ, n, len(uniq))
	}
	return nil
}

func TestRealtime_multiple(t *testing.T) {
	const N = 3
	var all ablytest.ResultGroup
	var wg sync.WaitGroup
	app, err := ablytest.NewSandbox(nil)
	assert.NoError(t, err,
		"NewSandbox()=%v", err)
	defer safeclose(t, app)
	wg.Add(N)
	idch := make(chan string, N)
	keych := make(chan string, N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			opts := app.Options()
			opts = append(opts, ably.WithClientID(fmt.Sprintf("client/%d", i)))
			opts = append(opts, ably.WithAutoConnect(false))
			c, err := ably.NewRealtime(opts...)
			if err != nil {
				all.Add(nil, err)
				return
			}
			defer safeclose(t, ablytest.FullRealtimeCloser(c))
			err = ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
			assert.NoError(t, err)
			var rg ablytest.ResultGroup
			rg.Add(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil)
			for j := 0; j < 10; j++ {
				channel := c.Channels.Get(fmt.Sprintf("client/%d/channel/%d", i, j))
				rg.GoAdd(func(ctx context.Context) error { return channel.Attach(ctx) })
				rg.GoAdd(func(ctx context.Context) error { return channel.Attach(ctx) })
				rg.GoAdd(func(ctx context.Context) error { return channel.Presence.Enter(ctx, "") })
			}
			if err := rg.Wait(); err != nil {
				all.Add(nil, err)
				return
			}
			for j := 0; j < 25; j++ {
				channel := c.Channels.Get(fmt.Sprintf("client/%d/channel/%d", i, j))
				event, data := fmt.Sprintf("event/%d/%d", i, j), fmt.Sprintf("data/%d/%d", i, j)
				rg.GoAdd(func(ctx context.Context) error {
					return channel.Publish(ctx, event, data)
				})
			}
			if err := rg.Wait(); err != nil {
				all.Add(nil, err)
				return
			}
			for j := 0; j < 10; j++ {
				channel := c.Channels.Get(fmt.Sprintf("client/%d/channel/%d", i, j))
				rg.GoAdd(func(ctx context.Context) error { return channel.Presence.Leave(ctx, "") })
			}
			if err := rg.Wait(); err != nil {
				all.Add(nil, err)
			}
			for j := 0; j < 10; j++ {
				channel := c.Channels.Get(fmt.Sprintf("client/%d/channel/%d", i, j))
				rg.GoAdd(func(ctx context.Context) error { return channel.Detach(ctx) })
				rg.GoAdd(func(ctx context.Context) error { return channel.Detach(ctx) })
			}
			idch <- c.Connection.ID()
			keych <- c.Connection.Key()
			if err := ablytest.FullRealtimeCloser(c).Close(); err != nil {
				all.Add(nil, err)
			}
		}(i)
	}
	wg.Wait()
	if err := all.Wait(); err != nil {
		t.Fatalf("want err=nil; got %s", err)
	}
	if err := checkUnique(idch, "ID", N); err != nil {
		t.Fatal(err)
	}
	if err := checkUnique(keych, "key", N); err != nil {
		t.Fatal(err)
	}
}

func TestRealtime_DontCrashOnCloseWhenEchoOff(t *testing.T) {

	app, client := ablytest.NewRealtime(ably.WithAutoConnect(false))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
}
