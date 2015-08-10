package ably_test

import (
	"testing"

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
		"ablytest_testhost",
		"ablytest_somehost",
		"ablytest_someotherhost",
	}
	for _, host := range hosts {
		client, err := ably.NewRealtimeClient(app.Options(rec.Options(host)))
		if err != nil {
			t.Errorf("NewRealtimeClient=%s (host=%s)", err)
			continue
		}
		if err := checkError(80000, ably.Wait(client.Connection.Connect())); err != nil {
			t.Errorf("%s (host=%s)", err, host)
			continue
		}
		if err := checkError(50002, client.Close()); err != nil {
			t.Errorf("%s (host=%s)", err, host)
			continue
		}
		if _, ok := rec.Hosts[host]; !ok {
			t.Errorf("host %s was not recorded", host)
		}
	}
}
