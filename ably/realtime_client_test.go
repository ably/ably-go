package ably_test

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablytest"
)

func TestRealtime_RealtimeHost(t *testing.T) {
	t.Parallel()
	hosts := []string{
		"127.0.0.1",
		"localhost",
		"::1",
	}
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
		if err != nil {
			t.Fatal(err)
		}
		client.Connect()
		var recordedHost string
		ablytest.Instantly.Recv(t, &recordedHost, dial, t.Fatalf)
		h, _, err := net.SplitHostPort(recordedHost)
		if err != nil {
			t.Fatal(err)
		}
		if h != host {
			t.Errorf(" expected %q got %q", host, h)
		}
	}
}

func TestRealtime_RTN17_HostFallback(t *testing.T) {

	getDNSErr := func() *net.DNSError {
		return &net.DNSError{
			Err:         "Can't resolve host",
			Name:        "Host unresolvable",
			Server:      "rest.ably.com",
			IsTimeout:   false,
			IsTemporary: false,
			IsNotFound:  false,
		}
	}

	getTimeoutErr := func() error {
		dnsErr := getDNSErr()
		dnsErr.IsTimeout = true
		return dnsErr
	}

	setUpWithError := func(err error, opts ...ably.ClientOption) (visitedHosts []string) {
		hostCh := make(chan string, 1)
		errCh := make(chan error, 1)
		defaultOptions := []ably.ClientOption{
			ably.WithAutoConnect(false),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				hostCh <- u.Hostname()
				return nil, <-errCh
			}),
		}
		opts = append(opts, defaultOptions...)
		_, client := ablytest.NewRealtime(opts...)
		client.Connect()
		connDisconnectedEventChan := make(chan struct{})
		// inject error to return after receiving host and break the loop once disconnect event is received
		go func(err error) {
			for {
				select {
				case host := <-hostCh:
					visitedHosts = append(visitedHosts, host)
					errCh <- err
				case <-connDisconnectedEventChan:
					return
				}
			}
		}(err)
		ablytest.Wait(ablytest.ConnWaiter(client, nil, ably.ConnectionEventDisconnected), nil)
		connDisconnectedEventChan <- struct{}{}
		return
	}

	t.Run("RTN17a: First attempt should be on default host first", func(t *testing.T) {
		t.Parallel()
		visitedHosts := setUpWithError(fmt.Errorf("host url is wrong"))
		expectedHost := "sandbox-realtime.ably.io"
		if visitedHosts[0] != expectedHost {
			t.Fatalf("expected %v; got %v", expectedHost, visitedHosts[0])
		}
	})

	t.Run("RTN17b: Fallback behaviour", func(t *testing.T) {
		t.Parallel()

		t.Run("apply when default realtime endpoint is not overridden, port/tlsport not set", func(t *testing.T) {
			t.Parallel()
			visitedHosts := setUpWithError(getTimeoutErr())
			expectedPrimaryHost := "sandbox-realtime.ably.io"
			expectedFallbackHosts := ably.GetEnvFallbackHosts("sandbox")
			if len(visitedHosts) != 6 {
				t.Fatalf("visited hosts other than primary hosts %v", visitedHosts)
			}
			if visitedHosts[0] != expectedPrimaryHost {
				t.Fatalf("expected %v; got %v", expectedPrimaryHost, visitedHosts[0])
			}
			assertDeepEquals(t, ablyutil.Sort(expectedFallbackHosts), ablyutil.Sort(visitedHosts[1:]))
		})

		t.Run("does not apply when the custom realtime endpoint is used", func(t *testing.T) {
			t.Parallel()
			visitedHosts := setUpWithError(getTimeoutErr(), ably.WithRealtimeHost("custom-realtime.ably.io"))
			expectedHost := "custom-realtime.ably.io"
			if len(visitedHosts) > 1 {
				t.Fatalf("visited hosts other than primary hosts %v", visitedHosts)
			}
			if visitedHosts[0] != expectedHost {
				t.Fatalf("expected %v; got %v", expectedHost, visitedHosts[0])
			}
		})

		t.Run("apply when fallbacks are provided", func(t *testing.T) {
			t.Parallel()
			fallbacks := []string{"fallback0", "fallback1", "fallback2"}
			visitedHosts := setUpWithError(getTimeoutErr(), ably.WithFallbackHosts(fallbacks))
			expectedPrimaryHost := "sandbox-realtime.ably.io"
			if len(visitedHosts) != 4 {
				t.Fatalf("visited hosts other than primary hosts %v", visitedHosts)
			}
			if visitedHosts[0] != expectedPrimaryHost {
				t.Fatalf("expected %v; got %v", expectedPrimaryHost, visitedHosts[0])
			}
			assertDeepEquals(t, ablyutil.Sort(fallbacks), ablyutil.Sort(visitedHosts[1:]))
		})

		t.Run("apply when fallbackHostUseDefault is true, even if env. or host is set", func(t *testing.T) {
			t.Parallel()
			visitedHosts := setUpWithError(
				getTimeoutErr(),
				ably.WithFallbackHostsUseDefault(true),
				ably.WithEnvironment("custom"),
				ably.WithRealtimeHost("custom-ably.realtime.com"))

			expectedPrimaryHost := "custom-ably.realtime.com"
			expectedFallbackHosts := ably.DefaultFallbackHosts()
			if len(visitedHosts) != 6 {
				t.Fatalf("visited hosts other than primary hosts %v", visitedHosts)
			}
			if visitedHosts[0] != expectedPrimaryHost {
				t.Fatalf("expected %v; got %v", expectedPrimaryHost, visitedHosts[0])
			}
			assertDeepEquals(t, ablyutil.Sort(expectedFallbackHosts), ablyutil.Sort(visitedHosts[1:]))
		})
	})

	t.Run("RTN17c: Verifies internet connection is active in case of error necessitating use of an alternative host", func(t *testing.T) {
		t.Parallel()
		const internetCheckUrl = "https://internet-up.ably-realtime.com/is-the-internet-up.txt"
		rec, optn := recorder()
		visitedHosts := setUpWithError(getDNSErr(), optn...)
		assertEquals(t, 6, len(visitedHosts)) // including primary host
		assertEquals(t, 5, len(rec.Requests()))
		for _, request := range rec.Requests() {
			assertEquals(t, request.URL.String(), internetCheckUrl)
		}
	})

	t.Run("RTN17d: Check for compatible errors before attempting to reconnect to a fallback host", func(t *testing.T) {
		t.Parallel()
		visitedHosts := setUpWithError(fmt.Errorf("host url is wrong")) // non-dns or non-timeout error
		if len(visitedHosts) != 1 {
			t.Fatalf("should not try fallback hosts for non-dns or timeout error, received visited hosts %v", visitedHosts)
		}
		visitedHosts = setUpWithError(getDNSErr())
		if len(visitedHosts) != 6 {
			t.Fatalf("should try fallbacks hosts for dns error, received visited hosts %v", visitedHosts)
		}
		visitedHosts = setUpWithError(getTimeoutErr())
		if len(visitedHosts) != 6 {
			t.Fatalf("should not try fallbacks hosts for timeout error, received visited hosts %v", visitedHosts)
		}
	})

	t.Run("RTN17e: Same fallback host should be used for REST as Realtime Fallback Host for a given active connection", func(t *testing.T) {
		t.Parallel()
		errCh := make(chan error, 1)
		errCh <- getTimeoutErr()
		realtimeMsgRecorder := NewMessageRecorder() // websocket recorder
		restMsgRecorder, optn := recorder()         // http recorder
		_, client := ablytest.NewRealtime(ably.WithAutoConnect(false),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				err, ok := <-errCh
				if ok {
					close(errCh)
					return nil, err // return timeout error for primary host
				}
				return realtimeMsgRecorder.Dial(protocol, u, timeout) // return dial for subsequent dials
			}), optn[0])

		ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventDisconnected), nil)
		err := ablytest.Wait(ablytest.ConnWaiter(client, nil, ably.ConnectionEventConnected), nil)
		if err != nil {
			t.Fatalf("Error connecting host with error %v", err)
		}
		client.Time(context.Background()) // make a rest request
		fallbackHosts := ably.GetEnvFallbackHosts("sandbox")

		realtimeSuccessHost := realtimeMsgRecorder.URLs()[0].Hostname()

		if !ablyutil.Contains(fallbackHosts, realtimeSuccessHost) {
			t.Fatalf("realtime host must be one of fallback hosts, received %v", realtimeSuccessHost)
		}
		restSuccessHost := restMsgRecorder.Request(1).URL.Hostname() // second request is to get the time, first for internet connection
		assertEquals(t, realtimeSuccessHost, restSuccessHost)
	})
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
	t.Parallel()
	const N = 3
	var all ablytest.ResultGroup
	var wg sync.WaitGroup
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatalf("NewSandbox()=%v", err)
	}
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
			if err = ablytest.Wait(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnected), nil); err != nil {
				t.Error(err)
				return
			}
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
	t.Parallel()
	app, client := ablytest.NewRealtime(ably.WithAutoConnect(false))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
}
