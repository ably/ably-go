package ably_test

import (
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
