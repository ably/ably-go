package ably_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/testutil"
)

func TestRealtimeClient_RealtimeHost(t *testing.T) {
	t.Parallel()
	httpClient := testutil.NewHTTPClient()
	app, err := testutil.NewSandbox(nil)
	if err != nil {
		t.Fatalf("NewSandbox=%s", err)
	}
	defer safeclose(t, app)

	rec := ably.NewRecorder(httpClient)
	hosts := []string{
		"127.0.0.1",
		"localhost",
		"::1",
	}
	stateRec := ably.NewStateRecorder(len(hosts))
	for _, host := range hosts {
		opts := rec.Options(host)
		opts.Listener = stateRec.Channel()
		client, err := ably.NewRealtimeClient(app.Options(opts))
		if err != nil {
			t.Errorf("NewRealtimeClient=%s (host=%s)", err, host)
			continue
		}
		if state := client.Connection.State(); state != ably.StateConnInitialized {
			t.Errorf("want state=%v; got %s", ably.StateConnInitialized, state)
			continue
		}
		if err := checkError(80000, wait(client.Connection.Connect())); err != nil {
			t.Errorf("%s (host=%s)", err, host)
			continue
		}
		if state := client.Connection.State(); state != ably.StateConnFailed {
			t.Errorf("want state=%v; got %s", ably.StateConnFailed, state)
			continue
		}
		if err := checkError(50002, client.Close()); err != nil {
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
	if err := stateRec.WaitFor(want, time.Second); err != nil {
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

func TestRealtimeClient_50clients(t *testing.T) {
	t.Parallel()
	var all ably.ResultGroup
	var wg sync.WaitGroup
	app, err := testutil.NewSandbox(nil)
	if err != nil {
		t.Fatalf("NewSandbox()=%v", err)
	}
	defer safeclose(t, app)
	wg.Add(50)
	for i := 0; i < 50; i++ {
		go func(i int) {
			defer wg.Done()
			opts := app.Options(nil)
			opts.ClientID = fmt.Sprintf("client-%d", i)
			opts.NoConnect = true
			c, err := ably.NewRealtimeClient(opts)
			if err != nil {
				all.Add(nil, err)
				return
			}
			var rg ably.ResultGroup
			rg.Add(c.Connection.Connect())
			for j := 0; j < 10; j++ {
				channel := c.Channels.Get(fmt.Sprintf("client-%d-channel-%d", i, j))
				rg.Add(channel.Attach())
				rg.Add(channel.Presence.Enter(""))
			}
			if err := rg.Wait(); err != nil {
				all.Add(nil, err)
			}
			if err := c.Close(); err != nil {
				all.Add(nil, err)
			}
		}(i)
	}
	wg.Wait()
	if err := all.Wait(); err != nil {
		t.Fatalf("want err=nil; got %s", err)
	}
}

func TestRealtimeClient_DontCrashOnCloseWhenEchoOff(t *testing.T) {
	t.Parallel()
	app, client := testutil.Provision(&ably.ClientOptions{NoConnect: true})
	defer safeclose(t, app)

	if err := checkError(50002, client.Close()); err != nil {
		t.Fatal(err)
	}
}
