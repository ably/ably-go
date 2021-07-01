package ably_test

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

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

	setUpWithEOF := func() (app *ablytest.Sandbox, client *ably.Realtime, doEOF chan struct{}, rec *MessageRecorder) {
		doEOF = make(chan struct{}, 1)
		rec = NewMessageRecorder()
		app, client = ablytest.NewRealtime(
			ably.WithAutoConnect(false),
			ably.WithDial(func(protocol string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
				c, err := rec.Dial(protocol, u, timeout)
				return protoConnWithFakeEOF{Conn: c, doEOF: doEOF}, err
			}))
		return
	}

	// todo - we can add check after getting disconnected and connected again
	t.Run("RTN17a: First attempt should be on default host first", func(t *testing.T) {
		t.Parallel()
		app, client, _, recorder := setUpWithEOF()
		defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
		err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
		if err != nil {
			t.Fatal(err)
		}
		connectionState := client.Connection.State()
		if connectionState != ably.ConnectionStateConnected {
			t.Fatalf("expected %v; got %v", ably.ConnectionStateConnected, connectionState)
		}
		retriedHost := recorder.URLs()[0].Hostname()
		expectedHost := "sandbox-realtime.ably.io"
		if retriedHost != expectedHost {
			t.Fatalf("expected %v; got %v", expectedHost, retriedHost)
		}
	})

	t.Run("RTN17b: Fallback behaviour", func(t *testing.T) {
		t.Parallel()
		t.Run("does not apply when the default custom endpoint is used", func(t *testing.T) {
			t.Parallel()
			rec := NewMessageRecorder()
			app, client := ablytest.NewRealtime(
				ably.WithAutoConnect(false),
				ably.WithRealtimeHost("un.reachable.host.example.com"),
				ably.WithDial(rec.Dial))
			defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
			err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventDisconnected), nil)
			if err == nil {
				t.Fatal("Should return error for unreachable host")
			}
			// Explicit check to not retry on fallbacks
		})

		t.Run("apply when HTTP client is using same fallback endpoint and default realtime endpoint not overriden", func(t *testing.T) {

		})

		t.Run("does not apply when environment is overriden and fallback not specified", func(t *testing.T) {

		})

		t.Run("apply when environment is overriden and fallback specified, the fallback is used", func(t *testing.T) {

		})
	})

	t.Run("RTN17c: Verifies internet connection is active in case of error necessitating use of an alternative host", func(t *testing.T) {

	})

	t.Run("RTN17d: Check for compatible errors before attempting to reconnect to a fallback host", func(t *testing.T) {

	})

	t.Run("RTN17e: Same fallback host should be used for REST as Realtime Fallback Host for a given active connection", func(t *testing.T) {

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
