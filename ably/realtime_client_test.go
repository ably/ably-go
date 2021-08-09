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
	"github.com/ably/ably-go/ablytest"
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
