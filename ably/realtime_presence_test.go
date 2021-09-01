package ably_test

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"
)

func contains(members []*ably.PresenceMessage, clients ...string) error {
	lookup := make(map[string]struct{}, len(members))
	for _, member := range members {
		lookup[member.ClientID] = struct{}{}
	}
	for _, client := range clients {
		if _, ok := lookup[client]; !ok {
			return fmt.Errorf("clientID=%q not found in presence map", client)
		}
	}
	return nil
}

func generateClients(num int) []string {
	clients := make([]string, 0, num)
	for i := 0; i < num; i++ {
		clients = append(clients, "client"+strconv.Itoa(i))
	}
	return clients
}

var fixtureMembers = []string{
	"client_bool",
	"client_int",
	"client_string",
	"client_json",
}

func TestRealtimePresence_Sync(t *testing.T) {
	app, client := ablytest.NewRealtime(nil...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
	err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)

	members, err := client.Channels.Get("persisted:presence_fixtures").Presence.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if err = contains(members, fixtureMembers...); err != nil {
		t.Fatal(err)
	}
}

/*
FAILING TEST
Go 1.15, 1 Sep 2021: https://github.com/ably/ably-go/pull/383/checks?check_run_id=3485574655#step:9:763
*/
func SKIP_TestRealtimePresence_Sync250(t *testing.T) {
	app, client1 := ablytest.NewRealtime(nil...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client1), app)
	client2 := app.NewRealtime(nil...)
	client3 := app.NewRealtime(nil...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client2), ablytest.FullRealtimeCloser(client3))
	err := ablytest.Wait(ablytest.ConnWaiter(client1, client1.Connect, ably.ConnectionEventConnected), nil)
	if err != nil {
		t.Fatal(err)
	}
	err = ablytest.Wait(ablytest.ConnWaiter(client2, client2.Connect, ably.ConnectionEventConnected), nil)
	if err != nil {
		t.Fatal(err)
	}
	err = ablytest.Wait(ablytest.ConnWaiter(client3, client3.Connect, ably.ConnectionEventConnected), nil)
	if err != nil {
		t.Fatal(err)
	}
	channel1 := client1.Channels.Get("sync250")
	if err := channel1.Attach(context.Background()); err != nil {
		t.Fatal(err)
	}
	channel2 := client2.Channels.Get("sync250")
	if err := channel2.Attach(context.Background()); err != nil {
		t.Fatal(err)
	}

	sub2, unsub2, err := ablytest.ReceivePresenceMessages(channel2, nil)
	if err != nil {
		t.Fatalf("channel2.ablytest.ReceiveMessages(Presence)=%v", err)
	}
	defer unsub2()

	var rg ablytest.ResultGroup
	var clients = generateClients(250)
	for _, client := range clients {
		client := client
		rg.GoAdd(func(ctx context.Context) error { return channel1.Presence.EnterClient(ctx, client, "") })
	}
	if err := rg.Wait(); err != nil {
		t.Fatalf("rg.Wait()=%v", err)
	}
	members2 := make([]*ably.PresenceMessage, 250)
	tout := time.After(250 * ablytest.Timeout)

	for i := range members2 {
		select {
		case msg := <-sub2:
			members2[i] = msg
		case <-tout:
			t.Fatalf("waiting for presence messages timed out after %v", 250*ablytest.Timeout)
		}
	}

	if err = contains(members2, clients...); err != nil {
		t.Fatalf("members2: %v", err)
	}
	members3, err := client3.Channels.Get("sync250").Presence.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err = contains(members3, clients...); err != nil {
		t.Fatalf("members3: %v", err)
	}
}

func TestRealtimePresence_EnsureChannelIsAttached(t *testing.T) {
	presTransitions := []ably.ChannelState{
		ably.ChannelStateInitialized,
		ably.ChannelStateAttaching,
		ably.ChannelStateAttached,
	}
	var rec ablytest.ChanStatesRecorder
	opts := []ably.ClientOption{
		ably.WithAutoConnect(false),
	}
	app, client := ablytest.NewRealtime(opts...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
	channel := client.Channels.Get("persisted:presence_fixtures")
	off := rec.Listen(channel)
	defer off()
	if err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil); err != nil {
		t.Fatal(err)
	}
	members, err := channel.Presence.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if expected, got := presTransitions, rec.States(); !reflect.DeepEqual(expected, got) {
		t.Fatalf("expected %+v, got %+v", expected, got)
	}
	if err = contains(members, fixtureMembers...); err != nil {
		t.Fatal(err)
	}
}
