package ably_test

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/proto"
	"github.com/ably/ably-go/ably/testutil"
)

func contains(members []*proto.PresenceMessage, clients ...string) error {
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
	t.Parallel()
	app, client := testutil.ProvisionRealtime(nil, nil)
	defer safeclose(t, client, app)

	members, err := client.Channels.GetAndAttach("persisted:presence_fixtures").Presence.Get(true)
	if err != nil {
		t.Fatal(err)
	}

	err = contains(members, fixtureMembers...)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRealtimePresence_Sync250(t *testing.T) {
	t.Parallel()
	app, client1 := testutil.ProvisionRealtime(nil, nil)
	defer safeclose(t, client1, app)
	client2 := ably.MustRealtimeClient(app.Options(nil))
	client3 := ably.MustRealtimeClient(app.Options(nil))
	defer safeclose(t, client2, client3)

	channel1 := client1.Channels.GetAndAttach("sync250")
	channel2 := client2.Channels.GetAndAttach("sync250")

	sub2, err := channel2.Presence.Subscribe()
	if err != nil {
		t.Fatalf("channel2.Presence.Subscribe()=%v", err)
	}
	defer safeclose(t, sub2)

	var rg ably.ResultGroup
	var clients = generateClients(250)
	for _, client := range clients {
		rg.Add(channel1.Presence.EnterClient(client, ""))
	}
	if err := rg.Wait(); err != nil {
		t.Fatalf("rg.Wait()=%v", err)
	}
	members2 := make([]*proto.PresenceMessage, 250)
	tout := time.After(250 * timeout)

	for i := range members2 {
		select {
		case msg := <-sub2.PresenceChannel():
			members2[i] = msg
		case <-tout:
			t.Fatalf("waiting for presence messages timed out after %v", 250*timeout)
		}
	}

	if err = contains(members2, clients...); err != nil {
		t.Fatalf("members2: %v", err)
	}
	members3, err := client3.Channels.GetAndAttach("sync250").Presence.Get(true)
	if err != nil {
		t.Fatal(err)
	}
	if err = contains(members3, clients...); err != nil {
		t.Fatalf("members3: %v", err)
	}
}

var stateGuard = ^ably.StateEnum(0)

var presTransitions = []ably.StateEnum{
	ably.StateConnConnecting,
	ably.StateConnConnected,
	stateGuard,
	ably.StateChanAttaching,
	ably.StateChanAttached,
}

func TestRealtimePresence_EnsureChannelIsAttached(t *testing.T) {
	rec := ably.NewStateRecorder()
	opts := rec.Options()
	opts.NoConnect = true
	app, client := testutil.ProvisionRealtime(nil, opts)
	defer safeclose(t, client, app)
	channel := client.Channels.Get("persisted:presence_fixtures")
	if err := ably.Wait(client.Connection.Connect()); err != nil {
		t.Fatal(err)
	}
	rec.Add(stateGuard)

	members, err := channel.Presence.Get(true)
	if err != nil {
		t.Fatal(err)
	}
	rec.Stop()
	if err = contains(members, fixtureMembers...); err != nil {
		t.Fatal(err)
	}
	if states := rec.States(); !reflect.DeepEqual(states, presTransitions) {
		t.Errorf("expected states=%# v; got %# v", presTransitions, states)
	}
}
