package ably_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
)

func TestRealtime_RealtimeHost(t *testing.T) {
	t.Parallel()
	httpClient := ablytest.NewHTTPClient()
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatalf("NewSandbox=%s", err)
	}
	defer safeclose(t, app)

	rec := ablytest.NewRecorder(httpClient)
	hosts := []string{
		"127.0.0.1",
		"localhost",
		"::1",
	}
	stateRec := ablytest.NewStateRecorder(len(hosts))
	for _, host := range hosts {
		opts := rec.Options(host)
		opts.Listener = stateRec.Channel()
		client, err := ably.NewRealtime(app.Options(opts))
		if err != nil {
			t.Errorf("NewRealtime=%s (host=%s)", err, host)
			continue
		}
		if state := client.Connection.State(); state != ably.StateConnInitialized {
			t.Errorf("want state=%v; got %s", ably.StateConnInitialized, state)
			continue
		}
		if err := checkError(80000, ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnectedV12).Wait()); err != nil {
			t.Errorf("%s (host=%s)", err, host)
			continue
		}
		if state := client.Connection.State(); state != ably.StateConnFailed {
			t.Errorf("want state=%v; got %s", ably.StateConnFailed, state)
			continue
		}
		if err := checkError(80000, ablytest.FullRealtimeCloser(client).Close()); err != nil {
			t.Errorf("%s (host=%s)", err, host)
			continue
		}
		if _, ok := rec.Hosts[host]; !ok {
			t.Errorf("host %s was not recorded (recorded %v)", host, rec.Hosts)
		}
	}
	want := []ably.StateEnum{
		ably.StateConnConnecting,
		ably.StateConnFailed,
		ably.StateConnConnecting,
		ably.StateConnFailed,
		ably.StateConnConnecting,
		ably.StateConnFailed,
	}
	if err := stateRec.WaitFor(want); err != nil {
		t.Fatal(err)
	}
	errors := stateRec.Errors()
	if len(errors) != len(hosts) {
		t.Fatalf("want len(errors)=%d; got %d", len(hosts), len(errors))
	}
	for i, err := range errors {
		if err := checkError(80000, err); err != nil {
			t.Errorf("%s (host=%s)", err, hosts[i])
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
			opts := app.Options(nil)
			opts.ClientID = fmt.Sprintf("client/%d", i)
			opts.NoConnect = true
			c, err := ably.NewRealtime(opts)
			if err != nil {
				all.Add(nil, err)
				return
			}
			var rg ablytest.ResultGroup
			rg.Add(ablytest.ConnWaiter(c, c.Connect, ably.ConnectionEventConnectedV12), nil)
			for j := 0; j < 10; j++ {
				channel := c.Channels.Get(fmt.Sprintf("client/%d/channel/%d", i, j))
				rg.Add(channel.Attach())
				rg.Add(channel.Attach())
				rg.Add(channel.Presence.Enter(""))
			}
			if err := rg.Wait(); err != nil {
				all.Add(nil, err)
			}
			for j := 0; j < 25; j++ {
				channel := c.Channels.Get(fmt.Sprintf("client/%d/channel/%d", i, j))
				rg.Add(channel.Publish(fmt.Sprintf("event/%d/%d", i, j), fmt.Sprintf("data/%d/%d", i, j)))
			}
			if err := rg.Wait(); err != nil {
				all.Add(nil, err)
			}
			for j := 0; j < 10; j++ {
				channel := c.Channels.Get(fmt.Sprintf("client/%d/channel/%d", i, j))
				rg.Add(channel.Presence.Leave(""))
				rg.Add(channel.Detach())
				rg.Add(channel.Detach())
			}
			if err := rg.Wait(); err != nil {
				all.Add(nil, err)
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
	app, client := ablytest.NewRealtime(&ably.ClientOptions{NoConnect: true})
	defer safeclose(t, app, ablytest.FullRealtimeCloser(client))
}
