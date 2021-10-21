package ably_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"
)

func TestRealtime_RealtimeHost(t *testing.T) {
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

func TestRealtime_RSC7_AblyAgent(t *testing.T) {
	t.Run("RSC7d3 : Should set ablyAgent header with correct identifiers", func(t *testing.T) {
		var agentHeaderValue string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			agentHeaderValue = r.Header.Get(ably.AblyAgentHeader)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		serverURL, err := url.Parse(server.URL)
		assertNil(t, err)

		client, err := ably.NewRealtime(
			ably.WithEnvironment(ablytest.Environment),
			ably.WithTLS(false),
			ably.WithToken("fake:token"),
			ably.WithUseTokenAuth(true),
			ably.WithRealtimeHost(serverURL.Host))
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()

		expectedAgentHeaderValue := ably.AblySDKIdentifier + " " + ably.GoRuntimeIdentifier + " " + ably.GoOSIdentifier()
		ablytest.Wait(ablytest.ConnWaiter(client, nil, ably.ConnectionEventDisconnected), nil)

		assertEquals(t, expectedAgentHeaderValue, agentHeaderValue)
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

/*
FAILING TEST
https://github.com/ably/ably-go/pull/383/checks?check_run_id=3489103775#step:7:618

--- FAIL: TestRealtime_DontCrashOnCloseWhenEchoOff (60.00s)
panic: Post "https://sandbox-rest.ably.io/apps": context deadline exceeded (Client.Timeout exceeded while awaiting headers) [recovered]
	panic: Post "https://sandbox-rest.ably.io/apps": context deadline exceeded (Client.Timeout exceeded while awaiting headers)

goroutine 324686 [running]:
testing.tRunner.func1.1(0xd3cca0, 0xc000a96090)
	/opt/hostedtoolcache/go/1.14.15/x64/src/testing/testing.go:999 +0x461
testing.tRunner.func1(0xc000468000)
	/opt/hostedtoolcache/go/1.14.15/x64/src/testing/testing.go:1002 +0x606
panic(0xd3cca0, 0xc000a96090)
	/opt/hostedtoolcache/go/1.14.15/x64/src/runtime/panic.go:975 +0x3e3
github.com/ably/ably-go/ablytest.MustSandbox(0x0, 0xc000d40d30)
	/home/runner/work/ably-go/ably-go/ablytest/sandbox.go:124 +0xd1
github.com/ably/ably-go/ablytest.NewRealtime(0xc0000ade98, 0x1, 0x1, 0xc000d40d20, 0x5bbe7c)
	/home/runner/work/ably-go/ably-go/ablytest/sandbox.go:104 +0x4d
github.com/ably/ably-go/ably_test.TestRealtime_DontCrashOnCloseWhenEchoOff(0xc000468000)
	/home/runner/work/ably-go/ably-go/ably/realtime_client_test.go:147 +0x85
testing.tRunner(0xc000468000, 0xdf8340)
	/opt/hostedtoolcache/go/1.14.15/x64/src/testing/testing.go:1050 +0x1ec
created by testing.(*T).Run
	/opt/hostedtoolcache/go/1.14.15/x64/src/testing/testing.go:1095 +0x538
*/
func TestRealtime_DontCrashOnCloseWhenEchoOff(t *testing.T) {
	t.Skip("FAILING TEST")

	app, client := ablytest.NewRealtime(ably.WithAutoConnect(false))
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
}
